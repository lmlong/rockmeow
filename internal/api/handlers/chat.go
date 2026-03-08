package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lingguard/internal/agent"
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

// OpenAI 兼容的 SSE 数据结构

// OpenAIChunk OpenAI 格式的流式响应块
type OpenAIChunk struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []OpenAIChoice    `json:"choices"`
	Usage   *OpenAIChunkUsage `json:"usage,omitempty"`
}

// OpenAIChoice 选择项
type OpenAIChoice struct {
	Index        int         `json:"index"`
	Delta        OpenAIDelta `json:"delta"`
	FinishReason *string     `json:"finish_reason"`
}

// OpenAIDelta 增量内容
type OpenAIDelta struct {
	Role      string           `json:"role,omitempty"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []OpenAIToolCall `json:"tool_calls,omitempty"`
}

// OpenAIToolCall 工具调用
type OpenAIToolCall struct {
	Index    int            `json:"index"`
	ID       string         `json:"id,omitempty"`
	Type     string         `json:"type,omitempty"`
	Function OpenAIFunction `json:"function,omitempty"`
}

// OpenAIFunction 函数调用
type OpenAIFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// OpenAIToolResult 工具结果（自定义扩展）
type OpenAIToolResult struct {
	Index  int    `json:"index"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
	Status string `json:"status"`
}

// OpenAIChunkUsage Token 使用量
type OpenAIChunkUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// handleStream 处理流式请求（OpenAI 兼容格式）
func (h *ChatHandler) handleStream(c *gin.Context, sessionID string, req ChatRequest) {
	if h.agent == nil {
		setupSSEHeaders(c)
		writeOpenAIError(c, "service_unavailable", "Agent service is not available")
		return
	}

	// 设置 SSE headers
	setupSSEHeaders(c)

	// 生成响应 ID
	responseID := "chatcmpl-" + uuid.New().String()[:24]
	created := time.Now().Unix()
	model := "lingguard-agent"
	toolCallIndex := 0

	// 发送角色信息
	writeOpenAIChunk(c, OpenAIChunk{
		ID:      responseID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []OpenAIChoice{
			{
				Index: 0,
				Delta: OpenAIDelta{Role: "assistant"},
			},
		},
	})

	// 心跳定时器
	heartbeatTicker := time.NewTicker(chatHeartbeatInterval)
	defer heartbeatTicker.Stop()

	// 心跳协程
	heartbeatCtx, cancelHeartbeat := context.WithCancel(c.Request.Context())
	defer cancelHeartbeat()
	go func() {
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-heartbeatTicker.C:
				// 发送注释作为心跳（OpenAI 兼容）
				c.Writer.Write([]byte(": heartbeat\n\n"))
				c.Writer.Flush()
			}
		}
	}()

	// 收集完整响应
	var fullContent string
	var currentToolCallID string

	// 创建流式回调
	callback := func(event stream.StreamEvent) {
		switch event.Type {
		case stream.EventText:
			fullContent += event.Content
			writeOpenAIChunk(c, OpenAIChunk{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []OpenAIChoice{
					{
						Index: 0,
						Delta: OpenAIDelta{Content: event.Content},
					},
				},
			})

		case stream.EventToolStart:
			currentToolCallID = "call_" + uuid.New().String()[:24]
			writeOpenAIChunk(c, OpenAIChunk{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []OpenAIChoice{
					{
						Index: 0,
						Delta: OpenAIDelta{
							ToolCalls: []OpenAIToolCall{
								{
									Index: toolCallIndex,
									ID:    currentToolCallID,
									Type:  "function",
									Function: OpenAIFunction{
										Name: event.ToolName,
									},
								},
							},
						},
					},
				},
			})
			toolCallIndex++

		case stream.EventToolEnd:
			// 工具结果作为自定义事件发送（OpenAI 协议不包含工具结果）
			// 使用 x-tool-result 扩展事件
			status := "completed"
			if event.ToolError != "" {
				status = "failed"
			}
			writeSSEEvent(c, "x-tool-result", OpenAIToolResult{
				Index:  toolCallIndex - 1,
				ID:     currentToolCallID,
				Name:   event.ToolName,
				Result: event.ToolResult,
				Error:  event.ToolError,
				Status: status,
			})

		case stream.EventDone:
			// 发送结束块
			finishReason := "stop"
			if toolCallIndex > 0 {
				finishReason = "tool_calls"
			}
			writeOpenAIChunk(c, OpenAIChunk{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   model,
				Choices: []OpenAIChoice{
					{
						Index:        0,
						Delta:        OpenAIDelta{},
						FinishReason: &finishReason,
					},
				},
			})
			// 发送 [DONE]
			c.Writer.Write([]byte("data: [DONE]\n\n"))
			c.Writer.Flush()

		case stream.EventError:
			writeOpenAIError(c, "internal_error", event.Error.Error())
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
		writeOpenAIError(c, "internal_error", err.Error())
	}
}

// setupSSEHeaders 设置 SSE 响应头
func setupSSEHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")
}

// writeOpenAIChunk 写入 OpenAI 格式的 SSE 数据块
func writeOpenAIChunk(c *gin.Context, chunk OpenAIChunk) {
	data, err := json.Marshal(chunk)
	if err != nil {
		logger.Error("Failed to marshal OpenAI chunk", "error", err)
		return
	}
	c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
	c.Writer.Flush()
}

// writeSSEEvent 写入带事件类型的 SSE 数据
func writeSSEEvent(c *gin.Context, eventType string, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Error("Failed to marshal SSE event", "error", err)
		return
	}
	c.Writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, jsonData)))
	c.Writer.Flush()
}

// writeOpenAIError 写入 OpenAI 格式的错误
func writeOpenAIError(c *gin.Context, code, message string) {
	errorData := map[string]interface{}{
		"error": map[string]string{
			"message": message,
			"type":    "invalid_request_error",
			"code":    code,
		},
	}
	jsonData, _ := json.Marshal(errorData)
	c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", jsonData)))
	c.Writer.Flush()
}
