// Package webchat - WebChat HTTP 处理器
package webchat

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lingguard/pkg/logger"
)

// SessionForUI 用于 UI 显示的会话格式
type SessionForUI struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Preview   string         `json:"preview"`
	Messages  []MessageForUI `json:"messages"`
	CreatedAt int64          `json:"createdAt"`
	UpdatedAt int64          `json:"updatedAt"`
}

// MessageForUI 用于 UI 显示的消息格式
type MessageForUI struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Complete  bool   `json:"complete"`
	Timestamp int64  `json:"timestamp"`
}

// HTTPHandler WebChat HTTP 处理器
type HTTPHandler struct {
	memoryDir string // ~/.lingguard/memory
}

// NewHTTPHandler 创建处理器
func NewHTTPHandler(memoryDir string) *HTTPHandler {
	return &HTTPHandler{memoryDir: memoryDir}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/webchat/sessions", h.handleSessions)
	mux.HandleFunc("/api/webchat/session", h.handleSession)
	logger.Info("WebChatAPI routes registered")
}

// handleSessions 处理会话列表
func (h *HTTPHandler) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getSessions(w, r)
	case http.MethodPost:
		// 前端保存不再需要，LLM 会话由后端自动管理
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSession 处理单个会话
func (h *HTTPHandler) handleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getSession(w, r)
	case http.MethodDelete:
		h.deleteSession(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getSessions 获取所有会话
func (h *HTTPHandler) getSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.loadAllSessions()
	if err != nil {
		logger.Error("Failed to get sessions", "error", err)
		http.Error(w, "Failed to get sessions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// getSession 获取单个会话
func (h *HTTPHandler) getSession(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing session id", http.StatusBadRequest)
		return
	}

	session, err := h.loadSession(id)
	if err != nil {
		logger.Error("Failed to get session", "error", err, "sessionId", id)
		http.Error(w, "Failed to get session", http.StatusInternalServerError)
		return
	}

	if session == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// deleteSession 删除会话
func (h *HTTPHandler) deleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing session id", http.StatusBadRequest)
		return
	}

	// 删除 LLM 会话文件
	sessionsDir := filepath.Join(h.memoryDir, "sessions", "webchat")
	filePath := filepath.Join(sessionsDir, id+".json")

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to delete session", "error", err, "sessionId", id)
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

// loadAllSessions 加载所有会话
func (h *HTTPHandler) loadAllSessions() ([]*SessionForUI, error) {
	sessionsDir := filepath.Join(h.memoryDir, "sessions", "webchat")

	entries, err := os.ReadDir(sessionsDir)
	if os.IsNotExist(err) {
		return []*SessionForUI{}, nil
	}
	if err != nil {
		return nil, err
	}

	var sessions []*SessionForUI
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(sessionsDir, entry.Name())
		session, err := h.loadSessionFromFile(filePath)
		if err != nil {
			logger.Warn("Failed to load session file", "file", filePath, "error", err)
			continue
		}
		if session != nil {
			sessions = append(sessions, session)
		}
	}

	return sessions, nil
}

// loadSession 加载单个会话
func (h *HTTPHandler) loadSession(id string) (*SessionForUI, error) {
	sessionsDir := filepath.Join(h.memoryDir, "sessions", "webchat")
	filePath := filepath.Join(sessionsDir, id+".json")
	return h.loadSessionFromFile(filePath)
}

// loadSessionFromFile 从文件加载会话
func (h *HTTPHandler) loadSessionFromFile(filePath string) (*SessionForUI, error) {
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// 解析 LLM 会话格式
	var llmSession struct {
		SessionID string `json:"sessionId"`
		Messages  []struct {
			ID        string `json:"id"`
			Role      string `json:"role"`
			Content   string `json:"content"`
			Timestamp string `json:"timestamp"`
		} `json:"messages"`
		CreatedAt string `json:"createdAt"`
		UpdatedAt string `json:"updatedAt"`
	}

	if err := json.Unmarshal(data, &llmSession); err != nil {
		return nil, err
	}

	// 转换为 UI 格式
	session := &SessionForUI{
		ID: llmSession.SessionID,
	}

	// 转换消息
	for _, msg := range llmSession.Messages {
		var timestamp int64
		if msg.Timestamp != "" {
			// 解析 ISO 时间格式，转换为毫秒
			if t, err := parseTimestamp(msg.Timestamp); err == nil {
				timestamp = t
			}
		}

		session.Messages = append(session.Messages, MessageForUI{
			Role:      msg.Role,
			Content:   msg.Content,
			Complete:  true,
			Timestamp: timestamp,
		})
	}

	// 生成标题和预览
	if len(session.Messages) > 0 {
		// 从第一条用户消息生成标题
		for _, msg := range session.Messages {
			if msg.Role == "user" {
				title := msg.Content
				if len(title) > 30 {
					title = title[:30] + "..."
				}
				session.Title = title
				break
			}
		}

		// 从最后一条消息生成预览
		lastMsg := session.Messages[len(session.Messages)-1]
		preview := lastMsg.Content
		if len(preview) > 50 {
			preview = preview[:50] + "..."
		}
		session.Preview = preview
	}

	// 转换时间戳
	if llmSession.CreatedAt != "" {
		if t, err := parseTimestamp(llmSession.CreatedAt); err == nil {
			session.CreatedAt = t
		}
	}
	if llmSession.UpdatedAt != "" {
		if t, err := parseTimestamp(llmSession.UpdatedAt); err == nil {
			session.UpdatedAt = t
		}
	}

	return session, nil
}

// parseTimestamp 解析 ISO 时间格式
func parseTimestamp(s string) (int64, error) {
	// 尝试解析 RFC3339Nano 格式
	t, err := time.Parse(time.RFC3339Nano, s)
	if err == nil {
		return t.UnixMilli(), nil
	}

	// 尝试解析 RFC3339 格式
	t, err = time.Parse(time.RFC3339, s)
	if err == nil {
		return t.UnixMilli(), nil
	}

	return 0, err
}
