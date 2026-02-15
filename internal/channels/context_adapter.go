package channels

import (
	"context"

	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/stream"
)

// ContextAdapter 带渠道上下文的 Agent 适配器
// 在处理消息前设置 cron 工具的渠道上下文
type ContextAdapter struct {
	handler     MessageHandler
	cronWrapper *tools.CronServiceWrapper
}

// NewContextAdapter 创建上下文适配器
func NewContextAdapter(handler MessageHandler, cronWrapper *tools.CronServiceWrapper) *ContextAdapter {
	return &ContextAdapter{
		handler:     handler,
		cronWrapper: cronWrapper,
	}
}

// HandleMessage 实现 MessageHandler 接口
func (a *ContextAdapter) HandleMessage(ctx context.Context, msg *Message) (string, error) {
	logger.Info("ContextAdapter.HandleMessage called: Channel=%s, UserID=%s", msg.Channel, msg.UserID)
	// 设置 cron 工具的渠道上下文
	if a.cronWrapper != nil {
		if msg.Channel != "" {
			a.cronWrapper.SetChannelContext(msg.Channel, msg.UserID)
			logger.Info("ContextAdapter: set channel context - Channel=%s, UserID=%s", msg.Channel, msg.UserID)
		} else {
			logger.Warn("ContextAdapter: msg.Channel is empty!")
		}
	} else {
		logger.Warn("ContextAdapter: cronWrapper is nil!")
	}

	return a.handler.HandleMessage(ctx, msg)
}

// HandleMessageStream 实现 StreamingMessageHandler 接口
func (a *ContextAdapter) HandleMessageStream(ctx context.Context, msg *Message, callback stream.StreamCallback) error {
	logger.Info("ContextAdapter.HandleMessageStream called: Channel=%s, UserID=%s", msg.Channel, msg.UserID)
	// 设置 cron 工具的渠道上下文
	if a.cronWrapper != nil {
		if msg.Channel != "" {
			a.cronWrapper.SetChannelContext(msg.Channel, msg.UserID)
			logger.Info("ContextAdapter: set channel context (stream) - Channel=%s, UserID=%s", msg.Channel, msg.UserID)
		} else {
			logger.Warn("ContextAdapter: msg.Channel is empty!")
		}
	} else {
		logger.Warn("ContextAdapter: cronWrapper is nil!")
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
