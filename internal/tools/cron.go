package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lingguard/internal/cron"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/utils"
)

// CronService 定时任务服务接口
type CronService interface {
	ListJobs(includeDisabled bool) []*cron.CronJob
	AddJob(name string, schedule cron.CronSchedule, message string, opts ...cron.JobOption) (*cron.CronJob, error)
	RemoveJob(id string) bool
	EnableJob(id string, enabled bool) *cron.CronJob
}

// CronServiceWithChannel 带渠道上下文的服务接口
type CronServiceWithChannel interface {
	CronService
	SetChannelContext(channel, to string)
}

// CronTool 定时任务管理工具
type CronTool struct {
	service CronService
}

// NewCronTool 创建定时任务工具
func NewCronTool(service CronService) *CronTool {
	return &CronTool{service: service}
}

func (t *CronTool) Name() string { return "cron" }

func (t *CronTool) Description() string {
	return `定时任务调度管理。支持的操作：
- list: 列出所有定时任务（用户询问定时任务列表时使用）
- add: 添加新的定时任务
- remove: 删除定时任务
- enable/disable: 启用/禁用任务
重要：当任务需要执行操作（如搜索、收集、整理、分析等）时，必须设置execute=true`
}

func (t *CronTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"list", "add", "remove", "enable", "disable"},
				"description": "操作类型：list=查看所有定时任务，add=创建新任务，remove=删除任务，enable/disable=启用/禁用",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "任务名称",
			},
			"schedule": map[string]interface{}{
				"type":        "string",
				"description": "时间（格式：cron:分 时 * * * 或 every:1h 或 at:2024-01-01T10:00）",
			},
			"message": map[string]interface{}{
				"type":        "string",
				"description": "任务内容",
			},
			"job_id": map[string]interface{}{
				"type":        "string",
				"description": "任务ID",
			},
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "时区（如Asia/Shanghai）",
			},
			"execute": map[string]interface{}{
				"type":        "boolean",
				"description": "执行模式：true=先执行Agent处理任务再通知结果（用于搜索、收集、整理等需要执行操作的任务），false=仅发送通知（用于简单提醒）。默认false。",
			},
		},
		"required": []string{"action"},
	}
}

func (t *CronTool) Execute(ctx context.Context, params json.RawMessage) (string, error) {
	var p struct {
		Action   string `json:"action"`
		Name     string `json:"name"`
		Schedule string `json:"schedule"`
		Message  string `json:"message"`
		JobID    string `json:"job_id"`
		Timezone string `json:"timezone"`
		Execute  bool   `json:"execute"`
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	switch p.Action {
	case "list":
		return t.listJobs()
	case "add":
		return t.addJob(p.Name, p.Schedule, p.Message, p.Timezone, p.Execute)
	case "remove":
		return t.removeJob(p.JobID)
	case "enable":
		return t.enableJob(p.JobID, true)
	case "disable":
		return t.enableJob(p.JobID, false)
	default:
		return "", fmt.Errorf("unknown action: %s", p.Action)
	}
}

func (t *CronTool) listJobs() (string, error) {
	jobs := t.service.ListJobs(true)
	if len(jobs) == 0 {
		return "No scheduled tasks found.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d scheduled tasks:\n\n", len(jobs)))

	for _, job := range jobs {
		status := "✓"
		if !job.Enabled {
			status = "✗"
		}
		sb.WriteString(fmt.Sprintf("- [%s] ID: %s | Name: %s\n", status, job.ID, job.Name))
		sb.WriteString(fmt.Sprintf("  Schedule: %s\n", formatSchedule(job.Schedule)))
		sb.WriteString(fmt.Sprintf("  Next Run: %s\n", formatTime(job.State.NextRunAtMs)))
		if job.State.LastRunAtMs > 0 {
			sb.WriteString(fmt.Sprintf("  Last Run: %s (%s)\n", formatTime(job.State.LastRunAtMs), job.State.LastStatus))
		}
		sb.WriteString(fmt.Sprintf("  Message: %s\n", utils.TruncateString(job.Payload.Message, 100)))
	}

	return sb.String(), nil
}

func (t *CronTool) addJob(name, scheduleStr, message, timezone string, execute bool) (string, error) {
	if name == "" || scheduleStr == "" || message == "" {
		return "", fmt.Errorf("name, schedule, and message are required")
	}

	schedule, err := parseSchedule(scheduleStr, timezone)
	if err != nil {
		return "", err
	}

	var opts []cron.JobOption

	// 设置执行模式
	if execute {
		opts = append(opts, cron.WithExecute(true))
	}

	// 设置投递渠道
	if w, ok := t.service.(*CronServiceWrapper); ok && w.Channel != "" {
		opts = append(opts, cron.WithDeliver(w.Channel, w.ChannelTo))
		logger.Debug("CronTool setting deliver", "channel", w.Channel, "to", w.ChannelTo)
	}

	job, err := t.service.AddJob(name, *schedule, message, opts...)
	if err != nil {
		return "", err
	}

	result := fmt.Sprintf("Task created!\n- ID: %s\n- Name: %s\n- Schedule: %s\n- Next Run: %s",
		job.ID, job.Name, formatSchedule(job.Schedule), formatTime(job.State.NextRunAtMs))

	if job.Payload.Deliver {
		result += fmt.Sprintf("\n- Notify on: %s", job.Payload.Channel)
	}

	if job.Payload.Execute {
		result += "\n- Mode: 🤖 Execute + Notify"
	} else {
		result += "\n- Mode: 📢 Notify only"
	}

	return result, nil
}

func (t *CronTool) removeJob(id string) (string, error) {
	if id == "" {
		return "", fmt.Errorf("job_id is required")
	}
	if t.service.RemoveJob(id) {
		return fmt.Sprintf("Task %s removed.", id), nil
	}
	return "", fmt.Errorf("task %s not found", id)
}

func (t *CronTool) enableJob(id string, enabled bool) (string, error) {
	if id == "" {
		return "", fmt.Errorf("job_id is required")
	}

	job := t.service.EnableJob(id, enabled)
	if job == nil {
		return "", fmt.Errorf("task %s not found", id)
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}

	result := fmt.Sprintf("Task '%s' %s.", job.Name, action)
	if enabled {
		result += fmt.Sprintf(" Next run: %s", formatTime(job.State.NextRunAtMs))
	}
	return result, nil
}

func (t *CronTool) IsDangerous() bool { return false }

// parseSchedule 解析调度字符串
func parseSchedule(s string, tz string) (*cron.CronSchedule, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid schedule format, use: every:<duration>, at:<datetime>, or cron:<expr>")
	}

	kind := strings.ToLower(parts[0])
	value := parts[1]

	switch kind {
	case "every":
		duration, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("invalid duration: %w", err)
		}
		if duration < time.Minute {
			return nil, fmt.Errorf("minimum interval is 1 minute")
		}
		return &cron.CronSchedule{
			Kind:    cron.ScheduleKindEvery,
			EveryMs: duration.Milliseconds(),
		}, nil

	case "at":
		t, err := utils.ParseTime(value)
		if err != nil {
			return nil, fmt.Errorf("invalid datetime: %w", err)
		}
		return &cron.CronSchedule{
			Kind: cron.ScheduleKindAt,
			AtMs: t.UnixMilli(),
		}, nil

	case "cron":
		return &cron.CronSchedule{
			Kind: cron.ScheduleKindCron,
			Expr: value,
			TZ:   tz,
		}, nil

	default:
		return nil, fmt.Errorf("unknown schedule kind: %s", kind)
	}
}

// formatSchedule 格式化调度信息
func formatSchedule(s cron.CronSchedule) string {
	switch s.Kind {
	case cron.ScheduleKindEvery:
		return fmt.Sprintf("every %s", time.Duration(s.EveryMs)*time.Millisecond)
	case cron.ScheduleKindAt:
		return fmt.Sprintf("at %s", formatTime(s.AtMs))
	case cron.ScheduleKindCron:
		if s.TZ != "" {
			return fmt.Sprintf("cron: %s (TZ: %s)", s.Expr, s.TZ)
		}
		return fmt.Sprintf("cron: %s", s.Expr)
	default:
		return string(s.Kind)
	}
}

// formatTime 格式化时间戳
func formatTime(ms int64) string {
	if ms == 0 {
		return "not scheduled"
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
}
