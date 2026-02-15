// Package channels 消息渠道管理
package channels

import (
	"context"

	"github.com/lingguard/pkg/stream"
)

// Message 表示从消息平台接收的消息
type Message struct {
	ID        string         // 消息唯一ID
	SessionID string         // 会话ID (格式: "feishu-{open_id}")
	Content   string         // 消息文本内容
	Metadata  map[string]any // 平台特定元数据
}

// MessageHandler 处理消息的接口 (由 Agent 适配器实现)
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *Message) (string, error)
}

// StreamingMessageHandler 流式处理消息的接口
type StreamingMessageHandler interface {
	MessageHandler
	// HandleMessageStream 流式处理消息
	HandleMessageStream(ctx context.Context, msg *Message, callback stream.StreamCallback) error
}

// Channel 表示一个消息渠道
type Channel interface {
	Name() string
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool
}
