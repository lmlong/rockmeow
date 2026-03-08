package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/lingguard/pkg/llm"
	"github.com/lingguard/pkg/logger"
)

// OpenAIProvider OpenAI 兼容提供商
type OpenAIProvider struct {
	name     string
	model    string
	apiKey   string
	apiBase  string
	client   *resty.Client
	supports struct {
		tools  bool
		vision bool
	}
}

// NewOpenAIProvider 创建 OpenAI 提供商
func NewOpenAIProvider(name string, cfg *ProviderConfig) *OpenAIProvider {
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = "https://api.openai.com/v1"
	}

	// 设置超时，默认 60 秒
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	// 检测是否支持工具：如果配置指定了则使用配置值，否则默认为 true
	supportsTools := true
	if cfg.SupportsTools != nil {
		supportsTools = *cfg.SupportsTools
	}

	return &OpenAIProvider{
		name:    name,
		model:   cfg.Model,
		apiKey:  cfg.APIKey,
		apiBase: apiBase,
		client:  resty.New().SetTimeout(timeout),
		supports: struct {
			tools  bool
			vision bool
		}{
			tools:  supportsTools,
			vision: true,
		},
	}
}

func (p *OpenAIProvider) Name() string  { return p.name }
func (p *OpenAIProvider) Model() string { return p.model }

func (p *OpenAIProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	start := time.Now()

	// 记录请求
	logger.LLMRequest(p.name, req.Model, req)

	// 手动序列化请求，确保使用自定义的 MarshalJSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// DEBUG: 记录请求体
	if len(reqBody) > 2000 {
		logger.Debug("Complete request body (truncated)", "body", string(reqBody[:2000]))
	} else {
		logger.Debug("Complete request body", "body", string(reqBody))
	}

	resp, err := p.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+p.apiKey).
		SetBody(reqBody).
		Post(p.apiBase + "/chat/completions")

	duration := time.Since(start)

	if err != nil {
		logger.LLMResponse(p.name, req.Model, nil, duration, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if !resp.IsSuccess() {
		err := fmt.Errorf("API error: %s", resp.String())
		logger.LLMResponse(p.name, req.Model, nil, duration, err)
		return nil, err
	}

	// 手动解析响应，确保使用自定义的 UnmarshalJSON
	var result llm.Response
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		logger.LLMResponse(p.name, req.Model, nil, duration, err)
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// 记录响应
	logger.LLMResponse(p.name, req.Model, &result, duration, nil)

	return &result, nil
}

func (p *OpenAIProvider) Stream(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error) {
	req.Stream = true
	eventChan := make(chan llm.StreamEvent, 100)

	// 记录请求开始
	logger.LLMRequest(p.name, req.Model, map[string]interface{}{
		"model":    req.Model,
		"messages": len(req.Messages),
		"stream":   true,
	})

	// 手动序列化请求，确保使用自定义的 MarshalJSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		close(eventChan)
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// DEBUG: 完整请求体日志（用于排查 content 格式问题）
	if len(reqBody) > 5000 {
		logger.Debug("Stream request body (truncated)", "body", string(reqBody[:5000]))
	} else {
		logger.Debug("Stream request body (full)", "body", string(reqBody))
	}

	start := time.Now()

	resp, err := p.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+p.apiKey).
		SetBody(reqBody).
		SetDoNotParseResponse(true).
		Post(p.apiBase + "/chat/completions")

	if err != nil {
		close(eventChan)
		duration := time.Since(start)
		logger.LLMResponse(p.name, req.Model, nil, duration, err)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// 检查 HTTP 状态码
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		bodyBytes, _ := io.ReadAll(resp.RawBody())
		resp.RawBody().Close()
		close(eventChan)
		duration := time.Since(start)
		errMsg := fmt.Sprintf("API error: status=%d body=%s", resp.StatusCode(), string(bodyBytes))
		logger.LLMResponse(p.name, req.Model, nil, duration, fmt.Errorf(errMsg))
		return nil, fmt.Errorf(errMsg)
	}

	go func() {
		defer close(eventChan)
		defer resp.RawBody().Close()

		// 使用 bufio.Reader 代替 Scanner，避免 64KB 行长度限制
		reader := bufio.NewReader(resp.RawBody())
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				logger.Warn("Stream read error", "error", err)
				return
			}

			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var event llm.StreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			select {
			case eventChan <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventChan, nil
}

func (p *OpenAIProvider) SupportsTools() bool  { return p.supports.tools }
func (p *OpenAIProvider) SupportsVision() bool { return p.supports.vision }
