package taskboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lingguard/pkg/logger"
)

// HTTPHandler 任务看板 HTTP 处理器
type HTTPHandler struct {
	service *Service
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler(service *Service) *HTTPHandler {
	return &HTTPHandler{service: service}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	// API 路由
	mux.HandleFunc("GET /api/tasks", h.handleListTasks)
	mux.HandleFunc("GET /api/tasks/{id}", h.handleGetTask)
	mux.HandleFunc("POST /api/tasks", h.handleCreateTask)
	mux.HandleFunc("PUT /api/tasks/{id}", h.handleUpdateTask)
	mux.HandleFunc("DELETE /api/tasks/{id}", h.handleDeleteTask)
	mux.HandleFunc("PUT /api/tasks/{id}/status", h.handleUpdateStatus)
	mux.HandleFunc("PUT /api/tasks/{id}/column", h.handleMoveColumn)
	mux.HandleFunc("POST /api/tasks/{id}/assign", h.handleAssign)
	mux.HandleFunc("GET /api/board", h.handleGetBoard)
	mux.HandleFunc("GET /api/stats", h.handleGetStats)
	mux.HandleFunc("GET /api/events", h.handleSSE)
}

// handleListTasks 列出任务
func (h *HTTPHandler) handleListTasks(w http.ResponseWriter, r *http.Request) {
	filter := &TaskFilter{}

	// 解析查询参数
	if status := r.URL.Query().Get("status"); status != "" {
		s := TaskStatus(status)
		filter.Status = &s
	}
	if source := r.URL.Query().Get("source"); source != "" {
		s := TaskSource(source)
		filter.Source = &s
	}
	if column := r.URL.Query().Get("column"); column != "" {
		c := Column(column)
		filter.Column = &c
	}
	if assignee := r.URL.Query().Get("assignee"); assignee != "" {
		filter.Assignee = assignee
	}

	tasks, err := h.service.ListTasks(filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, tasks)
}

// handleGetTask 获取单个任务
func (h *HTTPHandler) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	task, err := h.service.GetTask(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, task)
}

// handleCreateTask 创建任务
func (h *HTTPHandler) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	task.Source = TaskSourceManual
	if err := h.service.CreateTask(&task); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, task)
}

// handleUpdateTask 更新任务
func (h *HTTPHandler) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	task, err := h.service.GetTask(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var updates Task
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
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
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, task)
}

// handleDeleteTask 删除任务
func (h *HTTPHandler) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	if err := h.service.DeleteTask(id); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, map[string]string{"message": "task deleted"})
}

// handleUpdateStatus 更新状态
func (h *HTTPHandler) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var req struct {
		Status string `json:"status"`
		Result string `json:"result"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var err error
	switch TaskStatus(req.Status) {
	case TaskStatusRunning:
		err = h.service.StartTask(id)
	case TaskStatusCompleted:
		err = h.service.CompleteTask(id, req.Result)
	case TaskStatusFailed:
		err = h.service.FailTask(id, req.Error)
	case TaskStatusCancelled:
		err = h.service.CancelTask(id)
	default:
		err = h.service.UpdateStatus(id, TaskStatus(req.Status))
	}

	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	task, _ := h.service.GetTask(id)
	h.writeJSON(w, task)
}

// handleMoveColumn 移动列
func (h *HTTPHandler) handleMoveColumn(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var req struct {
		Column string `json:"column"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.service.MoveToColumn(id, Column(req.Column)); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	task, _ := h.service.GetTask(id)
	h.writeJSON(w, task)
}

// handleAssign 分配任务
func (h *HTTPHandler) handleAssign(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var req struct {
		Assignee     string       `json:"assignee"`
		AssigneeType AssigneeType `json:"assigneeType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.AssigneeType == "" {
		req.AssigneeType = AssigneeTypeAgent
	}

	if err := h.service.AssignTask(id, req.Assignee, req.AssigneeType); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	task, _ := h.service.GetTask(id)
	h.writeJSON(w, task)
}

// handleGetBoard 获取看板
func (h *HTTPHandler) handleGetBoard(w http.ResponseWriter, r *http.Request) {
	board, err := h.service.GetBoard()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, board)
}

// handleGetStats 获取统计
func (h *HTTPHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetStats()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, stats)
}

// handleSSE SSE 事件流
func (h *HTTPHandler) handleSSE(w http.ResponseWriter, r *http.Request) {
	// 设置 SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// 订阅事件
	eventCh := h.service.Subscribe()

	// 发送初始连接消息
	fmt.Fprintf(w, "event: connected\ndata: {\"message\":\"connected\"}\n\n")
	flusher.Flush()

	// 心跳定时器
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-eventCh:
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: task\ndata: %s\n\n", data)
			flusher.Flush()
		case <-ticker.C:
			// 发送心跳
			fmt.Fprintf(w, "event: ping\ndata: {\"time\":%d}\n\n", time.Now().Unix())
			flusher.Flush()
		}
	}
}

// Helper methods

func (h *HTTPHandler) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Warn("Failed to encode response", "error", err)
	}
}

func (h *HTTPHandler) writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
