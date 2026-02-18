// Package session 会话管理
package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/pkg/memory"
)

// Session 会话
type Session struct {
	Key       string
	Messages  []*memory.Message
	CreatedAt time.Time
	UpdatedAt time.Time
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
		return s
	}

	s := &Session{
		Key:       key,
		Messages:  make([]*memory.Message, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.sessions[key] = s
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

// generateID 生成唯一ID
func generateID() string {
	return uuid.New().String()[:8]
}
