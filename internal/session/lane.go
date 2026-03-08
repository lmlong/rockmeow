// Package session 会话管理
package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lingguard/pkg/logger"
)

// EnqueueCallback 执行完成后的回调函数
type EnqueueCallback func(result string, err error)

// QueuedMessage 排队的消息
type QueuedMessage struct {
	SessionID  string
	Content    string
	Media      []string
	Callback   EnqueueCallback
	EnqueuedAt time.Time
}

// InjectionMessage 注入的消息（用于 Steer 模式）
type InjectionMessage struct {
	Content string
	Media   []string
}

// StreamInjector 支持消息注入的执行器接口
// Agent 需要实现这个接口来支持 steer 模式
type StreamInjector interface {
	// InjectMessage 向正在执行的对话注入消息
	// 返回 true 表示注入成功，false 表示当前没有执行中的对话或无法注入
	InjectMessage(sessionID, content string, media []string) bool

	// IsExecuting 检查指定会话是否正在执行
	IsExecuting(sessionID string) bool

	// DrainInjectionChannel 清空注入通道中的消息并返回
	// 用于 Agent 结束执行后取回未被处理的消息
	DrainInjectionChannel(sessionID string) []InjectionMessage
}

// EnqueueResult 入队结
// EnqueueResult 入队结果
type EnqueueResult int

const (
	EnqueueQueued  EnqueueResult = iota // 入队等待（没有执行中的任务，或无法注入）
	EnqueueSteered                      // 成功注入到当前执行
)

// Lane 会话通道，管理消息队列和执行
// 实现纯 steer 模式：
// 1. 如果正在执行且可注入 → 直接注入
// 2. 如果无法注入 → 入队等待
// 3. 开始新执行时 → 先把队列中所有消息一起注入
type Lane struct {
	sessionID string
	injector  StreamInjector

	mu          sync.Mutex
	backlog     []*QueuedMessage // 等待队列
	isExecuting bool             // 标记 Lane 层面的执行状态
}

// NewLane 创建会话通道
func NewLane(sessionID string, injector StreamInjector) *Lane {
	return &Lane{
		sessionID: sessionID,
		injector:  injector,
		backlog:   make([]*QueuedMessage, 0),
	}
}

// Enqueue 将消息入队或注入
// 如果正在执行且可注入 → 直接注入
// 如果无法注入 → 入队等待
func (l *Lane) Enqueue(content string, media []string, callback EnqueueCallback) EnqueueResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 调试日志：记录执行状态
	laneExecuting := l.isExecuting
	var agentExecuting bool
	if l.injector != nil {
		agentExecuting = l.injector.IsExecuting(l.sessionID)
	}
	logger.Info("[Enqueue] State check", "session", l.sessionID, "laneExecuting", laneExecuting, "agentExecuting", agentExecuting)

	// 检查 Lane 自身是否正在执行
	if l.isExecuting && l.injector != nil {
		// Lane 正在执行，等待 Agent 启动（最多等待 50ms）
		// 这是为了解决 Lane.StartExecution() 和 Agent.runLoop() 之间的时序窗口
		for i := 0; i < 5; i++ {
			if l.injector.IsExecuting(l.sessionID) {
				// Agent 已启动，尝试注入
				if l.injector.InjectMessage(l.sessionID, content, media) {
					logger.Info("Message steered into current execution", "session", l.sessionID)
					return EnqueueSteered
				}
				logger.Warn("Injection failed, queuing", "session", l.sessionID)
				break
			}
			logger.Debug("[Enqueue] Waiting for Agent to start", "session", l.sessionID, "attempt", i+1)
			l.mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			l.mu.Lock()
		}
		// Agent 未启动或注入失败，入队等待
	}

	// 检查 injector 是否正在执行（非 Lane 执行时的直接检查）
	if l.injector != nil && l.injector.IsExecuting(l.sessionID) {
		// 正在执行中，尝试直接注入（steer 模式）
		if l.injector.InjectMessage(l.sessionID, content, media) {
			logger.Info("Message steered into current execution", "session", l.sessionID)
			return EnqueueSteered
		}
		// 注入失败，入队等待
	}

	// 无法注入或没有执行中的任务，入队等待
	msg := &QueuedMessage{
		SessionID:  l.sessionID,
		Content:    content,
		Media:      media,
		Callback:   callback,
		EnqueuedAt: time.Now(),
	}
	l.backlog = append(l.backlog, msg)
	logger.Info("Message queued", "session", l.sessionID, "queueLength", len(l.backlog))

	return EnqueueQueued
}

// GetQueueLength 获取队列长度
func (l *Lane) GetQueueLength() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.backlog)
}

// IsExecuting 检查是否正在执行（Lane 层面）
func (l *Lane) IsExecuting() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.isExecuting
}

// HasPending 检查是否有待处理的消息
func (l *Lane) HasPending() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.backlog) > 0
}

// GetPendingPreview 获取待处理消息的预览
func (l *Lane) GetPendingPreview() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	previews := make([]string, 0, len(l.backlog))
	for _, msg := range l.backlog {
		preview := msg.Content
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		previews = append(previews, preview)
	}
	return previews
}

// StartExecution 开始执行
// 1. 标记为执行中
// 2. 获取并清空队列中的消息
// 3. 返回合并后的内容，供执行器使用
// 调用者需要在执行完成后调用 EndExecution
func (l *Lane) StartExecution() (string, []string, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 标记为执行中
	l.isExecuting = true
	logger.Info("[Lane] Execution started", "session", l.sessionID, "queueLength", len(l.backlog))

	if len(l.backlog) == 0 {
		return "", nil, 0
	}

	// 复制并清空队列
	messages := make([]*QueuedMessage, len(l.backlog))
	copy(messages, l.backlog)
	l.backlog = make([]*QueuedMessage, 0)

	// 合并消息
	mergedContent, mergedMedia := l.mergeMessages(messages)
	return mergedContent, mergedMedia, len(messages)
}

// EndExecution 结束执行
func (l *Lane) EndExecution() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.isExecuting = false
	logger.Info("[Lane] Execution ended", "session", l.sessionID, "remainingQueue", len(l.backlog))
}

// mergeMessages 合并多个消息为一个
func (l *Lane) mergeMessages(messages []*QueuedMessage) (string, []string) {
	if len(messages) == 0 {
		return "", nil
	}

	if len(messages) == 1 {
		return messages[0].Content, messages[0].Media
	}

	// 合并内容
	var parts []string
	var allMedia []string

	for i, msg := range messages {
		// 添加消息内容
		if i == 0 {
			// 第一条消息作为主体
			parts = append(parts, msg.Content)
		} else {
			// 后续消息作为补充说明
			elapsed := time.Since(msg.EnqueuedAt).Round(time.Second)
			parts = append(parts, fmt.Sprintf("\n\n【补充说明 %s前】\n%s", elapsed, msg.Content))
		}

		// 收集所有媒体
		allMedia = append(allMedia, msg.Media...)
	}

	return strings.Join(parts, ""), allMedia
}

// Reset 重置通道，清空队列
func (l *Lane) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.backlog = make([]*QueuedMessage, 0)
	l.isExecuting = false
	logger.Info("Lane reset", "session", l.sessionID)
}

// Clear 清空等待队列（但不影响正在执行的任务）
func (l *Lane) Clear() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	count := len(l.backlog)
	l.backlog = make([]*QueuedMessage, 0)
	if count > 0 {
		logger.Info("Lane cleared", "session", l.sessionID, "clearedCount", count)
	}
	return count
}

// LaneManager 管理所有会话的 Lane
type LaneManager struct {
	mu          sync.RWMutex
	lanes       map[string]*Lane
	injector    StreamInjector
	onEnqueue   func(sessionID string, result EnqueueResult, queueLength int, previews []string)
	onExecuting func(sessionID string, messageCount int)
}

// NewLaneManager 创建 Lane 管理器
func NewLaneManager(injector StreamInjector) *LaneManager {
	return &LaneManager{
		lanes:    make(map[string]*Lane),
		injector: injector,
	}
}

// SetOnEnqueue 设置消息入队时的回调
func (m *LaneManager) SetOnEnqueue(callback func(sessionID string, result EnqueueResult, queueLength int, previews []string)) {
	m.onEnqueue = callback
}

// SetOnExecuting 设置开始执行时的回调
func (m *LaneManager) SetOnExecuting(callback func(sessionID string, messageCount int)) {
	m.onExecuting = callback
}

// GetOrCreateLane 获取或创建 Lane
func (m *LaneManager) GetOrCreateLane(sessionID string) *Lane {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lane, ok := m.lanes[sessionID]; ok {
		return lane
	}

	lane := NewLane(sessionID, m.injector)
	m.lanes[sessionID] = lane
	logger.Debug("Lane created", "session", sessionID)
	return lane
}

// Enqueue 向指定会话入队消息
func (m *LaneManager) Enqueue(sessionID, content string, media []string, callback EnqueueCallback) EnqueueResult {
	lane := m.GetOrCreateLane(sessionID)
	result := lane.Enqueue(content, media, callback)

	// 触发回调
	if m.onEnqueue != nil {
		m.onEnqueue(sessionID, result, lane.GetQueueLength(), lane.GetPendingPreview())
	}

	return result
}

// StartExecution 开始执行，返回合并后的内容
func (m *LaneManager) StartExecution(sessionID string) (string, []string, int) {
	lane := m.GetOrCreateLane(sessionID)
	content, media, count := lane.StartExecution()

	// 触发回调
	if count > 0 && m.onExecuting != nil {
		m.onExecuting(sessionID, count)
	}

	return content, media, count
}

// EndExecution 结束执行
func (m *LaneManager) EndExecution(sessionID string) {
	m.mu.RLock()
	lane, ok := m.lanes[sessionID]
	m.mu.RUnlock()

	if ok {
		lane.EndExecution()
	}
}

// GetQueueLength 获取指定会话的队列长度
func (m *LaneManager) GetQueueLength(sessionID string) int {
	m.mu.RLock()
	lane, ok := m.lanes[sessionID]
	m.mu.RUnlock()

	if !ok {
		return 0
	}
	return lane.GetQueueLength()
}

// IsExecuting 检查指定会话是否正在执行
func (m *LaneManager) IsExecuting(sessionID string) bool {
	m.mu.RLock()
	lane, ok := m.lanes[sessionID]
	m.mu.RUnlock()

	if !ok {
		return false
	}
	return lane.IsExecuting()
}

// HasPending 检查指定会话是否有待处理消息
func (m *LaneManager) HasPending(sessionID string) bool {
	m.mu.RLock()
	lane, ok := m.lanes[sessionID]
	m.mu.RUnlock()

	if !ok {
		return false
	}
	return lane.HasPending()
}

// ResetLane 重置指定会话的 Lane
func (m *LaneManager) ResetLane(sessionID string) {
	m.mu.RLock()
	lane, ok := m.lanes[sessionID]
	m.mu.RUnlock()

	if ok {
		lane.Reset()
	}
}

// ClearLane 清空指定会话的等待队列
func (m *LaneManager) ClearLane(sessionID string) int {
	m.mu.RLock()
	lane, ok := m.lanes[sessionID]
	m.mu.RUnlock()

	if !ok {
		return 0
	}
	return lane.Clear()
}

// GetStats 获取所有 Lane 的统计信息
func (m *LaneManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeCount := 0
	queuedCount := 0
	pendingSessions := 0

	for _, lane := range m.lanes {
		if lane.IsExecuting() {
			activeCount++
		}
		if lane.HasPending() {
			pendingSessions++
		}
		queuedCount += lane.GetQueueLength()
	}

	return map[string]interface{}{
		"totalLanes":      len(m.lanes),
		"activeLanes":     activeCount,
		"pendingSessions": pendingSessions,
		"queuedMessages":  queuedCount,
	}
}

// IsInjectorExecuting 检查 Injector (Agent) 是否正在执行
// 与 IsExecuting 不同，这个方法直接检查 Agent 的执行状态
func (m *LaneManager) IsInjectorExecuting(sessionID string) bool {
	if m.injector == nil {
		return false
	}
	return m.injector.IsExecuting(sessionID)
}

// DrainInjectionToQueue 清空注入通道并将消息添加到队列
// 返回取回的消息数量
func (m *LaneManager) DrainInjectionToQueue(sessionID string) int {
	if m.injector == nil {
		return 0
	}

	// 从 Agent 的注入通道中取回消息
	messages := m.injector.DrainInjectionChannel(sessionID)
	if len(messages) == 0 {
		return 0
	}

	// 获取 Lane 并将消息添加到队列
	lane := m.GetOrCreateLane(sessionID)
	lane.mu.Lock()
	defer lane.mu.Unlock()

	for _, msg := range messages {
		queuedMsg := &QueuedMessage{
			SessionID:  sessionID,
			Content:    msg.Content,
			Media:      msg.Media,
			EnqueuedAt: time.Now(),
		}
		lane.backlog = append(lane.backlog, queuedMsg)
	}

	logger.Info("Drained injection channel to queue", "session", sessionID, "count", len(messages))
	return len(messages)
}
