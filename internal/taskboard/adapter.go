package taskboard

import (
	"fmt"
	"time"

	"github.com/lingguard/internal/cron"
	"github.com/lingguard/internal/subagent"
	"github.com/lingguard/pkg/logger"
)

// SubagentAdapter 子代理适配器
type SubagentAdapter struct {
	service *Service
}

// NewSubagentAdapter 创建子代理适配器
func NewSubagentAdapter(service *Service) *SubagentAdapter {
	return &SubagentAdapter{service: service}
}

// OnSubagentCreated 子代理创建时调用
func (a *SubagentAdapter) OnSubagentCreated(sub *subagent.Subagent, parentTaskID string) {
	if a.service == nil {
		return
	}

	task, err := a.service.CreateSubagentTask(sub.ID(), sub.Task(), sub.Context(), parentTaskID)
	if err != nil {
		logger.Warn("Failed to create subagent task", "subagentId", sub.ID(), "error", err)
		return
	}

	logger.Info("Subagent task created", "taskId", task.ID, "subagentId", sub.ID())
}

// OnSubagentStatusChanged 子代理状态变化时调用
func (a *SubagentAdapter) OnSubagentStatusChanged(sub *subagent.Subagent) {
	if a.service == nil {
		return
	}

	var status TaskStatus
	switch sub.Status() {
	case subagent.StatusRunning:
		status = TaskStatusRunning
	case subagent.StatusCompleted:
		status = TaskStatusCompleted
	case subagent.StatusFailed:
		status = TaskStatusFailed
	default:
		return
	}

	if err := a.service.UpdateSubagentStatus(sub.ID(), status, sub.Result(), sub.Error()); err != nil {
		logger.Warn("Failed to update subagent task status", "subagentId", sub.ID(), "error", err)
	}
}

// CronAdapter 定时任务适配器
type CronAdapter struct {
	service *Service
}

// NewCronAdapter 创建定时任务适配器
func NewCronAdapter(service *Service) *CronAdapter {
	return &CronAdapter{service: service}
}

// OnCronJobCreated 定时任务创建时调用
// 为所有定时任务（单次和周期性）创建看板任务
func (a *CronAdapter) OnCronJobCreated(job *cron.CronJob) {
	if a.service == nil {
		return
	}

	// 检查是否已存在该 cron 任务的看板任务
	tasks, err := a.service.ListTasks(&TaskFilter{
		Source: ptrSource(TaskSourceCron),
		Limit:  100,
	})
	if err != nil {
		logger.Warn("Failed to list cron tasks", "error", err)
	} else {
		for _, t := range tasks {
			if t.SourceRef == job.ID {
				// 已存在，更新 metadata
				a.updateCronTaskMetadata(t, job)
				return
			}
		}
	}

	// 创建新的看板任务
	scheduleType := "周期性"
	scheduleExpr := ""
	if job.Schedule.Kind == cron.ScheduleKindAt {
		scheduleType = "单次"
		scheduleExpr = time.UnixMilli(job.Schedule.AtMs).Format("2006-01-02 15:04:05")
	} else if job.Schedule.Kind == cron.ScheduleKindCron {
		scheduleExpr = job.Schedule.Expr
		if job.Schedule.TZ != "" {
			scheduleExpr += " (TZ: " + job.Schedule.TZ + ")"
		}
	} else if job.Schedule.Kind == cron.ScheduleKindEvery {
		scheduleExpr = fmt.Sprintf("每 %s", time.Duration(job.Schedule.EveryMs)*time.Millisecond)
	}

	task := &Task{
		Title:        job.Name,
		Description:  job.Payload.Message,
		Status:       TaskStatusRunning, // 所有定时任务默认为进行中
		Column:       ColumnInProgress,  // 所有定时任务默认在进行中列
		Source:       TaskSourceCron,
		SourceRef:    job.ID,
		Assignee:     "cron-service",
		AssigneeType: AssigneeTypeAgent,
		Metadata: map[string]interface{}{
			"scheduleType":   scheduleType,
			"scheduleKind":   string(job.Schedule.Kind),
			"scheduleExpr":   scheduleExpr,
			"enabled":        job.Enabled,
			"nextRunAtMs":    job.State.NextRunAtMs,
			"lastRunAtMs":    job.State.LastRunAtMs,
			"lastStatus":     job.State.LastStatus,
			"executingAt":    "",
			"executionCount": 0,
		},
	}

	if err := a.service.CreateTask(task); err != nil {
		logger.Warn("Failed to create cron task", "cronId", job.ID, "error", err)
		return
	}

	logger.Info("Cron task created", "taskId", task.ID, "cronId", job.ID, "name", job.Name, "scheduleType", scheduleType)
}

// updateCronTaskMetadata 更新定时任务元数据
func (a *CronAdapter) updateCronTaskMetadata(task *Task, job *cron.CronJob) {
	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}

	task.Metadata["enabled"] = job.Enabled
	task.Metadata["nextRunAtMs"] = job.State.NextRunAtMs
	task.Metadata["lastRunAtMs"] = job.State.LastRunAtMs
	task.Metadata["lastStatus"] = job.State.LastStatus

	// 更新任务状态
	if job.Enabled {
		if task.Status == TaskStatusPending {
			// 保持待定状态
		}
	} else {
		task.Status = TaskStatusPending
		task.Column = ColumnTodo
	}

	if err := a.service.UpdateTask(task); err != nil {
		logger.Warn("Failed to update cron task metadata", "taskId", task.ID, "error", err)
	}
}

// OnCronJobExecuting 定时任务执行时调用
// 更新现有任务状态为执行中，不创建新任务
func (a *CronAdapter) OnCronJobExecuting(job *cron.CronJob) {
	if a.service == nil {
		return
	}

	logger.Info("Cron job executing", "cronId", job.ID, "name", job.Name)

	// 查找对应的看板任务
	tasks, err := a.service.ListTasks(&TaskFilter{
		Source: ptrSource(TaskSourceCron),
		Limit:  100,
	})
	if err != nil {
		logger.Warn("Failed to find cron task", "cronId", job.ID, "error", err)
		return
	}

	for _, task := range tasks {
		if task.SourceRef == job.ID {
			// 更新任务状态为执行中
			if task.Metadata == nil {
				task.Metadata = make(map[string]interface{})
			}
			task.Metadata["executingAt"] = time.Now().Format(time.RFC3339)
			task.Metadata["nextRunAtMs"] = job.State.NextRunAtMs

			// 增加执行次数
			execCount := 0
			if v, ok := task.Metadata["executionCount"].(int); ok {
				execCount = v
			}
			task.Metadata["executionCount"] = execCount + 1

			// 更新状态
			task.Status = TaskStatusRunning
			task.Column = ColumnInProgress

			if err := a.service.UpdateTask(task); err != nil {
				logger.Warn("Failed to update cron task", "taskId", task.ID, "error", err)
			} else {
				logger.Info("Cron task status updated to running", "taskId", task.ID, "cronId", job.ID, "executionCount", execCount+1)
			}
			return
		}
	}

	logger.Warn("No cron task found for executing job", "cronId", job.ID, "name", job.Name)
}

// OnCronJobCompleted 定时任务执行完成时调用
// 更新任务状态为完成/失败
func (a *CronAdapter) OnCronJobCompleted(job *cron.CronJob, result string, errMsg string) {
	if a.service == nil {
		return
	}

	logger.Info("Cron job completed", "cronId", job.ID, "name", job.Name, "hasError", errMsg != "")

	// 查找对应的看板任务
	tasks, err := a.service.ListTasks(&TaskFilter{
		Source: ptrSource(TaskSourceCron),
		Limit:  100,
	})
	if err != nil {
		logger.Warn("Failed to find cron task", "cronId", job.ID, "error", err)
		return
	}

	for _, task := range tasks {
		if task.SourceRef == job.ID {
			// 更新任务状态
			if task.Metadata == nil {
				task.Metadata = make(map[string]interface{})
			}
			task.Metadata["lastRunAtMs"] = job.State.LastRunAtMs
			task.Metadata["lastStatus"] = job.State.LastStatus
			task.Metadata["nextRunAtMs"] = job.State.NextRunAtMs

			// 单次任务：标记为完成/失败
			// 周期性任务：恢复为待定状态，等待下次执行
			isOneTime := job.Schedule.Kind == cron.ScheduleKindAt

			if errMsg != "" {
				task.Error = errMsg
				if isOneTime {
					task.Status = TaskStatusFailed
					task.Column = ColumnDone
				} else {
					// 周期性任务执行失败，恢复待定状态
					task.Status = TaskStatusPending
					task.Column = ColumnTodo
				}
				if err := a.service.UpdateTask(task); err != nil {
					logger.Warn("Failed to update cron task", "taskId", task.ID, "error", err)
				} else {
					logger.Info("Cron task marked as failed", "taskId", task.ID, "cronId", job.ID)
				}
			} else {
				task.Result = result
				if isOneTime {
					task.Status = TaskStatusCompleted
					task.Column = ColumnDone
				} else {
					// 周期性任务执行成功，恢复待定状态
					task.Status = TaskStatusPending
					task.Column = ColumnTodo
				}
				if err := a.service.UpdateTask(task); err != nil {
					logger.Warn("Failed to update cron task", "taskId", task.ID, "error", err)
				} else {
					logger.Info("Cron task marked as completed", "taskId", task.ID, "cronId", job.ID)
				}
			}
			return
		}
	}

	logger.Warn("No cron task found for completed job", "cronId", job.ID, "name", job.Name)
}

// OnCronJobRemoved 定时任务删除时调用
func (a *CronAdapter) OnCronJobRemoved(job *cron.CronJob) {
	if a.service == nil {
		return
	}

	// 查找并删除相关的看板任务
	tasks, err := a.service.ListTasks(&TaskFilter{
		Source: ptrSource(TaskSourceCron),
		Limit:  200,
	})
	if err != nil {
		logger.Warn("Failed to find cron task for removal", "cronId", job.ID, "error", err)
		return
	}

	for _, task := range tasks {
		if task.SourceRef == job.ID {
			if err := a.service.DeleteTask(task.ID); err != nil {
				logger.Warn("Failed to delete cron task", "taskId", task.ID, "error", err)
			} else {
				logger.Info("Cron task deleted", "taskId", task.ID, "cronId", job.ID)
			}
		}
	}
}

// ptrSource 返回 TaskSource 指针
func ptrSource(s TaskSource) *TaskSource {
	return &s
}
