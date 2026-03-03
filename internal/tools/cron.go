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
	UpdateJob(id string, opts cron.UpdateJobOptions) (*cron.CronJob, error)
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
- update: 更新现有任务（修改时间、名称、内容等）
- remove: 删除定时任务
- enable/disable: 启用/禁用任务

⚠️ 关键规则：判断 execute 参数
- message 包含"搜索/查询/收集/整理/分析/生成/检查/抓取/访问"等动作词 → 必须设置 execute=true
- message 只是提醒文字（如"该开会了"、"记得吃药"）→ 不设置 execute

示例判断：
- "搜索AI新闻推送给我" → 需要先搜索 → execute=true
- "查询天气推送给我" → 需要先查询 → execute=true
- "提醒我开会" → 只是通知 → 不需要 execute
- "每小时叫我休息" → 只是通知 → 不需要 execute`
}

func (t *CronTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"list", "add", "update", "remove", "enable", "disable"},
				"description": "操作类型：list=查看所有定时任务，add=创建新任务，update=更新任务，remove=删除任务，enable/disable=启用/禁用",
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
				"description": "任务ID（更新/删除/启用/禁用时必填）",
			},
			"timezone": map[string]interface{}{
				"type":        "string",
				"description": "时区（如Asia/Shanghai）",
			},
			"execute": map[string]interface{}{
				"type":        "boolean",
				"description": "【重要】执行模式：true=先执行Agent处理任务再通知结果（用于搜索、收集、整理、分析、推送等需要执行操作的任务），false或未设置=仅发送通知（用于简单提醒）。当message包含任何需要Agent执行的操作时必须设为true！",
			},
			"enabled": map[string]interface{}{
				"type":        "boolean",
				"description": "启用状态（更新任务时使用）",
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
		Execute  *bool  `json:"execute"` // 使用指针区分未设置和 false
		Enabled  *bool  `json:"enabled"` // 使用指针区分未设置和 false
	}

	if err := json.Unmarshal(params, &p); err != nil {
		return "", fmt.Errorf("invalid parameters: %w", err)
	}

	switch p.Action {
	case "list":
		return t.listJobs()
	case "add":
		// 智能判断 execute 模式
		// 1. 如果 LLM 显式设置了 execute 参数，使用该值
		// 2. 否则，自动检测 message 中是否包含动作关键词
		var execute bool
		if p.Execute != nil {
			execute = *p.Execute
			logger.Info("CronTool addJob (explicit)", "name", p.Name, "execute", execute)
		} else {
			// 自动检测：如果 message 包含动作关键词，自动设置 execute=true
			execute = detectExecuteMode(p.Message)
			logger.Info("CronTool addJob (auto-detect)", "name", p.Name, "execute", execute)
		}
		return t.addJob(p.Name, p.Schedule, p.Message, p.Timezone, execute)
	case "update":
		// 智能判断 execute 模式（如果更新了 message 但没有显式设置 execute）
		executeParam := p.Execute
		if p.Message != "" && p.Execute == nil {
			// message 更新了，自动检测是否需要执行模式
			if detectExecuteMode(p.Message) {
				execute := true
				executeParam = &execute
				logger.Info("CronTool updateJob (auto-detect execute)", "job_id", p.JobID, "execute", true)
			}
		}
		return t.updateJob(p.JobID, p.Name, p.Schedule, p.Message, p.Timezone, p.Enabled, executeParam)
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
		// 显示执行模式
		if job.Payload.Execute {
			sb.WriteString("  Mode: 🤖 Execute + Notify\n")
		} else {
			sb.WriteString("  Mode: 📢 Notify only\n")
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

	// 设置执行模式 - 记录日志便于调试
	logger.Info("CronTool addJob", "name", name, "execute", execute)

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

func (t *CronTool) updateJob(id, name, scheduleStr, message, timezone string, enabled *bool, execute *bool) (string, error) {
	if id == "" {
		return "", fmt.Errorf("job_id is required")
	}

	opts := cron.UpdateJobOptions{}

	// 更新名称
	if name != "" {
		opts.Name = &name
	}

	// 更新调度
	if scheduleStr != "" {
		schedule, err := parseSchedule(scheduleStr, timezone)
		if err != nil {
			return "", err
		}
		opts.Schedule = schedule
	}

	// 更新消息
	if message != "" {
		opts.Message = &message
	}

	// 更新启用状态
	if enabled != nil {
		opts.Enabled = enabled
	}

	// 更新执行模式
	if execute != nil {
		opts.Execute = execute
		logger.Info("CronTool updateJob", "id", id, "execute", *execute)
	}

	job, err := t.service.UpdateJob(id, opts)
	if err != nil {
		return "", err
	}

	var changes []string
	if name != "" {
		changes = append(changes, fmt.Sprintf("name=%s", job.Name))
	}
	if scheduleStr != "" {
		changes = append(changes, fmt.Sprintf("schedule=%s", formatSchedule(job.Schedule)))
	}
	if message != "" {
		changes = append(changes, "message updated")
	}
	if enabled != nil {
		status := "disabled"
		if *enabled {
			status = "enabled"
		}
		changes = append(changes, status)
	}
	if execute != nil {
		mode := "notify only"
		if *execute {
			mode = "execute + notify"
		}
		changes = append(changes, fmt.Sprintf("mode=%s", mode))
	}

	result := fmt.Sprintf("Task '%s' updated!\n- ID: %s\n- Changes: %s\n- Next Run: %s",
		job.Name, job.ID, strings.Join(changes, ", "), formatTime(job.State.NextRunAtMs))

	// 显示当前执行模式
	if job.Payload.Execute {
		result += "\n- Mode: 🤖 Execute + Notify"
	} else {
		result += "\n- Mode: 📢 Notify only"
	}

	return result, nil
}

func (t *CronTool) IsDangerous() bool { return false }

func (t *CronTool) ShouldLoadByDefault() bool { return false }

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

// executeActionKeywords 需要执行模式的动作关键词
var executeActionKeywords = []string{
	// 搜索/查询类
	"搜索", "查询", "查找", "检索", "寻找", "搜一下",
	// 收集/整理类
	"收集", "整理", "汇总", "统计", "归纳", "总结", "汇总",
	// 分析类
	"分析", "研究", "对比", "比较", "评估",
	// 生成类
	"生成", "创建", "编写", "撰写", "制作",
	// 检查类
	"检查", "监控", "检测", "访问", "抓取", "爬取", "获取",
	// 推送内容类（需要先获取内容）
	"推送天气", "推送新闻", "推送资讯", "推送动态",
}

// detectExecuteMode 自动检测是否需要执行模式
// 当 message 包含动作关键词时，返回 true
func detectExecuteMode(message string) bool {
	if message == "" {
		return false
	}

	lowerMsg := strings.ToLower(message)

	for _, keyword := range executeActionKeywords {
		if strings.Contains(lowerMsg, keyword) {
			logger.Info("CronTool auto-detect execute mode", "keyword", keyword, "message", utils.TruncateString(message, 50))
			return true
		}
	}

	return false
}
