package taskboard

import (
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
func (a *CronAdapter) OnCronJobCreated(job *cron.CronJob) {
	if a.service == nil {
		return
	}

	task, err := a.service.CreateCronTask(job.ID, job.Name, job.Payload.Message)
	if err != nil {
		logger.Warn("Failed to create cron task", "cronId", job.ID, "error", err)
		return
	}

	// 直接设为进行中状态
	if err := a.service.StartTask(task.ID); err != nil {
		logger.Warn("Failed to start cron task", "taskId", task.ID, "error", err)
	}

	logger.Info("Cron task created and started", "taskId", task.ID, "cronId", job.ID, "name", job.Name)
}

// OnCronJobExecuting 定时任务执行时调用（记录执行日志）
func (a *CronAdapter) OnCronJobExecuting(job *cron.CronJob) {
	if a.service == nil {
		return
	}

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
			// 更新任务，记录最后执行时间
			task.Metadata["lastRunAt"] = job.State.LastRunAtMs
			task.Metadata["runCount"] = incrementRunCount(task.Metadata)
			if err := a.service.UpdateTask(task); err != nil {
				logger.Warn("Failed to update cron task", "taskId", task.ID, "error", err)
			}
			break
		}
	}

	logger.Debug("Cron job executing", "cronId", job.ID, "name", job.Name)
}

// OnCronJobCompleted 定时任务执行完成时调用
func (a *CronAdapter) OnCronJobCompleted(job *cron.CronJob, result string, errMsg string) {
	if a.service == nil {
		return
	}

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
			// 判断是否为单次任务
			isOneTime := job.Schedule.Kind == cron.ScheduleKindAt

			if isOneTime {
				// 单次任务：完成后移到 Done 列
				if errMsg != "" {
					if err := a.service.FailTask(task.ID, errMsg); err != nil {
						logger.Warn("Failed to fail cron task", "taskId", task.ID, "error", err)
					}
				} else {
					if err := a.service.CompleteTask(task.ID, result); err != nil {
						logger.Warn("Failed to complete cron task", "taskId", task.ID, "error", err)
					}
				}
				logger.Info("One-time cron task completed", "taskId", task.ID, "cronId", job.ID)
			} else {
				// 周期性任务：保持进行中，只更新结果
				task.Result = result
				if errMsg != "" {
					task.Error = errMsg
				} else {
					task.Error = ""
				}
				task.Metadata["lastResult"] = result
				task.Metadata["lastStatus"] = string(job.State.LastStatus)
				if err := a.service.UpdateTask(task); err != nil {
					logger.Warn("Failed to update cron task", "taskId", task.ID, "error", err)
				}
				logger.Debug("Recurring cron task executed", "taskId", task.ID, "cronId", job.ID)
			}
			return
		}
	}
}

// OnCronJobRemoved 定时任务删除时调用
func (a *CronAdapter) OnCronJobRemoved(job *cron.CronJob) {
	if a.service == nil {
		return
	}

	// 查找并删除对应的看板任务
	tasks, err := a.service.ListTasks(&TaskFilter{
		Source: ptrSource(TaskSourceCron),
		Limit:  100,
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
			return
		}
	}
}

// incrementRunCount 增加运行次数
func incrementRunCount(metadata map[string]interface{}) int {
	if metadata == nil {
		return 1
	}
	count := 0
	if v, ok := metadata["runCount"].(float64); ok {
		count = int(v)
	}
	return count + 1
}
