package channels

import (
	"context"

	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/stream"
)

// ContextSetter 定义设置上下文的接口
type ContextSetter interface {
	SetContext(channel, chatID string)
}

// ContextAdapter 带渠道上下文的 Agent 适配器
// 在处理消息前设置工具的渠道上下文
type ContextAdapter struct {
	handler     MessageHandler
	cronWrapper *tools.CronServiceWrapper
	messageTool ContextSetter
}

// NewContextAdapter 创建上下文适配器
func NewContextAdapter(handler MessageHandler, cronWrapper *tools.CronServiceWrapper) *ContextAdapter {
	return &ContextAdapter{
		handler:     handler,
		cronWrapper: cronWrapper,
	}
}

// SetMessageTool 设置消息工具
func (a *ContextAdapter) SetMessageTool(tool ContextSetter) {
	a.messageTool = tool
}

// HandleMessage 实现 MessageHandler 接口
func (a *ContextAdapter) HandleMessage(ctx context.Context, msg *Message) (string, error) {
	logger.Debug("ContextAdapter.HandleMessage", "channel", msg.Channel, "userID", msg.UserID)

	// 设置 cron 工具的渠道上下文
	if a.cronWrapper != nil && msg.Channel != "" {
		a.cronWrapper.SetChannelContext(msg.Channel, msg.UserID)
	}

	// 设置 message 工具的渠道上下文
	if a.messageTool != nil && msg.Channel != "" {
		a.messageTool.SetContext(msg.Channel, msg.UserID)
	}

	return a.handler.HandleMessage(ctx, msg)
}

// HandleMessageStream 实现 StreamingMessageHandler 接口
func (a *ContextAdapter) HandleMessageStream(ctx context.Context, msg *Message, callback stream.StreamCallback) error {
	logger.Debug("ContextAdapter.HandleMessageStream", "channel", msg.Channel, "userID", msg.UserID)

	// 设置 cron 工具的渠道上下文
	if a.cronWrapper != nil && msg.Channel != "" {
		a.cronWrapper.SetChannelContext(msg.Channel, msg.UserID)
	}

	// 设置 message 工具的渠道上下文
	if a.messageTool != nil && msg.Channel != "" {
		a.messageTool.SetContext(msg.Channel, msg.UserID)
	}

	// 检查是否支持流式处理
	if sh, ok := a.handler.(StreamingMessageHandler); ok {
		return sh.HandleMessageStream(ctx, msg, callback)
	}

	// 降级为非流式处理
	response, err := a.handler.HandleMessage(ctx, msg)
	if err != nil {
		callback(stream.NewErrorEvent(err))
		return err
	}
	callback(stream.NewTextEvent(response))
	callback(stream.NewDoneEvent())
	return nil
}
