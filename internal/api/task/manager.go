// Package task 异步任务管理
package task

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/internal/agent"
	"github.com/lingguard/pkg/logger"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Task 异步任务
type Task struct {
	ID          string     `json:"id"`
	Status      TaskStatus `json:"status"`
	Progress    int        `json:"progress"`
	ProgressMsg string     `json:"progress_message,omitempty"`
	Message     string     `json:"message"`
	Media       []string   `json:"media,omitempty"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	AgentID     string     `json:"agent_id"`
	SessionID   string     `json:"session_id,omitempty"`
	Stream      bool       `json:"stream"`
	CallbackURL string     `json:"callback_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// 内部字段
	events chan Event
	cancel context.CancelFunc
	mu     sync.RWMutex
}

// Event 任务事件
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// EventType 事件类型
const (
	EventStarted   = "started"
	EventProgress  = "progress"
	EventContent   = "content"
	EventCompleted = "completed"
	EventFailed    = "failed"
	EventCancelled = "cancelled"
)

// TaskOption 任务选项
type TaskOption func(*Task)

// WithSessionID 设置会话 ID
func WithSessionID(sessionID string) TaskOption {
	return func(t *Task) {
		t.SessionID = sessionID
	}
}

// WithAgentID 设置智能体 ID
func WithAgentID(agentID string) TaskOption {
	return func(t *Task) {
		t.AgentID = agentID
	}
}

// WithMedia 设置媒体
func WithMedia(media []string) TaskOption {
	return func(t *Task) {
		t.Media = media
	}
}

// WithStream 设置流式
func WithStream(stream bool) TaskOption {
	return func(t *Task) {
		t.Stream = stream
	}
}

// WithCallbackURL 设置回调 URL
func WithCallbackURL(url string) TaskOption {
	return func(t *Task) {
		t.CallbackURL = url
	}
}

// Manager 任务管理器
type Manager struct {
	tasks map[string]*Task
	mu    sync.RWMutex
	agent *agent.Agent
}

// NewManager 创建任务管理器
func NewManager(ag *agent.Agent) *Manager {
	return &Manager{
		tasks: make(map[string]*Task),
		agent: ag,
	}
}

// Create 创建新任务
func (m *Manager) Create(message string, opts ...TaskOption) (*Task, error) {
	task := &Task{
		ID:        "task-" + uuid.New().String()[:8],
		Status:    TaskStatusPending,
		Message:   message,
		AgentID:   "default",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		events:    make(chan Event, 100),
	}

	// 应用选项
	for _, opt := range opts {
		opt(task)
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 启动异步执行
	ctx, cancel := context.WithCancel(context.Background())
	task.cancel = cancel
	go m.execute(ctx, task)

	logger.Info("Task created", "taskId", task.ID, "agentId", task.AgentID)
	return task, nil
}

// execute 执行任务
func (m *Manager) execute(ctx context.Context, task *Task) {
	task.mu.Lock()
	task.Status = TaskStatusRunning
	task.UpdatedAt = time.Now()
	task.mu.Unlock()

	task.emit(Event{Type: EventStarted, Data: map[string]interface{}{
		"task_id": task.ID,
	}})

	var result string
	var err error

	// 根据是否有媒体选择不同的调用方式
	if len(task.Media) > 0 {
		result, err = m.agent.ProcessMessageWithMedia(ctx, task.SessionID, task.Message, task.Media)
	} else {
		result, err = m.agent.ProcessMessage(ctx, task.SessionID, task.Message)
	}

	now := time.Now()
	task.mu.Lock()
	task.UpdatedAt = now
	task.CompletedAt = &now

	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
		task.mu.Unlock()

		task.emit(Event{Type: EventFailed, Data: map[string]interface{}{
			"error": err.Error(),
		}})
		logger.Error("Task failed", "taskId", task.ID, "error", err)
	} else {
		task.Status = TaskStatusCompleted
		task.Result = result
		task.Progress = 100
		task.mu.Unlock()

		task.emit(Event{Type: EventCompleted, Data: map[string]interface{}{
			"result":   result,
			"task_id":  task.ID,
			"duration": now.Sub(task.CreatedAt).Milliseconds(),
		}})
		logger.Info("Task completed", "taskId", task.ID, "duration", now.Sub(task.CreatedAt).Milliseconds())
	}

	// 关闭事件通道
	close(task.events)
}

// Get 获取任务
func (m *Manager) Get(taskID string) *Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[taskID]
}

// Cancel 取消任务
func (m *Manager) Cancel(taskID string) bool {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	if task.Status != TaskStatusPending && task.Status != TaskStatusRunning {
		return false
	}

	if task.cancel != nil {
		task.cancel()
	}

	task.Status = TaskStatusCancelled
	now := time.Now()
	task.UpdatedAt = now
	task.CompletedAt = &now

	task.emit(Event{Type: EventCancelled, Data: map[string]interface{}{
		"task_id": task.ID,
	}})

	logger.Info("Task cancelled", "taskId", task.ID)
	return true
}

// Delete 删除任务
func (m *Manager) Delete(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return false
	}

	// 如果任务正在运行，先取消
	if task.Status == TaskStatusRunning && task.cancel != nil {
		task.cancel()
	}

	delete(m.tasks, taskID)
	logger.Info("Task deleted", "taskId", taskID)
	return true
}

// List 列出任务
func (m *Manager) List(status TaskStatus, limit, offset int) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Task
	for _, task := range m.tasks {
		if status == "" || task.Status == status {
			result = append(result, task)
		}
	}

	// 按创建时间倒序
	// sort.Slice(result, func(i, j int) bool {
	// 	return result[i].CreatedAt.After(result[j].CreatedAt)
	// })

	// 分页
	if offset >= len(result) {
		return []*Task{}
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end]
}

// Count 统计任务数量
func (m *Manager) Count(status TaskStatus) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if status == "" {
		return len(m.tasks)
	}

	count := 0
	for _, task := range m.tasks {
		if task.Status == status {
			count++
		}
	}
	return count
}

// emit 发送事件
func (t *Task) emit(event Event) {
	select {
	case t.events <- event:
	default:
		// 通道满了，丢弃事件
		logger.Warn("Task event channel full, dropping event", "taskId", t.ID, "type", event.Type)
	}
}

// Events 获取事件通道
func (t *Task) Events() <-chan Event {
	return t.events
}
