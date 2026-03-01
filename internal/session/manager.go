// Package session 会话管理
package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/memory"
)

// Session 会话
type Session struct {
	Key       string
	Messages  []*memory.Message
	CreatedAt time.Time
	UpdatedAt time.Time

	// processingMu 用于保证同一会话的消息串行处理
	// 防止并发消息导致会话历史出现连续的 user 消息
	processingMu sync.Mutex

	// isProcessing 标记是否正在处理消息
	isProcessing bool

	// lockedAt 锁定时间，用于超时检测
	lockedAt time.Time

	// forceUnlock 强制解锁信号通道
	forceUnlock chan struct{}
}

// Manager 会话管理器
type Manager struct {
	mu       sync.RWMutex
	store    memory.Store
	sessions map[string]*Session
	window   int // 历史消息窗口大小
}

// NewManager 创建会话管理器
func NewManager(store memory.Store, window int) *Manager {
	return &Manager{
		store:    store,
		sessions: make(map[string]*Session),
		window:   window,
	}
}

// GetOrCreate 获取或创建会话
func (m *Manager) GetOrCreate(key string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s, ok := m.sessions[key]; ok {
		logger.Debug("Session accessed", "key", key, "messageCount", len(s.Messages), "age", time.Since(s.CreatedAt).Round(time.Second))
		return s
	}

	s := &Session{
		Key:       key,
		Messages:  make([]*memory.Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		forceUnlock: make(chan struct{}),
	}
	m.sessions[key] = s
	logger.Info("Session created", "key", key)
	return s
}

// AddMessage 添加消息
func (s *Session) AddMessage(role, content string) {
	s.Messages = append(s.Messages, &memory.Message{
		ID:        generateID(),
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// AddMessageWithMedia 添加带媒体的消息
func (s *Session) AddMessageWithMedia(role, content string, media []string) {
	s.Messages = append(s.Messages, &memory.Message{
		ID:        generateID(),
		Role:      role,
		Content:   content,
		Media:     media,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// GetHistory 获取历史消息（限制窗口大小）
func (s *Session) GetHistory(window int) []*memory.Message {
	if window <= 0 || len(s.Messages) <= window {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-window:]
}

// Clear 清空会话
func (s *Session) Clear() {
	s.Messages = make([]*memory.Message, 0)
	s.UpdatedAt = time.Now()
}

// TryLockForProcessing 尝试锁定会话以进行消息处理
// 返回 true 表示成功获取锁，false 表示会话正在处理其他消息
func (s *Session) TryLockForProcessing() bool {
	if s.processingMu.TryLock() {
		s.isProcessing = true
		s.lockedAt = time.Now()
		logger.Debug("Session locked", "key", s.Key)
		return true
	}
	logger.Warn("Session busy, lock failed", "key", s.Key, "lockedAt", s.lockedAt)
	return false
}

// TryLockWithTimeout 尝试锁定会话，支持超时后强制解锁
// timeout: 锁持有超时时间，超过后可被强制释放
func (s *Session) TryLockWithTimeout(timeout time.Duration) bool {
	// 首先尝试正常获取锁
	if s.processingMu.TryLock() {
		s.isProcessing = true
		s.lockedAt = time.Now()
		logger.Debug("Session locked", "key", s.Key)
		return true
	}

	// 检查是否超时，如果超时则强制解锁
	if !s.lockedAt.IsZero() && time.Since(s.lockedAt) > timeout {
		logger.Warn("Session lock timeout, force unlocking", "key", s.Key, "lockedAt", s.lockedAt, "timeout", timeout)
		// 发送强制解锁信号
		close(s.forceUnlock)
		// 重新创建通道供下次使用
		s.forceUnlock = make(chan struct{})
		// 重置状态后重新尝试获取锁
		s.isProcessing = false
		s.lockedAt = time.Time{}
		// 递归尝试获取锁
		return s.TryLockWithTimeout(timeout)
	}

	logger.Warn("Session busy, lock failed", "key", s.Key, "lockedAt", s.lockedAt)
	return false
}

// LockForProcessing 锁定会话以进行消息处理（阻塞版本）
// 确保同一会话的消息串行处理，避免并发导致的历史消息错乱
func (s *Session) LockForProcessing() {
	s.processingMu.Lock()
	s.isProcessing = true
}

// UnlockAfterProcessing 释放会话处理锁
func (s *Session) UnlockAfterProcessing() {
	s.isProcessing = false
	lockedDuration := time.Since(s.lockedAt)
	s.lockedAt = time.Time{} // 清除锁定时间
	s.processingMu.Unlock()
	logger.Debug("Session unlocked", "key", s.Key, "lockedDuration", lockedDuration.Round(time.Millisecond))
}

// ForceUnlockChannel 返回强制解锁通道，用于监听解锁信号
func (s *Session) ForceUnlockChannel() <-chan struct{} {
	return s.forceUnlock
}

// IsProcessing 检查会话是否正在处理消息
func (s *Session) IsProcessing() bool {
	return s.isProcessing
}

// generateID 生成唯一ID
func generateID() string {
	return uuid.New().String()[:8]
}