// Package webchat - WebChat 会话存储
package webchat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lingguard/pkg/logger"
	_ "modernc.org/sqlite"
)

// Message 消息
type Message struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Complete  bool   `json:"complete"`
	Timestamp int64  `json:"timestamp"`
}

// Session 会话
type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Preview   string    `json:"preview"`
	Messages  []Message `json:"messages"`
	CreatedAt int64     `json:"createdAt"`
	UpdatedAt int64     `json:"updatedAt"`
}

// Store 存储接口
type Store interface {
	GetAll() ([]*Session, error)
	Get(id string) (*Session, error)
	Save(session *Session) error
	SaveBatch(sessions []*Session) error
	Delete(id string) error
	Close() error
}

// SQLiteStore SQLite 存储
type SQLiteStore struct {
	dbPath string
	mu     sync.RWMutex
}

// NewSQLiteStore 创建 SQLite 存储
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	logger.Info("WebChat store initialized", "path", dbPath)
	return &SQLiteStore{dbPath: dbPath}, nil
}

// GetAll 获取所有会话
func (s *SQLiteStore) GetAll() ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.dbPath)
	if os.IsNotExist(err) {
		return []*Session{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read store: %w", err)
	}

	if len(data) == 0 {
		return []*Session{}, nil
	}

	var sessions []*Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("unmarshal sessions: %w", err)
	}

	return sessions, nil
}

// Get 获取单个会话
func (s *SQLiteStore) Get(id string) (*Session, error) {
	sessions, err := s.GetAll()
	if err != nil {
		return nil, err
	}

	for _, session := range sessions {
		if session.ID == id {
			return session, nil
		}
	}

	return nil, nil
}

// Save 保存会话
func (s *SQLiteStore) Save(session *Session) error {
	return s.SaveBatch([]*Session{session})
}

// SaveBatch 批量保存会话
func (s *SQLiteStore) SaveBatch(newSessions []*Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions, err := s.GetAll()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	sessionMap := make(map[string]*Session)
	for _, s := range sessions {
		sessionMap[s.ID] = s
	}

	now := time.Now().UnixMilli()
	for _, s := range newSessions {
		if s.CreatedAt == 0 {
			s.CreatedAt = now
		}
		s.UpdatedAt = now
		sessionMap[s.ID] = s
	}

	var result []*Session
	for _, s := range sessionMap {
		result = append(result, s)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sessions: %w", err)
	}

	if err := os.WriteFile(s.dbPath, data, 0644); err != nil {
		return fmt.Errorf("write store: %w", err)
	}

	return nil
}

// Delete 删除会话
func (s *SQLiteStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions, err := s.GetAll()
	if err != nil {
		return err
	}

	var result []*Session
	for _, s := range sessions {
		if s.ID != id {
			result = append(result, s)
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sessions: %w", err)
	}

	if err := os.WriteFile(s.dbPath, data, 0644); err != nil {
		return fmt.Errorf("write store: %w", err)
	}

	return nil
}

// Close 关闭存储
func (s *SQLiteStore) Close() error {
	return nil
}
