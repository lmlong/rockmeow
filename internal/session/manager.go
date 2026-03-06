// Package session 会话管理
package session

import (
	"context"
	"fmt"
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

	// store 持久化存储（可选）
	store memory.Store

	// processingMu 用于保证同一会话的消息串行处理
	// 防止并发消息导致会话历史出现连续的 user 消息
	processingMu sync.Mutex

	// messagesMu 保护 Messages 和 UpdatedAt 的并发访问
	messagesMu sync.RWMutex

	// isProcessing 标记是否正在处理消息
	isProcessing bool

	// lockedAt 锁定时间，用于超时检测
	lockedAt time.Time

	// forceUnlock 强制解锁信号通道
	forceUnlock chan struct{}

	// forceUnlockMu 保护 forceUnlock 通道的并发操作
	forceUnlockMu sync.Mutex
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
		Key:         key,
		Messages:    make([]*memory.Message, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		forceUnlock: make(chan struct{}),
	}

	// 尝试从存储加载历史消息
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		msgs, err := m.store.Get(ctx, key, m.window)
		if err == nil && len(msgs) > 0 {
			s.Messages = msgs
			logger.Info("Session restored from store", "key", key, "messageCount", len(msgs))
		}
	}

	m.sessions[key] = s
	logger.Info("Session created", "key", key)
	return s
}

// PersistMessage 持久化消息到存储
func (m *Manager) PersistMessage(sessionKey string, msg *memory.Message) {
	if m.store == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := m.store.Add(ctx, sessionKey, msg); err != nil {
		logger.Warn("Failed to persist message", "session", sessionKey, "error", err)
	}
}

// AddMessageWithPersist 添加消息并持久化
func (m *Manager) AddMessageWithPersist(sessionKey, role, content string) {
	s := m.GetOrCreate(sessionKey)
	s.AddMessage(role, content)

	// 持久化到存储
	msg := &memory.Message{
		ID:        generateID(),
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}
	m.PersistMessage(sessionKey, msg)
}

// AddMessageWithMediaAndPersist 添加带媒体的消息并持久化
func (m *Manager) AddMessageWithMediaAndPersist(sessionKey, role, content string, media []string) {
	s := m.GetOrCreate(sessionKey)
	s.AddMessageWithMedia(role, content, media)

	// 持久化到存储
	msg := &memory.Message{
		ID:        generateID(),
		Role:      role,
		Content:   content,
		Media:     media,
		Timestamp: time.Now(),
	}
	m.PersistMessage(sessionKey, msg)
}

// ClearSession 清空会话（同时清除存储）
func (m *Manager) ClearSession(sessionKey string) {
	m.mu.RLock()
	s, ok := m.sessions[sessionKey]
	m.mu.RUnlock()

	if !ok {
		return
	}

	s.Clear()

	// 清除存储中的会话
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.store.Clear(ctx, sessionKey); err != nil {
			logger.Warn("Failed to clear session from store", "session", sessionKey, "error", err)
		}
	}
}

// AddMessage 添加消息
func (s *Session) AddMessage(role, content string) {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

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
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

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
	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()

	if window <= 0 || len(s.Messages) <= window {
		return s.Messages
	}
	return s.Messages[len(s.Messages)-window:]
}

// Clear 清空会话
func (s *Session) Clear() {
	s.messagesMu.Lock()
	defer s.messagesMu.Unlock()

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
		// 加锁保护 forceUnlock 通道操作
		s.forceUnlockMu.Lock()
		close(s.forceUnlock)
		s.forceUnlock = make(chan struct{})
		s.forceUnlockMu.Unlock()
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
	s.forceUnlockMu.Lock()
	defer s.forceUnlockMu.Unlock()
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

// ========== API 所需方法 ==========

// SessionInfo 会话信息（用于 API 响应）
type SessionInfo struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	AgentID      string    `json:"agent_id"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SessionDetail 会话详情（用于 API 响应）
type SessionDetail struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	AgentID      string            `json:"agent_id"`
	Messages     []*SessionMessage `json:"messages"`
	MessageCount int               `json:"message_count"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// SessionMessage 会话消息（用于 API 响应）
type SessionMessage struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Media     []string  `json:"media,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// List 列出会话
func (m *Manager) List(limit, offset int, agentID string) []SessionInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []SessionInfo
	count := 0

	for _, s := range m.sessions {
		if count < offset {
			count++
			continue
		}
		if len(result) >= limit {
			break
		}

		s.messagesMu.RLock()
		info := SessionInfo{
			ID:           s.Key,
			Title:        generateSessionTitle(s),
			AgentID:      "default",
			MessageCount: len(s.Messages),
			CreatedAt:    s.CreatedAt,
			UpdatedAt:    s.UpdatedAt,
		}
		s.messagesMu.RUnlock()

		result = append(result, info)
		count++
	}

	return result
}

// Count 获取会话总数
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// Get 获取会话详情
func (m *Manager) Get(sessionID string) (*SessionDetail, error) {
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	s.messagesMu.RLock()
	defer s.messagesMu.RUnlock()

	messages := make([]*SessionMessage, len(s.Messages))
	for i, msg := range s.Messages {
		messages[i] = &SessionMessage{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			Media:     msg.Media,
			CreatedAt: msg.Timestamp,
		}
	}

	return &SessionDetail{
		ID:           s.Key,
		Title:        generateSessionTitle(s),
		AgentID:      "default",
		Messages:     messages,
		MessageCount: len(s.Messages),
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}, nil
}

// Delete 删除会话
func (m *Manager) Delete(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	// 清除存储
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.store.Clear(ctx, sessionID)
	}

	// 清空会话消息
	s.messagesMu.Lock()
	s.Messages = nil
	s.messagesMu.Unlock()

	delete(m.sessions, sessionID)
	logger.Info("Session deleted", "sessionId", sessionID)
	return nil
}

// ClearHistory 清空会话历史
func (m *Manager) ClearHistory(sessionID string) error {
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	s.Clear()

	// 清除存储
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		m.store.Clear(ctx, sessionID)
	}

	logger.Info("Session history cleared", "sessionId", sessionID)
	return nil
}

// generateSessionTitle 生成会话标题
func generateSessionTitle(s *Session) string {
	if len(s.Messages) == 0 {
		return "New Conversation"
	}

	// 使用第一条用户消息作为标题
	for _, msg := range s.Messages {
		if msg.Role == "user" {
			title := msg.Content
			if len(title) > 50 {
				return title[:47] + "..."
			}
			return title
		}
	}

	return "Conversation"
}
