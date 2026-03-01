// Package agent Steer 模式支持
package agent

import (
	"sync"

	"github.com/lingguard/pkg/logger"
)

// InjectionMessage 注入的消息
type InjectionMessage struct {
	Content string
	Media   []string
}

// steerState 会话的 steer 状态
type steerState struct {
	mu          sync.RWMutex
	injectionCh chan InjectionMessage
	isExecuting bool
}

// steerManager 管理 steer 状态
type steerManager struct {
	mu     sync.RWMutex
	states map[string]*steerState
}

// newSteerManager 创建 steer 管理器
func newSteerManager() *steerManager {
	return &steerManager{
		states: make(map[string]*steerState),
	}
}

// getOrCreateState 获取或创建会话的 steer 状态
func (m *steerManager) getOrCreateState(sessionID string) *steerState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if state, ok := m.states[sessionID]; ok {
		return state
	}

	state := &steerState{
		injectionCh: make(chan InjectionMessage, 10), // 缓冲区大小 10
		isExecuting: false,
	}
	m.states[sessionID] = state
	return state
}

// getState 获取会话的 steer 状态
func (m *steerManager) getState(sessionID string) *steerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.states[sessionID]
}

// Inject 注入消息到正在执行的会话
func (m *steerManager) Inject(sessionID, content string, media []string) bool {
	state := m.getState(sessionID)
	if state == nil {
		return false
	}

	// 检查是否正在执行
	state.mu.RLock()
	isExecuting := state.isExecuting
	state.mu.RUnlock()

	if !isExecuting {
		return false
	}

	// 尝试注入消息（非阻塞）
	select {
	case state.injectionCh <- InjectionMessage{Content: content, Media: media}:
		logger.Info("Message injected", "session", sessionID)
		return true
	default:
		// 通道满了，注入失败
		logger.Warn("Injection channel full, message dropped", "session", sessionID)
		return false
	}
}

// IsExecuting 检查会话是否正在执行
func (m *steerManager) IsExecuting(sessionID string) bool {
	state := m.getState(sessionID)
	if state == nil {
		return false
	}

	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.isExecuting
}

// setExecuting 设置会话执行状态
func (m *steerManager) setExecuting(sessionID string, executing bool) {
	state := m.getOrCreateState(sessionID)
	state.mu.Lock()
	defer state.mu.Unlock()
	state.isExecuting = executing
	logger.Debug("Steer state updated", "session", sessionID, "isExecuting", executing)
}

// getInjectionChannel 获取注入通道
func (m *steerManager) getInjectionChannel(sessionID string) <-chan InjectionMessage {
	state := m.getOrCreateState(sessionID)
	return state.injectionCh
}

// DrainInjectionChannel 清空注入通道中的消息并返回
// 用于 Agent 结束执行后取回未被处理的消息
func (m *steerManager) DrainInjectionChannel(sessionID string) []InjectionMessage {
	state := m.getState(sessionID)
	if state == nil {
		return nil
	}

	var messages []InjectionMessage
	for {
		select {
		case msg := <-state.injectionCh:
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}
// checkInjection 非阻塞检查是否有注入的消息
func (m *steerManager) checkInjection(sessionID string) *InjectionMessage {
	state := m.getState(sessionID)
	if state == nil {
		logger.Debug("checkInjection: state is nil", "session", sessionID)
		return nil
	}

	select {
	case msg := <-state.injectionCh:
		logger.Info("checkInjection: message retrieved", "session", sessionID, "content", msg.Content[:min(50, len(msg.Content))])
		return &msg
	default:
		logger.Debug("checkInjection: no message in channel", "session", sessionID)
		return nil
	}
}
