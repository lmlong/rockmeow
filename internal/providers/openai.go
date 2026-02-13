package providers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
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
			tools:  true,
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

	resp, err := p.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+p.apiKey).
		SetBody(req).
		SetResult(&llm.Response{}).
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

	result := resp.Result().(*llm.Response)

	// 记录响应
	logger.LLMResponse(p.name, req.Model, result, duration, nil)

	return result, nil
}

func (p *OpenAIProvider) Stream(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error) {
	req.Stream = true
	eventChan := make(chan llm.StreamEvent, 100)

	resp, err := p.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+p.apiKey).
		SetBody(req).
		SetDoNotParseResponse(true).
		Post(p.apiBase + "/chat/completions")

	if err != nil {
		close(eventChan)
		return nil, fmt.Errorf("request failed: %w", err)
	}

	go func() {
		defer close(eventChan)
		defer resp.RawBody().Close()

		scanner := bufio.NewScanner(resp.RawBody())
		for scanner.Scan() {
			line := scanner.Text()
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
