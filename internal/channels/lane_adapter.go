package channels

import (
	"context"
	"fmt"
	"time"

	"github.com/lingguard/internal/session"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/stream"
)

// LaneAdapter 带 Session Lane 的消息处理器
// 实现纯 steer 模式：
// 1. 如果正在执行且可注入 → 直接注入
// 2. 如果无法注入 → 入队等待
// 3. 队列为空时收到新消息 → 开始执行（调用被包装的 handler)
type LaneAdapter struct {
	laneManager *session.LaneManager
	handler     MessageHandler // 被包装的处理器（通常是 AgentAdapter）
}

// NewLaneAdapter 创建 Lane 适配器
func NewLaneAdapter(laneManager *session.LaneManager, handler MessageHandler) *LaneAdapter {
	return &LaneAdapter{
		laneManager: laneManager,
		handler:     handler,
	}
}

// SetOnEnqueue 设置消息入队时的回调
func (a *LaneAdapter) SetOnEnqueue(callback func(sessionID string, result session.EnqueueResult, queueLength int, previews []string)) {
	a.laneManager.SetOnEnqueue(callback)
}

// SetOnExecuting 设置开始执行时的回调
func (a *LaneAdapter) SetOnExecuting(callback func(sessionID string, messageCount int)) {
	a.laneManager.SetOnExecuting(callback)
}

// HandleMessage 实现 MessageHandler 接口
func (a *LaneAdapter) HandleMessage(ctx context.Context, msg *Message) (string, error) {
	// 尝试注入或入队
	result := a.laneManager.Enqueue(msg.SessionID, msg.Content, msg.Media, nil)

	switch result {
	case session.EnqueueSteered:
		// 成功注入到当前执行，但需要确认 Agent 会处理
		// 因为可能存在时序问题：消息注入成功，但 Agent 刚好结束执行
		return a.handleSteered(ctx, msg.SessionID)

	case session.EnqueueQueued:
		// 入队等待，检查是否需要触发执行
		if !a.laneManager.IsExecuting(msg.SessionID) {
			// 没有执行中的任务，开始执行
			return a.startExecution(ctx, msg.SessionID)
		}
		// 有执行中的任务，消息已入队，等待下次执行
		queueLen := a.laneManager.GetQueueLength(msg.SessionID)
		return fmt.Sprintf("📝 消息已入队（队列长度：%d），将在当前任务完成后处理。", queueLen), nil

	default:
		return "", fmt.Errorf("unknown enqueue result: %v", result)
	}
}

// HandleMessageStream 实现 StreamingMessageHandler 接口
func (a *LaneAdapter) HandleMessageStream(ctx context.Context, msg *Message, callback stream.StreamCallback) error {
	// 尝试注入/入队
	result := a.laneManager.Enqueue(msg.SessionID, msg.Content, msg.Media, nil)

	switch result {
	case session.EnqueueSteered:
		// 成功注入到当前执行，但需要确认 Agent 会处理
		return a.handleSteeredStream(ctx, msg.SessionID, callback)

	case session.EnqueueQueued:
		// 入队等待，检查是否需要触发执行
		if !a.laneManager.IsExecuting(msg.SessionID) {
			// 没有执行中的任务，开始执行
			return a.startExecutionStream(ctx, msg.SessionID, callback)
		}
		// 有执行中的任务，消息已入队，等待下次执行
		queueLen := a.laneManager.GetQueueLength(msg.SessionID)
		callback(stream.NewTextEvent(fmt.Sprintf("📝 消息已入队（队列长度：%d），将在当前任务完成后处理。", queueLen)))
		callback(stream.NewDoneEvent())
		return nil

	default:
		err := fmt.Errorf("unknown enqueue result: %v", result)
		callback(stream.NewErrorEvent(err))
		return err
	}
}

// startExecution 开始执行队列中的消息
func (a *LaneAdapter) startExecution(ctx context.Context, sessionID string) (string, error) {
	// 获取队列中的消息
	content, media, count := a.laneManager.StartExecution(sessionID)
	if count == 0 {
		return "", nil
	}

	logger.Info("Starting execution", "session", sessionID, "messageCount", count)

	// 调用被包装的处理器执行
	msg := &Message{
		SessionID: sessionID,
		Content:   content,
		Media:     media,
	}

	result, err := a.handler.HandleMessage(ctx, msg)

	// 标记执行完成
	a.laneManager.EndExecution(sessionID)

	// 检查是否有新的排队消息，如果有，递归执行
	if a.laneManager.HasPending(sessionID) && !a.laneManager.IsExecuting(sessionID) {
		go func() {
			// 异步执行下一批
			a.startExecution(context.Background(), sessionID)
		}()
	}

	return result, err
}

// startExecutionStream 开始执行队列中的消息（流式）
func (a *LaneAdapter) startExecutionStream(ctx context.Context, sessionID string, callback stream.StreamCallback) error {
	// 获取队列中的消息
	content, media, count := a.laneManager.StartExecution(sessionID)
	if count == 0 {
		callback(stream.NewDoneEvent())
		return nil
	}

	logger.Info("Starting execution (stream)", "session", sessionID, "messageCount", count)

	// 调用被包装的处理器执行
	msg := &Message{
		SessionID: sessionID,
		Content:   content,
		Media:     media,
	}

	// 尝试转换为 StreamingMessageHandler
	if streamHandler, ok := a.handler.(StreamingMessageHandler); ok {
		err := streamHandler.HandleMessageStream(ctx, msg, callback)
		a.laneManager.EndExecution(sessionID)
		return err
	}

	// 如果不支持流式，使用非流式
	result, err := a.handler.HandleMessage(ctx, msg)
	a.laneManager.EndExecution(sessionID)

	if err != nil {
		callback(stream.NewErrorEvent(err))
		return err
	}

	callback(stream.NewTextEvent(result))
	callback(stream.NewDoneEvent())
	return nil
}

// HasPending 检查是否有待处理消息
func (a *LaneAdapter) HasPending(sessionID string) bool {
	return a.laneManager.HasPending(sessionID)
}

// GetStats 获取统计信息
func (a *LaneAdapter) GetStats() map[string]interface{} {
	return a.laneManager.GetStats()
}

// GetQueueLength 获取队列长度
func (a *LaneAdapter) GetQueueLength(sessionID string) int {
	return a.laneManager.GetQueueLength(sessionID)
}

// IsExecuting 检查是否正在执行
func (a *LaneAdapter) IsExecuting(sessionID string) bool {
	return a.laneManager.IsExecuting(sessionID)
}

// GetLaneManager 获取 Lane 管理器
func (a *LaneAdapter) GetLaneManager() *session.LaneManager {
	return a.laneManager
}

// handleSteered 处理 Steer 成功后的时序问题
// 消息已注入到 Agent 的 injectionCh，但 Agent 可能在注入后立即结束执行
// 需要等待确认 Agent 仍在执行，否则需要取回消息并触发新执行
func (a *LaneAdapter) handleSteered(ctx context.Context, sessionID string) (string, error) {
	// 等待一小段时间，检查 Agent 是否仍在执行
	// 如果 Agent 在这段时间内结束执行，说明消息可能不会被处理
	time.Sleep(200 * time.Millisecond)

	// 检查 Agent 是否仍在执行
	if a.laneManager.IsInjectorExecuting(sessionID) {
		// Agent 仍在执行，消息会被处理
		logger.Info("Message steered and confirmed", "session", sessionID)
		return "✅ 补充说明已注入当前对话。", nil
	}

	// Agent 已结束执行，消息可能还在 injectionCh 中
	// 尝试从注入通道取回消息并入队
	recovered := a.laneManager.DrainInjectionToQueue(sessionID)
	if recovered > 0 {
		logger.Info("Recovered injected message after Agent ended", "session", sessionID, "count", recovered)
		// 触发新执行
		return a.startExecution(ctx, sessionID)
	}

	// 没有取回消息，可能已被 Agent 处理
	return "✅ 补充说明已注入当前对话。", nil
}

// handleSteeredStream 处理 Steer 成功后的时序问题（流式版本）
func (a *LaneAdapter) handleSteeredStream(ctx context.Context, sessionID string, callback stream.StreamCallback) error {
	// 等待一小段时间，检查 Agent 是否仍在执行
	time.Sleep(200 * time.Millisecond)

	// 检查 Agent 是否仍在执行
	if a.laneManager.IsInjectorExecuting(sessionID) {
		// Agent 仍在执行，消息已被注入，Agent 会在后续迭代中处理
		// 不发送任何响应，让原始的 Agent 执行继续，它会通过原始的 callback 发送响应
		logger.Info("Message steered, Agent still executing", "session", sessionID)
		return nil
	}

	// Agent 已结束执行，尝试取回消息
	recovered := a.laneManager.DrainInjectionToQueue(sessionID)
	if recovered > 0 {
		logger.Info("Recovered injected message after Agent ended (stream)", "session", sessionID, "count", recovered)
		// 触发新执行
		return a.startExecutionStream(ctx, sessionID, callback)
	}

	// 没有取回消息，可能已被 Agent 处理
	callback(stream.NewTextEvent("✅ 补充说明已注入当前对话。"))
	callback(stream.NewDoneEvent())
	return nil
}
