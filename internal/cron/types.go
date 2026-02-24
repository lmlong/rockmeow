// Package cron 定时任务管理
package cron

import "time"

// ScheduleKind 调度类型
type ScheduleKind string

const (
	ScheduleKindAt    ScheduleKind = "at"    // 一次性任务，指定时间执行
	ScheduleKindEvery ScheduleKind = "every" // 重复任务，指定间隔执行
	ScheduleKindCron  ScheduleKind = "cron"  // cron 表达式
)

// JobStatus 任务状态
type JobStatus string

const (
	JobStatusOK      JobStatus = "ok"
	JobStatusError   JobStatus = "error"
	JobStatusSkipped JobStatus = "skipped"
)

// PayloadKind 负载类型
type PayloadKind string

const (
	PayloadKindSystemEvent PayloadKind = "system_event" // 系统事件
	PayloadKindAgentTurn   PayloadKind = "agent_turn"   // Agent 对话
)

// CronSchedule 调度定义
type CronSchedule struct {
	Kind ScheduleKind `json:"kind"`

	// At 类型：执行时间戳（毫秒）
	AtMs int64 `json:"atMs,omitempty"`

	// Every 类型：间隔时间（毫秒）
	EveryMs int64 `json:"everyMs,omitempty"`

	// Cron 类型：cron 表达式（如 "0 9 * * *"）
	Expr string `json:"expr,omitempty"`

	// 时区（用于 cron 表达式）
	TZ string `json:"tz,omitempty"`
}

// CronPayload 任务负载
type CronPayload struct {
	Kind    PayloadKind `json:"kind"`
	Message string      `json:"message"`

	// 执行模式
	Execute bool `json:"execute"` // 是否先执行 Agent，true=先执行再通知，false=仅通知

	// 是否将响应发送到渠道
	Deliver bool   `json:"deliver"`
	Channel string `json:"channel,omitempty"` // 如 "feishu"
	To      string `json:"to,omitempty"`      // 如用户 ID

	// 源任务ID（用于关联用户请求任务，定时任务完成后同时完成源任务）
	SourceTaskID string `json:"sourceTaskId,omitempty"`
}

// CronJobState 任务运行时状态
type CronJobState struct {
	NextRunAtMs  int64     `json:"nextRunAtMs,omitempty"`
	LastRunAtMs  int64     `json:"lastRunAtMs,omitempty"`
	LastStatus   JobStatus `json:"lastStatus,omitempty"`
	LastError    string    `json:"lastError,omitempty"`
	LastResponse string    `json:"lastResponse,omitempty"`
}

// CronJob 定时任务
type CronJob struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Enabled        bool         `json:"enabled"`
	Schedule       CronSchedule `json:"schedule"`
	Payload        CronPayload  `json:"payload"`
	State          CronJobState `json:"state"`
	CreatedAtMs    int64        `json:"createdAtMs"`
	UpdatedAtMs    int64        `json:"updatedAtMs"`
	DeleteAfterRun bool         `json:"deleteAfterRun"` // 执行后删除（一次性任务）
}

// CronStore 任务存储
type CronStore struct {
	Version int        `json:"version"`
	Jobs    []*CronJob `json:"jobs"`
}

// JobCallback 任务执行回调函数
type JobCallback func(job *CronJob) (string, error)

// EventCallback 事件回调函数（用于任务看板同步等）
// eventType: "before", "after"
type EventCallback func(job *CronJob, eventType string, result string, errMsg string)

// nowMs 获取当前时间戳（毫秒）
func nowMs() int64 {
	return time.Now().UnixMilli()
}
