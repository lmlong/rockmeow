// Package taskboard 任务看板系统
package taskboard

import (
	"time"
)

// Task 看板任务
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`

	// 状态管理
	Status   TaskStatus `json:"status"`
	Priority Priority   `json:"priority"`
	Column   Column     `json:"column"`

	// 分配
	Assignee     string       `json:"assignee,omitempty"`
	AssigneeType AssigneeType `json:"assigneeType,omitempty"`

	// 时间
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	DueDate     *time.Time `json:"dueDate,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// 关联
	SessionID string     `json:"sessionId,omitempty"` // 会话 ID
	Source    TaskSource `json:"source"`              // 来源
	SourceRef string     `json:"sourceRef,omitempty"` // 源系统 ID

	// 结果
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`

	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// Column 看板列
type Column string

const (
	ColumnBacklog    Column = "backlog"
	ColumnTodo       Column = "todo"
	ColumnInProgress Column = "in_progress"
	ColumnDone       Column = "done"
)

// Priority 优先级
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// TaskSource 任务来源
type TaskSource string

const (
	TaskSourceUser     TaskSource = "user"     // 用户请求（核心）
	TaskSourceManual   TaskSource = "manual"   // 手动创建
	TaskSourceSubagent TaskSource = "subagent" // 子代理
	TaskSourceCron     TaskSource = "cron"     // 定时任务
)

// AssigneeType 分配对象类型
type AssigneeType string

const (
	AssigneeTypeAgent AssigneeType = "agent"
	AssigneeTypeUser  AssigneeType = "user"
)

// TaskFilter 任务过滤条件
type TaskFilter struct {
	Status   *TaskStatus `json:"status,omitempty"`
	Source   *TaskSource `json:"source,omitempty"`
	Column   *Column     `json:"column,omitempty"`
	Priority *Priority   `json:"priority,omitempty"`
	Assignee string      `json:"assignee,omitempty"`
	Limit    int         `json:"limit,omitempty"`
	Offset   int         `json:"offset,omitempty"`
}

// Board 看板视图
type Board struct {
	Backlog    []*Task `json:"backlog"`
	Todo       []*Task `json:"todo"`
	InProgress []*Task `json:"inProgress"`
	Done       []*Task `json:"done"`
}

// Stats 统计信息
type Stats struct {
	Total      int            `json:"total"`
	ByStatus   map[string]int `json:"byStatus"`
	BySource   map[string]int `json:"bySource"`
	ByColumn   map[string]int `json:"byColumn"`
	ByPriority map[string]int `json:"byPriority"`
}

// TaskEvent 任务事件（用于 SSE）
type TaskEvent struct {
	Type      string    `json:"type"` // create, update, delete, move
	TaskID    string    `json:"taskId"`
	Task      *Task     `json:"task,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// EventType 事件类型
const (
	EventTypeCreate = "create"
	EventTypeUpdate = "update"
	EventTypeDelete = "delete"
	EventTypeMove   = "move"
)
