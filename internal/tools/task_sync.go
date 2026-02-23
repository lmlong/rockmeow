package tools

import (
	"context"
	"sync"
	"time"
)

// TaskSource 任务来源
type TaskSource string

const (
	TaskSourceCron      TaskSource = "cron"      // 定时任务
	TaskSourceSubagent  TaskSource = "subagent"  // 子代理
	TaskSourceHeartbeat TaskSource = "heartbeat" // 心跳任务
	TaskSourceAgent     TaskSource = "agent"     // Agent 会话
)

// TaskEvent 任务事件类型
type TaskEvent string

const (
	TaskEventCreated   TaskEvent = "created"
	TaskEventStarted   TaskEvent = "started"
	TaskEventCompleted TaskEvent = "completed"
	TaskEventFailed    TaskEvent = "failed"
)

// TaskSyncEvent 任务同步事件
type TaskSyncEvent struct {
	Source      TaskSource    // 来源
	Event       TaskEvent     // 事件类型
	ExternalID  string        // 外部唯一标识
	Title       string        // 任务标题
	Description string        // 任务描述
	Status      TaskStatus    // 任务状态
	Assignee    TaskAssignee  // 分配者
	SessionID   string        // 会话 ID
	SubagentID  string        // 子代理 ID
	Priority    TaskPriority  // 优先级
	Tags        []string      // 标签
	Result      string        // 结果
	Error       string        // 错误信息
	Metadata    *TaskMetadata // 元数据
}

// TaskSyncer 任务同步器接口
type TaskSyncer interface {
	// Sync 同步任务事件到看板
	Sync(ctx context.Context, event *TaskSyncEvent) error
}

// TasksBoardSyncer 任务看板同步器实现
type TasksBoardSyncer struct {
	tool    *TasksBoardTool
	enabled bool
	mu      sync.RWMutex
}

// NewTasksBoardSyncer 创建任务看板同步器
func NewTasksBoardSyncer(tool *TasksBoardTool) *TasksBoardSyncer {
	return &TasksBoardSyncer{
		tool:    tool,
		enabled: tool != nil && tool.config != nil && tool.config.URL != "",
	}
}

// Sync 同步任务事件到看板
func (s *TasksBoardSyncer) Sync(ctx context.Context, event *TaskSyncEvent) error {
	s.mu.RLock()
	enabled := s.enabled
	s.mu.RUnlock()

	if !enabled {
		return nil
	}

	task := &Task{
		ExternalID:  event.ExternalID,
		Title:       event.Title,
		Description: event.Description,
		Status:      event.Status,
		Assignee:    event.Assignee,
		SessionID:   event.SessionID,
		SubagentID:  event.SubagentID,
		Priority:    event.Priority,
		Tags:        event.Tags,
		Result:      event.Result,
		Error:       event.Error,
		Metadata:    event.Metadata,
	}

	// 根据事件类型设置时间戳
	now := time.Now().UnixMilli()
	switch event.Event {
	case TaskEventStarted:
		task.StartedAt = &now
	case TaskEventCompleted, TaskEventFailed:
		task.CompletedAt = &now
		task.StartedAt = &now // 确保有开始时间
	}

	// 使用 sync 操作（通过 externalId 更新或创建）
	_, err := s.tool.syncTasks(ctx, []*Task{task})
	return err
}

// SetEnabled 设置是否启用
func (s *TasksBoardSyncer) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled && s.tool != nil && s.tool.config != nil && s.tool.config.URL != ""
}

// IsEnabled 返回是否启用
func (s *TasksBoardSyncer) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// NoopTaskSyncer 空同步器（禁用同步时使用）
type NoopTaskSyncer struct{}

func NewNoopTaskSyncer() *NoopTaskSyncer {
	return &NoopTaskSyncer{}
}

func (s *NoopTaskSyncer) Sync(ctx context.Context, event *TaskSyncEvent) error {
	return nil
}

func (s *NoopTaskSyncer) IsEnabled() bool {
	return false
}

// GlobalTaskSyncer 全局任务同步器实例
var globalTaskSyncer TaskSyncer = &NoopTaskSyncer{}

// SetGlobalTaskSyncer 设置全局任务同步器
func SetGlobalTaskSyncer(syncer TaskSyncer) {
	globalTaskSyncer = syncer
}

// GetGlobalTaskSyncer 获取全局任务同步器
func GetGlobalTaskSyncer() TaskSyncer {
	return globalTaskSyncer
}

// SyncTask 全局同步任务函数（便捷方法）
func SyncTask(ctx context.Context, event *TaskSyncEvent) error {
	return globalTaskSyncer.Sync(ctx, event)
}
