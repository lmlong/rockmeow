// Package providers LLM 提供商
package providers

import (
	"context"

	"github.com/lingguard/pkg/llm"
)

// Provider LLM 提供商接口
type Provider interface {
	// Name 返回提供商名称
	Name() string

	// Model 返回配置的模型名称
	Model() string

	// Complete 发送消息并获取完成响应
	Complete(ctx context.Context, req *llm.Request) (*llm.Response, error)

	// Stream 发送消息并获取流式响应
	Stream(ctx context.Context, req *llm.Request) (<-chan llm.StreamEvent, error)

	// SupportsTools 是否支持工具调用
	SupportsTools() bool

	// SupportsVision 是否支持视觉
	SupportsVision() bool
}

// ProviderConfig 提供商配置
type ProviderConfig struct {
	APIKey        string
	APIBase       string
	Model         string
	Temperature   float64
	MaxTokens     int
	Timeout       int   // 请求超时时间（秒）
	SupportsTools *bool // 是否支持工具调用，nil 表示自动检测
}
