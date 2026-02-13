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

// AnthropicProvider Anthropic 兼容提供商
type AnthropicProvider struct {
	name     string
	apiKey   string
	apiBase  string
	model    string
	client   *resty.Client
	supports struct {
		tools  bool
		vision bool
	}
}

// NewAnthropicProvider 创建 Anthropic 提供商
func NewAnthropicProvider(name string, cfg *ProviderConfig) *AnthropicProvider {
	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = "https://api.anthropic.com"
	}

	model := cfg.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}

	// 设置超时，默认 60 秒
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	return &AnthropicProvider{
		name:    name,
		apiKey:  cfg.APIKey,
		apiBase: apiBase,
		model:   model,
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

func (p *AnthropicProvider) Name() string  { return p.name }
func (p *AnthropicProvider) Model() string { return p.model }

// anthropicRequest Anthropic API 请求格式
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

// anthropicMessage Anthropic 消息格式
type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// anthropicContent Anthropic 内容格式
type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// anthropicTool Anthropic 工具格式
type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// anthropicResponse Anthropic 响应格式
type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// anthropicStreamEvent Anthropic 流式事件
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"delta,omitempty"`
	ContentBlock *struct {
		Type  string          `json:"type"`
		Text  string          `json:"text,omitempty"`
		ID    string          `json:"id,omitempty"`
		Name  string          `json:"name,omitempty"`
		Input json.RawMessage `json:"input,omitempty"`
	} `json:"content_block,omitempty"`
	Message *anthropicResponse `json:"message,omitempty"`
}

func (p *AnthropicProvider) Complete(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	start := time.Now()

	// 转换请求格式
	anthropicReq := p.convertRequest(req)

	// 记录请求
	logger.LLMRequest(p.name, req.Model, anthropicReq)

	resp, err := p.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("x-api-key", p.apiKey).
		SetHeader("anthropic-version", "2023-06-01").
		SetBody(anthropicReq).
		SetResult(&anthropicResponse{}).
		Post(p.apiBase + "/v1/messages")

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

	// 转换响应格式
	result := p.convertResponse(resp.Result().(*anthropicResponse), req.Model)

	// 记录响应
	logger.LLMResponse(p.name, req.Model, result, duration, nil)

	return result, nil
}

func (p *AnthropicProvider) Stream(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error) {
	anthropicReq := p.convertRequest(req)
	anthropicReq.Stream = true

	eventChan := make(chan llm.StreamEvent, 100)

	resp, err := p.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("x-api-key", p.apiKey).
		SetHeader("anthropic-version", "2023-06-01").
		SetBody(anthropicReq).
		SetDoNotParseResponse(true).
		Post(p.apiBase + "/v1/messages")

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
			if data == "" {
				continue
			}

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			// 转换为通用流式事件
			streamEvent := p.convertStreamEvent(&event)
			if streamEvent != nil {
				select {
				case eventChan <- *streamEvent:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return eventChan, nil
}

func (p *AnthropicProvider) SupportsTools() bool  { return p.supports.tools }
func (p *AnthropicProvider) SupportsVision() bool { return p.supports.vision }

// convertRequest 转换 OpenAI 格式请求为 Anthropic 格式
func (p *AnthropicProvider) convertRequest(req *llm.Request) *anthropicRequest {
	anthropicReq := &anthropicRequest{
		Model:     p.model,
		MaxTokens: 4096,
		Messages:  make([]anthropicMessage, 0),
	}

	if req.MaxTokens > 0 {
		anthropicReq.MaxTokens = req.MaxTokens
	}

	// 提取系统消息并转换其他消息
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			anthropicReq.System = msg.Content
			continue
		}

		anthropicMsg := anthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// 处理工具调用消息
		if len(msg.ToolCalls) > 0 {
			// Assistant 消息包含工具调用
			content := make([]interface{}, 0)
			if msg.Content != "" {
				content = append(content, anthropicContent{
					Type: "text",
					Text: msg.Content,
				})
			}
			for _, tc := range msg.ToolCalls {
				content = append(content, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": json.RawMessage(tc.Function.Arguments),
				})
			}
			anthropicMsg.Content = content
		} else if msg.Role == "tool" {
			// Tool 结果消息
			anthropicMsg.Role = "user"
			anthropicMsg.Content = []interface{}{
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": msg.ToolCallID,
					"content":     msg.Content,
				},
			}
		}

		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMsg)
	}

	// 转换工具定义
	if len(req.Tools) > 0 {
		anthropicReq.Tools = make([]anthropicTool, 0, len(req.Tools))
		for _, tool := range req.Tools {
			if fn, ok := tool["function"].(map[string]interface{}); ok {
				at := anthropicTool{
					Name:        fn["name"].(string),
					Description: fn["description"].(string),
					InputSchema: fn["parameters"].(map[string]interface{}),
				}
				anthropicReq.Tools = append(anthropicReq.Tools, at)
			}
		}
	}

	return anthropicReq
}

// convertResponse 转换 Anthropic 响应为 OpenAI 格式
func (p *AnthropicProvider) convertResponse(resp *anthropicResponse, model string) *llm.Response {
	result := &llm.Response{
		ID:     resp.ID,
		Object: "chat.completion",
		Model:  model,
		Choices: make([]struct {
			Index        int         `json:"index"`
			Message      llm.Message `json:"message"`
			FinishReason string      `json:"finish_reason"`
		}, 1),
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	// 转换内容
	message := llm.Message{Role: "assistant"}
	var textContent string
	var toolCalls []llm.ToolCall

	for _, content := range resp.Content {
		switch content.Type {
		case "text":
			textContent += content.Text
		case "tool_use":
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:   content.ID,
				Type: "function",
				Function: llm.FunctionCall{
					Name:      content.Name,
					Arguments: content.Input,
				},
			})
		}
	}

	message.Content = textContent
	message.ToolCalls = toolCalls

	result.Choices[0].Message = message
	result.Choices[0].Index = 0
	result.Choices[0].FinishReason = resp.StopReason
	if resp.StopReason == "tool_use" {
		result.Choices[0].FinishReason = "tool_calls"
	}

	return result
}

// convertStreamEvent 转换流式事件
func (p *AnthropicProvider) convertStreamEvent(event *anthropicStreamEvent) *llm.StreamEvent {
	switch event.Type {
	case "content_block_delta":
		if event.Delta != nil && event.Delta.Type == "text_delta" {
			return &llm.StreamEvent{
				ID:    "",
				Model: p.model,
				Choices: []struct {
					Index        int       `json:"index"`
					Delta        llm.Delta `json:"delta"`
					FinishReason string    `json:"finish_reason"`
				}{
					{
						Index: event.Index,
						Delta: llm.Delta{
							Content: event.Delta.Text,
						},
					},
				},
			}
		}
	case "message_start":
		if event.Message != nil {
			return &llm.StreamEvent{
				ID:    event.Message.ID,
				Model: event.Message.Model,
				Choices: []struct {
					Index        int       `json:"index"`
					Delta        llm.Delta `json:"delta"`
					FinishReason string    `json:"finish_reason"`
				}{
					{
						Index: 0,
						Delta: llm.Delta{
							Role: "assistant",
						},
					},
				},
			}
		}
	}
	return nil
}
