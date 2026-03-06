// Package handlers API 处理器
package handlers

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/api/sse"
	"github.com/lingguard/internal/taskboard"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/validation"
)

// TaskboardHandler 任务看板处理器（Gin 版本）
type TaskboardHandler struct {
	service     *taskboard.Service
	cronDeleter taskboard.CronDeleter
}

// NewTaskboardHandler 创建任务看板处理器
func NewTaskboardHandler(service *taskboard.Service) *TaskboardHandler {
	return &TaskboardHandler{service: service}
}

// SetCronDeleter 设置 cron 删除器
func (h *TaskboardHandler) SetCronDeleter(deleter taskboard.CronDeleter) {
	h.cronDeleter = deleter
}

// RegisterRoutes 注册路由
func (h *TaskboardHandler) RegisterRoutes(r *gin.RouterGroup) {
	tasks := r.Group("/api")
	{
		tasks.GET("/tasks", h.ListTasks)
		tasks.GET("/tasks/:id", h.GetTask)
		tasks.POST("/tasks", h.CreateTask)
		tasks.PUT("/tasks/:id", h.UpdateTask)
		tasks.DELETE("/tasks/:id", h.DeleteTask)
		tasks.PUT("/tasks/:id/status", h.UpdateStatus)
		tasks.PUT("/tasks/:id/column", h.MoveColumn)
		tasks.POST("/tasks/:id/assign", h.Assign)
		tasks.GET("/board", h.GetBoard)
		tasks.GET("/stats", h.GetStats)
		tasks.GET("/events", h.SSE)
	}
}

// ListTasks 列出任务
func (h *TaskboardHandler) ListTasks(c *gin.Context) {
	filter := &taskboard.TaskFilter{}

	if status := c.Query("status"); status != "" {
		s := taskboard.TaskStatus(status)
		filter.Status = &s
	}
	if source := c.Query("source"); source != "" {
		s := taskboard.TaskSource(source)
		filter.Source = &s
	}
	if column := c.Query("column"); column != "" {
		col := taskboard.Column(column)
		filter.Column = &col
	}
	if assignee := c.Query("assignee"); assignee != "" {
		filter.Assignee = assignee
	}

	tasks, err := h.service.ListTasks(filter)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, tasks)
}

// GetTask 获取单个任务
func (h *TaskboardHandler) GetTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "task id is required"})
		return
	}

	task, err := h.service.GetTask(id)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, task)
}

// CreateTask 创建任务
func (h *TaskboardHandler) CreateTask(c *gin.Context) {
	var task taskboard.Task
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	// 验证任务
	v := validation.New()
	if !v.Validate(&task) {
		c.JSON(400, gin.H{
			"error":   "validation failed",
			"details": v.Errors,
		})
		return
	}

	task.Source = taskboard.TaskSourceManual
	if err := h.service.CreateTask(&task); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, task)
}

// UpdateTask 更新任务
func (h *TaskboardHandler) UpdateTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "task id is required"})
		return
	}

	task, err := h.service.GetTask(id)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	var updates taskboard.Task
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	// 只更新允许的字段
	if updates.Title != "" {
		task.Title = updates.Title
	}
	if updates.Description != "" {
		task.Description = updates.Description
	}
	if updates.Priority != "" {
		task.Priority = updates.Priority
	}
	if updates.DueDate != nil {
		task.DueDate = updates.DueDate
	}

	if err := h.service.UpdateTask(task); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, task)
}

// DeleteTask 删除任务
func (h *TaskboardHandler) DeleteTask(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "task id is required"})
		return
	}

	// 先获取任务信息，检查是否是 cron 任务
	task, err := h.service.GetTask(id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// 先删除看板任务
	if err := h.service.DeleteTask(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// 如果是 cron 任务，同时删除 cron job
	if task.Source == taskboard.TaskSourceCron && task.SourceRef != "" && h.cronDeleter != nil {
		logger.Info("Deleting cron job along with taskboard task", "taskId", id, "cronId", task.SourceRef)
		h.cronDeleter.RemoveJob(task.SourceRef)
	}

	c.JSON(200, gin.H{"message": "task deleted"})
}

// UpdateStatus 更新状态
func (h *TaskboardHandler) UpdateStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "task id is required"})
		return
	}

	var req struct {
		Status string `json:"status"`
		Result string `json:"result"`
		Error  string `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	var err error
	switch taskboard.TaskStatus(req.Status) {
	case taskboard.TaskStatusRunning:
		err = h.service.StartTask(id)
	case taskboard.TaskStatusCompleted:
		err = h.service.CompleteTask(id, req.Result)
	case taskboard.TaskStatusFailed:
		err = h.service.FailTask(id, req.Error)
	case taskboard.TaskStatusCancelled:
		err = h.service.CancelTask(id)
	default:
		err = h.service.UpdateStatus(id, taskboard.TaskStatus(req.Status))
	}

	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	task, _ := h.service.GetTask(id)
	c.JSON(200, task)
}

// MoveColumn 移动列
func (h *TaskboardHandler) MoveColumn(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "task id is required"})
		return
	}

	var req struct {
		Column string `json:"column"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.MoveToColumn(id, taskboard.Column(req.Column)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	task, _ := h.service.GetTask(id)
	c.JSON(200, task)
}

// Assign 分配任务
func (h *TaskboardHandler) Assign(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(400, gin.H{"error": "task id is required"})
		return
	}

	var req struct {
		Assignee     string                 `json:"assignee"`
		AssigneeType taskboard.AssigneeType `json:"assigneeType"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}

	if req.AssigneeType == "" {
		req.AssigneeType = taskboard.AssigneeTypeAgent
	}

	if err := h.service.AssignTask(id, req.Assignee, req.AssigneeType); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	task, _ := h.service.GetTask(id)
	c.JSON(200, task)
}

// GetBoard 获取看板
func (h *TaskboardHandler) GetBoard(c *gin.Context) {
	board, err := h.service.GetBoard()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, board)
}

// GetStats 获取统计
func (h *TaskboardHandler) GetStats(c *gin.Context) {
	stats, err := h.service.GetStats()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, stats)
}

// SSE SSE 事件流
func (h *TaskboardHandler) SSE(c *gin.Context) {
	sse.SetupHeaders(c)

	writer := sse.NewWriter(c.Writer)

	// 订阅事件
	eventCh := h.service.Subscribe()

	// 发送初始连接消息
	writer.WriteEvent("connected", gin.H{"message": "connected"})

	// 心跳定时器
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case event := <-eventCh:
			writer.WriteEvent("task", event)
		case <-ticker.C:
			writer.WriteEvent("ping", gin.H{"time": time.Now().Unix()})
		}
	}
}
