// Package memory 记忆系统
package memory

import (
	"context"
	"sync"
	"time"
)

// Message 记忆消息
type Message struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"` // user, assistant, system, tool
	Content   string                 `json:"content"`
	Media     []string               `json:"media,omitempty"` // 媒体文件路径列表
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Store 记忆存储接口
type Store interface {
	// Add 添加消息
	Add(ctx context.Context, sessionID string, msg *Message) error

	// Get 获取消息
	Get(ctx context.Context, sessionID string, limit int) ([]*Message, error)

	// Clear 清除会话记忆
	Clear(ctx context.Context, sessionID string) error

	// Close 关闭存储
	Close() error
}

// MemoryStore 内存存储实现
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string][]*Message
}

// NewMemoryStore 创建内存存储
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string][]*Message),
	}
}

func (s *MemoryStore) Add(ctx context.Context, sessionID string, msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	s.sessions[sessionID] = append(s.sessions[sessionID], msg)
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, sessionID string, limit int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	msgs := s.sessions[sessionID]
	if limit > 0 && len(msgs) > limit {
		start := len(msgs) - limit
		return msgs[start:], nil
	}
	return msgs, nil
}

func (s *MemoryStore) Clear(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, sessionID)
	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}
