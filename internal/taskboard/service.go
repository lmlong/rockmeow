package taskboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/lingguard/pkg/logger"
)

// Service 任务看板服务
type Service struct {
	store Store
}

// NewService 创建任务看板服务
func NewService(store Store) *Service {
	return &Service{
		store: store,
	}
}

// CreateTask 创建任务
func (s *Service) CreateTask(task *Task) error {
	if task.Title == "" {
		return fmt.Errorf("task title is required")
	}
	return s.store.Create(task)
}

// CreateTaskFromUserRequest 从用户请求创建任务
func (s *Service) CreateTaskFromUserRequest(sessionID, message string) (*Task, error) {
	task := &Task{
		Title:        truncateTitle(message, 50),
		Status:       TaskStatusPending,
		Column:       ColumnTodo,
		Source:       TaskSourceUser,
		SessionID:    sessionID,
		Assignee:     "main-agent",
		AssigneeType: AssigneeTypeAgent,
	}

	if err := s.store.Create(task); err != nil {
		return nil, err
	}

	logger.Info("Task created from user request", "taskId", task.ID, "session", sessionID, "title", task.Title)
	return task, nil
}

// GetTask 获取任务
func (s *Service) GetTask(id string) (*Task, error) {
	return s.store.Get(id)
}

// UpdateTask 更新任务
func (s *Service) UpdateTask(task *Task) error {
	return s.store.Update(task)
}

// DeleteTask 删除任务
func (s *Service) DeleteTask(id string) error {
	return s.store.Delete(id)
}

// ListTasks 列出任务
func (s *Service) ListTasks(filter *TaskFilter) ([]*Task, error) {
	return s.store.List(filter)
}

// UpdateStatus 更新任务状态
func (s *Service) UpdateStatus(id string, status TaskStatus) error {
	logger.Info("Task status updated", "taskId", id, "status", status)
	return s.store.UpdateStatus(id, status)
}

// MoveToColumn 移动任务到指定列
func (s *Service) MoveToColumn(id string, column Column) error {
	logger.Info("Task moved to column", "taskId", id, "column", column)
	return s.store.MoveToColumn(id, column)
}

// StartTask 开始任务（状态变为 running，移动到 in_progress）
func (s *Service) StartTask(id string) error {
	if err := s.UpdateStatus(id, TaskStatusRunning); err != nil {
		return err
	}
	return s.MoveToColumn(id, ColumnInProgress)
}

// CompleteTask 完成任务
func (s *Service) CompleteTask(id string, result string) error {
	if err := s.UpdateStatus(id, TaskStatusCompleted); err != nil {
		return err
	}
	if err := s.MoveToColumn(id, ColumnDone); err != nil {
		return err
	}
	if result != "" {
		if err := s.store.SetResult(id, result); err != nil {
			logger.Warn("Failed to set task result", "taskId", id, "error", err)
		}
	}
	logger.Info("Task completed", "taskId", id)
	return nil
}

// FailTask 标记任务失败
func (s *Service) FailTask(id string, errMsg string) error {
	if err := s.UpdateStatus(id, TaskStatusFailed); err != nil {
		return err
	}
	if err := s.MoveToColumn(id, ColumnDone); err != nil {
		return err
	}
	if errMsg != "" {
		if err := s.store.SetError(id, errMsg); err != nil {
			logger.Warn("Failed to set task error", "taskId", id, "error", err)
		}
	}
	logger.Info("Task failed", "taskId", id, "error", errMsg)
	return nil
}

// CancelTask 取消任务
func (s *Service) CancelTask(id string) error {
	if err := s.UpdateStatus(id, TaskStatusCancelled); err != nil {
		return err
	}
	return s.MoveToColumn(id, ColumnDone)
}

// AssignTask 分配任务
func (s *Service) AssignTask(id string, assignee string, assigneeType AssigneeType) error {
	return s.store.SetAssignee(id, assignee, assigneeType)
}

// GetStats 获取统计信息
func (s *Service) GetStats() (*Stats, error) {
	return s.store.GetStats()
}

// GetBoard 获取看板视图
func (s *Service) GetBoard() (*Board, error) {
	return s.store.GetBoard()
}

// Subscribe 订阅事件
func (s *Service) Subscribe() <-chan TaskEvent {
	return s.store.Subscribe()
}

// CreateSubagentTask 创建子代理任务
func (s *Service) CreateSubagentTask(subagentID, task, context string, parentTaskID string) (*Task, error) {
	t := &Task{
		Title:        truncateTitle(task, 50),
		Description:  context,
		Status:       TaskStatusRunning,
		Column:       ColumnInProgress,
		Source:       TaskSourceSubagent,
		SourceRef:    subagentID,
		Assignee:     "subagent",
		AssigneeType: AssigneeTypeAgent,
		Metadata: map[string]interface{}{
			"parentTaskId": parentTaskID,
		},
	}

	if err := s.store.Create(t); err != nil {
		return nil, err
	}

	logger.Info("Subagent task created", "taskId", t.ID, "subagentId", subagentID)
	return t, nil
}

// CreateCronTask 创建定时任务对应的看板任务
func (s *Service) CreateCronTask(cronID, name, message string) (*Task, error) {
	t := &Task{
		Title:        fmt.Sprintf("[Cron] %s", name),
		Description:  message,
		Status:       TaskStatusPending,
		Column:       ColumnTodo,
		Source:       TaskSourceCron,
		SourceRef:    cronID,
		Assignee:     "cron-service",
		AssigneeType: AssigneeTypeAgent,
	}

	if err := s.store.Create(t); err != nil {
		return nil, err
	}

	logger.Info("Cron task created", "taskId", t.ID, "cronId", cronID)
	return t, nil
}

// UpdateSubagentStatus 更新子代理任务状态
func (s *Service) UpdateSubagentStatus(subagentID string, status TaskStatus, result string, errMsg string) error {
	// 查找子代理任务
	tasks, err := s.store.List(&TaskFilter{
		Source: ptrSource(TaskSourceSubagent),
		Limit:  100,
	})
	if err != nil {
		return err
	}

	for _, t := range tasks {
		if t.SourceRef == subagentID {
			if status == TaskStatusCompleted {
				return s.CompleteTask(t.ID, result)
			} else if status == TaskStatusFailed {
				return s.FailTask(t.ID, errMsg)
			} else {
				return s.UpdateStatus(t.ID, status)
			}
		}
	}

	return fmt.Errorf("subagent task not found: %s", subagentID)
}

// CleanupOldTasks 清理旧任务（保留最近 N 天）
func (s *Service) CleanupOldTasks(days int) (int, error) {
	tasks, err := s.store.List(nil)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	count := 0

	for _, t := range tasks {
		// 只删除已完成的旧任务
		if t.Status == TaskStatusCompleted || t.Status == TaskStatusFailed || t.Status == TaskStatusCancelled {
			if t.CompletedAt != nil && t.CompletedAt.Before(cutoff) {
				if err := s.store.Delete(t.ID); err != nil {
					logger.Warn("Failed to delete old task", "taskId", t.ID, "error", err)
				} else {
					count++
				}
			}
		}
	}

	if count > 0 {
		logger.Info("Cleaned up old tasks", "count", count, "days", days)
	}

	return count, nil
}

// Helper functions

// truncateTitle 截断标题
func truncateTitle(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ptrSource 返回 TaskSource 指针
func ptrSource(s TaskSource) *TaskSource {
	return &s
}

// ptrStatus 返回 TaskStatus 指针
func ptrStatus(s TaskStatus) *TaskStatus {
	return &s
}

// ptrColumn 返回 Column 指针
func ptrColumn(c Column) *Column {
	return &c
}
