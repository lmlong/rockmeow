// Package memory - 会话存储实现（每个会话一个JSON文件）
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SessionFile 会话文件结构
type SessionFile struct {
	SessionID string     `json:"sessionId"`
	Messages  []*Message `json:"messages"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// SessionStore 会话存储（每个会话一个JSON文件）
type SessionStore struct {
	memoryDir string
	mu        sync.RWMutex
	cache     map[string]*SessionFile // 内存缓存
}

// NewSessionStore 创建会话存储
func NewSessionStore(memoryDir string) *SessionStore {
	expandedDir := expandHome(memoryDir)

	store := &SessionStore{
		memoryDir: expandedDir,
		cache:     make(map[string]*SessionFile),
	}

	// 确保目录存在
	os.MkdirAll(expandedDir, 0755)

	return store
}

// Add 添加消息到会话
func (s *SessionStore) Add(ctx context.Context, sessionID string, msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 获取或加载会话文件
	session, err := s.loadSession(sessionID)
	if err != nil {
		return err
	}

	// 设置时间戳
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// 生成消息ID
	if msg.ID == "" {
		msg.ID = generateMessageID()
	}

	// 添加消息
	session.Messages = append(session.Messages, msg)
	session.UpdatedAt = time.Now()

	// 保存到文件
	return s.saveSession(session)
}

// Get 获取会话消息
func (s *SessionStore) Get(ctx context.Context, sessionID string, limit int) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, err := s.loadSession(sessionID)
	if err != nil {
		return nil, err
	}

	if session == nil || len(session.Messages) == 0 {
		return []*Message{}, nil
	}

	// 限制返回数量
	if limit > 0 && len(session.Messages) > limit {
		return session.Messages[len(session.Messages)-limit:], nil
	}

	return session.Messages, nil
}

// Clear 清除会话
func (s *SessionStore) Clear(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从缓存中删除
	delete(s.cache, sessionID)

	// 删除文件
	filePath := s.getSessionFilePath(sessionID)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}

	return os.Remove(filePath)
}

// Close 关闭存储
func (s *SessionStore) Close() error {
	// 刷新所有缓存到文件
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range s.cache {
		if err := s.saveSession(session); err != nil {
			return err
		}
	}

	return nil
}

// loadSession 加载会话（如果不在缓存中则从文件加载）
func (s *SessionStore) loadSession(sessionID string) (*SessionFile, error) {
	// 检查缓存
	if session, ok := s.cache[sessionID]; ok {
		return session, nil
	}

	// 从文件加载
	filePath := s.getSessionFilePath(sessionID)

	// 如果文件不存在，创建新会话
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		session := &SessionFile{
			SessionID: sessionID,
			Messages:  make([]*Message, 0),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		s.cache[sessionID] = session
		return session, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read session file: %w", err)
	}

	var session SessionFile
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("parse session file: %w", err)
	}

	// 确保切片不为 nil
	if session.Messages == nil {
		session.Messages = make([]*Message, 0)
	}

	s.cache[sessionID] = &session
	return &session, nil
}

// saveSession 保存会话到文件
func (s *SessionStore) saveSession(session *SessionFile) error {
	// 获取文件路径（会自动创建渠道子目录）
	filePath := s.getSessionFilePath(session.SessionID)
	sessionsDir := filepath.Dir(filePath)

	// 确保目录存在
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("create sessions directory: %w", err)
	}

	// 序列化
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write session file: %w", err)
	}

	return nil
}

// getSessionFilePath 获取会话文件路径
// 根据 sessionID 前缀自动分类到渠道子目录
func (s *SessionStore) getSessionFilePath(sessionID string) string {
	// 使用安全的文件名（替换可能的问题字符）
	safeID := sanitizeFilename(sessionID)

	// 根据 sessionID 前缀确定渠道子目录
	channel := "common" // 默认子目录
	switch {
	case startsWithPrefix(sessionID, "feishu-"):
		channel = "feishu"
	case startsWithPrefix(sessionID, "webchat-"):
		channel = "webchat"
	case startsWithPrefix(sessionID, "qq-"):
		channel = "qq"
	case startsWithPrefix(sessionID, "session-"):
		// webchat 旧格式兼容
		channel = "webchat"
	}

	return filepath.Join(s.memoryDir, "sessions", channel, safeID+".json")
}

// startsWithPrefix 检查字符串是否以指定前缀开头（不区分大小写）
func startsWithPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// sanitizeFilename 清理文件名，移除不安全字符
func sanitizeFilename(name string) string {
	result := make([]byte, 0, len(name))
	for _, c := range []byte(name) {
		switch {
		case c >= 'a' && c <= 'z':
			result = append(result, c)
		case c >= 'A' && c <= 'Z':
			result = append(result, c)
		case c >= '0' && c <= '9':
			result = append(result, c)
		case c == '-' || c == '_' || c == '.':
			result = append(result, c)
		default:
			// 其他字符替换为下划线
			result = append(result, '_')
		}
	}
	return string(result)
}

// generateMessageID 生成消息ID
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// ListSessions 列出所有会话
func (s *SessionStore) ListSessions() ([]string, error) {
	sessionsDir := filepath.Join(s.memoryDir, "sessions")

	// 扫描所有子目录
	var sessions []string
	err := filepath.Walk(sessionsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			// 去掉 .json 后缀
			filename := filepath.Base(path)
			sessionID := filename[:len(filename)-5]
			sessions = append(sessions, sessionID)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	return sessions, nil
}

// GetSessionCount 获取会话数量
func (s *SessionStore) GetSessionCount(sessionID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, err := s.loadSession(sessionID)
	if err != nil {
		return 0, err
	}

	return len(session.Messages), nil
}

// 确保 SessionStore 实现 Store 接口
var _ Store = (*SessionStore)(nil)
