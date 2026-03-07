package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/api/sse"
	"github.com/lingguard/internal/api/task"
	"github.com/lingguard/pkg/logger"
)

// TaskHandler Task API 处理器
type TaskHandler struct {
	manager *task.Manager
}

// NewTaskHandler 创建 Task 处理器
func NewTaskHandler(manager *task.Manager) *TaskHandler {
	return &TaskHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *TaskHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/tasks", h.CreateTask)
	r.GET("/tasks/:task_id", h.GetTask)
	r.DELETE("/tasks/:task_id", h.DeleteTask)
	r.GET("/tasks/:task_id/events", h.GetTaskEvents)
	r.GET("/tasks", h.ListTasks)
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Message     string   `json:"message" binding:"required"`
	Media       []string `json:"media,omitempty"`
	SessionID   string   `json:"session_id,omitempty"`
	AgentID     string   `json:"agent_id,omitempty"`
	Stream      bool     `json:"stream,omitempty"`
	CallbackURL string   `json:"callback_url,omitempty"`
}

// CreateTask 创建异步任务
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, ErrorResponse{
			Error: ErrorDetail{
				Code:    "invalid_request",
				Message: err.Error(),
			},
		})
		return
	}

	opts := []task.TaskOption{}
	if req.SessionID != "" {
		opts = append(opts, task.WithSessionID(req.SessionID))
	}
	if req.AgentID != "" {
		opts = append(opts, task.WithAgentID(req.AgentID))
	}
	if len(req.Media) > 0 {
		opts = append(opts, task.WithMedia(req.Media))
	}
	if req.Stream {
		opts = append(opts, task.WithStream(req.Stream))
	}
	if req.CallbackURL != "" {
		opts = append(opts, task.WithCallbackURL(req.CallbackURL))
	}

	t, err := h.manager.Create(req.Message, opts...)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Error: ErrorDetail{
				Code:    "internal_error",
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(201, t)
}

// GetTask 获取任务状态
func (h *TaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("task_id")

	t := h.manager.Get(taskID)
	if t == nil {
		c.JSON(404, ErrorResponse{
			Error: ErrorDetail{
				Code:    "task_not_found",
				Message: "Task not found: " + taskID,
			},
		})
		return
	}

	c.JSON(200, t)
}

// DeleteTask 删除任务
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	taskID := c.Param("task_id")

	if !h.manager.Delete(taskID) {
		c.JSON(404, ErrorResponse{
			Error: ErrorDetail{
				Code:    "task_not_found",
				Message: "Task not found: " + taskID,
			},
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "task deleted",
		"id":      taskID,
	})
}

// GetTaskEvents SSE 事件流
func (h *TaskHandler) GetTaskEvents(c *gin.Context) {
	taskID := c.Param("task_id")

	t := h.manager.Get(taskID)
	if t == nil {
		c.JSON(404, ErrorResponse{
			Error: ErrorDetail{
				Code:    "task_not_found",
				Message: "Task not found: " + taskID,
			},
		})
		return
	}

	// 设置 SSE headers
	sse.SetupHeaders(c)
	writer := sse.NewWriter(c.Writer)

	// 发送连接事件
	writer.WriteEvent("connected", gin.H{"task_id": taskID})

	// 监听事件
	for event := range t.Events() {
		if err := writer.WriteEvent(event.Type, event.Data); err != nil {
			logger.Warn("Failed to write SSE event", "taskId", taskID, "error", err)
			return
		}

		// 如果是终止事件，结束流
		if event.Type == task.EventCompleted ||
			event.Type == task.EventFailed ||
			event.Type == task.EventCancelled {
			return
		}
	}
}

// ListTasks 列出任务
func (h *TaskHandler) ListTasks(c *gin.Context) {
	status := task.TaskStatus(c.Query("status"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	tasks := h.manager.List(status, limit, offset)
	total := h.manager.Count(status)

	c.JSON(200, gin.H{
		"tasks":  tasks,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
