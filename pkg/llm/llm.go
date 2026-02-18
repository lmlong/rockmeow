// Package llm LLM 客户端封装
package llm

import (
	"encoding/json"
)

// Message LLM 消息
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`

	// 多模态内容（当 ContentParts 非空时使用，Content 会被忽略）
	ContentParts []ContentPart `json:"-"`
}

// ContentPart 多模态内容部分
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
	VideoURL *VideoURL `json:"video_url,omitempty"` // 视频URL（Qwen-Omni）
	Video    []string  `json:"video,omitempty"`     // 视频帧URL列表（Qwen-VL）
}

// ImageURL 图片 URL
type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "low", "high", "auto"
}

// VideoURL 视频 URL（用于 Qwen-Omni 模型）
type VideoURL struct {
	URL string `json:"url"`
}

// MarshalJSON 自定义 JSON 序列化，支持多模态内容
func (m Message) MarshalJSON() ([]byte, error) {
	// 使用匿名结构体来控制序列化
	type Alias Message

	if len(m.ContentParts) > 0 {
		// 多模态消息：content 是数组
		return json.Marshal(struct {
			Role       string        `json:"role"`
			Content    []ContentPart `json:"content"`
			ToolCalls  []ToolCall    `json:"tool_calls,omitempty"`
			ToolCallID string        `json:"tool_call_id,omitempty"`
			Name       string        `json:"name,omitempty"`
		}{
			Role:       m.Role,
			Content:    m.ContentParts,
			ToolCalls:  m.ToolCalls,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		})
	}

	// 普通消息：使用默认序列化
	return json.Marshal(struct {
		Role       string     `json:"role"`
		Content    string     `json:"content,omitempty"`
		ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
		ToolCallID string     `json:"tool_call_id,omitempty"`
		Name       string     `json:"name,omitempty"`
	}{
		Role:       m.Role,
		Content:    m.Content,
		ToolCalls:  m.ToolCalls,
		ToolCallID: m.ToolCallID,
		Name:       m.Name,
	})
}

// UnmarshalJSON 自定义 JSON 反序列化，处理 content 为对象的情况
// 某些模型（如 Qwen3.5-Plus, DeepSeek R1）的响应中 content 可能是：
// - string: "普通文本"
// - object: {"text": "内容", "reasoning": "思考过程"}
// - array: [{"type": "text", "text": "内容"}]
func (m *Message) UnmarshalJSON(data []byte) error {
	// 临时结构体，content 使用 RawMessage 来灵活处理
	type tempMsg struct {
		Role       string          `json:"role"`
		Content    json.RawMessage `json:"content,omitempty"`
		ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
		ToolCallID string          `json:"tool_call_id,omitempty"`
		Name       string          `json:"name,omitempty"`
	}

	var temp tempMsg
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	m.Role = temp.Role
	m.ToolCalls = temp.ToolCalls
	m.ToolCallID = temp.ToolCallID
	m.Name = temp.Name

	// 处理 content 字段
	if len(temp.Content) > 0 {
		// 尝试解析为字符串
		var strContent string
		if err := json.Unmarshal(temp.Content, &strContent); err == nil {
			m.Content = strContent
		} else {
			// 尝试解析为对象（Qwen/DeepSeek reasoning 格式）
			var objContent struct {
				Text      string `json:"text"`
				Reasoning string `json:"reasoning"`
			}
			if err := json.Unmarshal(temp.Content, &objContent); err == nil {
				// 使用 text 字段作为内容，reasoning 可以选择性添加
				if objContent.Text != "" {
					m.Content = objContent.Text
				} else if objContent.Reasoning != "" {
					// 如果只有 reasoning，使用 reasoning 作为内容
					m.Content = objContent.Reasoning
				}
			} else {
				// 尝试解析为数组（多模态格式）
				var arrContent []ContentPart
				if err := json.Unmarshal(temp.Content, &arrContent); err == nil {
					m.ContentParts = arrContent
					// 提取文本内容
					for _, part := range arrContent {
						if part.Type == "text" && part.Text != "" {
							m.Content = part.Text
							break
						}
					}
				}
				// 如果都失败了，保持 Content 为空
			}
		}
	}

	return nil
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction 工具函数定义
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// Request LLM 请求
type Request struct {
	Model       string                   `json:"model"`
	Messages    []Message                `json:"messages"`
	Tools       []map[string]interface{} `json:"tools,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
}

// Response LLM 响应
type Response struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage Usage `json:"usage"`
}

// Usage Token 使用量
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamEvent 流式事件
type StreamEvent struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		Delta        Delta  `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Delta 流式增量
type Delta struct {
	Role      string          `json:"role,omitempty"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []DeltaToolCall `json:"tool_calls,omitempty"`
}

// DeltaToolCall 流式增量中的工具调用（包含 index 字段）
type DeltaToolCall struct {
	Index    int           `json:"index"`
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function DeltaFunction `json:"function"`
}

// DeltaFunction 流式增量中的函数调用
type DeltaFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // 流式时是字符串片段，需要累积
}

// ToMessage 将响应转换为消息
func (r *Response) ToMessage() Message {
	if len(r.Choices) == 0 {
		return Message{Role: "assistant"}
	}
	return r.Choices[0].Message
}

// GetContent 获取响应内容
func (r *Response) GetContent() string {
	if len(r.Choices) == 0 {
		return ""
	}
	return r.Choices[0].Message.Content
}

// GetToolCalls 获取工具调用
func (r *Response) GetToolCalls() []ToolCall {
	if len(r.Choices) == 0 {
		return nil
	}
	return r.Choices[0].Message.ToolCalls
}

// HasToolCalls 检查是否有工具调用
func (r *Response) HasToolCalls() bool {
	return len(r.GetToolCalls()) > 0
}
