package handlers

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/api/sse"
	"github.com/lingguard/internal/session"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/stream"
)

// chatHeartbeatInterval Chat SSE 心跳间隔
const chatHeartbeatInterval = 15 * time.Second

// ChatHandler Chat API 处理器
type ChatHandler struct {
	agent      *agent.Agent
	sessionMgr *session.Manager
}

// NewChatHandler 创建 Chat 处理器
func NewChatHandler(ag *agent.Agent, sessionMgr *session.Manager) *ChatHandler {
	return &ChatHandler{
		agent:      ag,
		sessionMgr: sessionMgr,
	}
}

// ChatRequest 对话请求
type ChatRequest struct {
	Message      string   `json:"message" binding:"required"`
	Media        []string `json:"media,omitempty"`
	SessionID    string   `json:"session_id,omitempty"`
	Stream       bool     `json:"stream,omitempty"`
	ClearHistory bool     `json:"clear_history,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
}

// ChatResponse 对话响应
type ChatResponse struct {
	ID        string     `json:"id"`
	SessionID string     `json:"session_id"`
	AgentID   string     `json:"agent_id"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Usage     *Usage     `json:"usage,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ToolCall 工具调用记录
type ToolCall struct {
	ID     string                 `json:"id"`
	Tool   string                 `json:"tool"`
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params,omitempty"`
	Result string                 `json:"result"`
	Status string                 `json:"status"`
}

// Usage Token 使用量
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RegisterRoutes 注册路由
func (h *ChatHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/agent/chat", h.HandleChat)
}

// HandleChat 处理对话请求
func (h *ChatHandler) HandleChat(c *gin.Context) {

	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, ErrorResponse{
			Error: ErrorDetail{
				Code:    "invalid_request",
				Message: err.Error(),
			},
		})
		return
	}

	// 生成或使用会话 ID
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// 获取或创建会话
	sess := h.sessionMgr.GetOrCreate(sessionID)

	// 清空历史
	if req.ClearHistory {
		sess.Clear()
		logger.Info("Session history cleared", "sessionId", sessionID)
	}

	// 流式响应
	if req.Stream {
		h.handleStream(c, sessionID, req)
		return
	}

	// 非流式响应
	h.handleNonStream(c, sessionID, req)
}

// handleNonStream 处理非流式请求
func (h *ChatHandler) handleNonStream(c *gin.Context, sessionID string, req ChatRequest) {
	if h.agent == nil {
		c.JSON(503, ErrorResponse{
			Error: ErrorDetail{
				Code:    "service_unavailable",
				Message: "Agent service is not available",
			},
		})
		return
	}

	var content string
	var err error

	if len(req.Media) > 0 {
		content, err = h.agent.ProcessMessageWithMedia(c.Request.Context(), sessionID, req.Message, req.Media)
	} else {
		content, err = h.agent.ProcessMessage(c.Request.Context(), sessionID, req.Message)
	}

	if err != nil {
		logger.Error("Agent process message failed", "error", err, "sessionId", sessionID)
		c.JSON(500, ErrorResponse{
			Error: ErrorDetail{
				Code:    "internal_error",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(200, ChatResponse{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		AgentID:   "default",
		Content:   content,
		CreatedAt: time.Now(),
	})
}

// handleStream 处理流式请求
func (h *ChatHandler) handleStream(c *gin.Context, sessionID string, req ChatRequest) {
	if h.agent == nil {
		sse.SetupHeaders(c)
		writer := sse.NewWriter(c.Writer)
		writer.WriteEvent("error", gin.H{
			"code":    "service_unavailable",
			"message": "Agent service is not available",
		})
		return
	}

	// 设置 SSE headers
	sse.SetupHeaders(c)

	writer := sse.NewWriter(c.Writer)

	// 发送 connected 事件
	writer.WriteEvent("connected", gin.H{"session_id": sessionID})

	// 启动心跳协程（防止长任务被代理/负载均衡器断开）
	heartbeatCtx, cancelHeartbeat := context.WithCancel(c.Request.Context())
	defer cancelHeartbeat()

	stopHeartbeat := sse.HeartbeatRunner(heartbeatCtx, writer, chatHeartbeatInterval)
	defer stopHeartbeat()

	// 收集完整响应（用于工具调用记录）
	var fullContent string
	var toolCalls []ToolCall
	var toolCallID string

	// 创建流式回调
	callback := func(event stream.StreamEvent) {
		switch event.Type {
		case stream.EventText:
			fullContent += event.Content
			writer.WriteEvent("content", gin.H{"delta": event.Content})

		case stream.EventToolStart:
			toolCallID = uuid.New().String()[:8]
			writer.WriteEvent("tool_call", gin.H{
				"id":     toolCallID,
				"tool":   event.ToolName,
				"status": "running",
			})

		case stream.EventToolEnd:
			status := "completed"
			if event.ToolError != "" {
				status = "failed"
			}
			toolCalls = append(toolCalls, ToolCall{
				ID:     toolCallID,
				Tool:   event.ToolName,
				Result: event.ToolResult,
				Status: status,
			})
			writer.WriteEvent("tool_result", gin.H{
				"id":     toolCallID,
				"tool":   event.ToolName,
				"status": status,
				"result": event.ToolResult,
			})

		case stream.EventDone:
			writer.WriteEvent("completed", gin.H{
				"id":         uuid.New().String(),
				"session_id": sessionID,
				"agent_id":   "default",
			})

		case stream.EventError:
			writer.WriteEvent("error", gin.H{
				"code":    "internal_error",
				"message": event.Error.Error(),
			})
		}
	}

	var err error
	if len(req.Media) > 0 {
		err = h.agent.ProcessMessageStreamWithMedia(c.Request.Context(), sessionID, req.Message, req.Media, callback)
	} else {
		err = h.agent.ProcessMessageStream(c.Request.Context(), sessionID, req.Message, callback)
	}

	if err != nil {
		logger.Error("Agent process message stream failed", "error", err, "sessionId", sessionID)
		writer.WriteEvent("error", gin.H{
			"code":    "internal_error",
			"message": err.Error(),
		})
	}
}
