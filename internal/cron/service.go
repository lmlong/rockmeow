package cron

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/internal/to
	"github.com/lingguard/pkg/logger"
	"github.com/robfig/cron/v3"
)


)

// Service 定时任务服务
type Service struct 
	taskSyncer tools.TaskSyncer // 任务看板同步器
	storePath string
	onJob     JobCallback

	mu       sync.RWMutex
	store    *CronStore
	timer    *time.Timer
	running  bool
	stopChan chan struct{}
func NewService(storePath string, onJob JobCallback, taskSyncer tools.TaskSyncer) *Service {
	if taskSyncer == nil {
		taskSyncer = &tools.NoopTaskSyncer{}
	}

		storePath:  storePath,
		onJob:      onJob,
		taskSyncer: taskSyncer,
		stopChan:   make(chan struct{}),
		storePath: storePath,
		onJob:     onJob,
		stopChan:  make(chan struct{}),
	}
}

// Start 启动定时任务服务
func (s *Service) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	// 加载存储
	if err := s.loadStore(); err != nil {
		return fmt.Errorf("load store: %w", err)
	}

	// 重新计算下次执行时间
	s.recomputeNextRuns()
	s.saveStore()

	// 启动定时器
	s.running = true
	s.armTimer()

	logger.Info("Cron service started", "jobs", len(s.store.Jobs))
	return nil
}

// Stop 停止定时任务服务
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running = false
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	close(s.stopChan)
	logger.Info("Cron service stopped")
}

// loadStore 从磁盘加载任务存储
func (s *Service) loadStore() error {
	if s.store != nil {
		return nil
	}

	s.store = &CronStore{Version: 1}

	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, s.store); err != nil {
		logger.Warn("Failed to parse cron store", "error", err)
		s.store = &CronStore{Version: 1}
		return nil
	}

	return nil
}

// saveStore 保存任务存储到磁盘
func (s *Service) saveStore() error {
	if s.store == nil {
		return nil
	}

	dir := filepath.Dir(s.storePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.store, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.storePath, data, 0644)
}

// computeNextRun 计算下次执行时间
func computeNextRun(schedule *CronSchedule, nowMs int64) int64 {
	switch schedule.Kind {
	case ScheduleKindAt:
		if schedule.AtMs > nowMs {
			return schedule.AtMs
		}
		return 0

	case ScheduleKindEvery:
		if schedule.EveryMs <= 0 {
			return 0
		}
		return nowMs + schedule.EveryMs

	case ScheduleKindCron:
		if schedule.Expr == "" {
			return 0
		}
		// 使用简单的 cron 解析（需要导入 cron 库）
		// 这里先返回 0，后续可以集成 robfig/cron
		return parseCronNextRun(schedule.Expr, schedule.TZ, nowMs)
	}

	return 0
}

// parseCronNextRun 解析 cron 表达式并计算下次执行时间
func parseCronNextRun(expr, tz string, nowMs int64) int64 {
	// 解析时区
	loc := time.Local // 默认使用本地时区
	if tz != "" {
		var err error
		loc, err = time.LoadLocation(tz)
		if err != nil {
			logger.Warn("Invalid timezone, using local time", "timezone", tz, "error", err)
			loc = time.Local
		}
	}

	// 使用 robfig/cron 库解析表达式
	// 使用 WithSeconds 支持可选的秒字段，或使用标准5字段
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	// 创建带时区的 cron 调度器
	cronSchedule, err := parser.Parse(expr)
	if err != nil {
		logger.Warn("Invalid cron expression", "expr", expr, "error", err)
		return 0
	}

	// 将当前时间转换到目标时区
	now := time.UnixMilli(nowMs).In(loc)
	next := cronSchedule.Next(now)
	return next.UnixMilli()
}

// recomputeNextRuns 重新计算所有启用任务的下次执行时间
func (s *Service) recomputeNextRuns() {
	if s.store == nil {
		return
	}

	now := nowMs()
	for _, job := range s.store.Jobs {
		if job.Enabled {
			job.State.NextRunAtMs = computeNextRun(&job.Schedule, now)
		}
	}
}

// getNextWakeMs 获取最早的下次执行时间
func (s *Service) getNextWakeMs() int64 {
	if s.store == nil {
		return 0
	}

	var minMs int64 = 0
	for _, job := range s.store.Jobs {
		if job.Enabled && job.State.NextRunAtMs > 0 {
			if minMs == 0 || job.State.NextRunAtMs < minMs {
				minMs = job.State.NextRunAtMs
			}
		}
	}
	return minMs
}

// armTimer 设置定时器
func (s *Service) armTimer() {
	if s.timer != nil {
		s.timer.Stop()
	}

	nextWake := s.getNextWakeMs()
	if nextWake == 0 || !s.running {
		return
	}

	delay := time.Duration(nextWake-nowMs()) * time.Millisecond
	if delay < 0 {
		delay = 0
	}

	s.timer = time.AfterFunc(delay, s.onTimer)
}

// onTimer 定时器触发
func (s *Service) onTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.store == nil {
		return
	}

	now := nowMs()
	var dueJobs []*CronJob

	for _, job := range s.store.Jobs {
		if job.Enabled && job.State.NextRunAtMs > 0 && now >= job.State.NextRunAtMs {
			dueJobs = append(dueJobs, job)
		}
	}

	for _, job := range dueJobs {
		s.executeJob(job)
	}

	s.saveStore()
	s.armTimer()
}

// executeJob 执行单个任务
func (s *Service) executeJob(job *CronJob) {
	startMs := nowMs()
	logger.Info("Cron executing job", "name", job.Name, "id", job.ID)

	var response string
	var err error

	if s.onJob != nil {
		response, err = s.onJob(job)
	}

	if err != nil {
		job.State.LastStatus = JobStatusError
		job.State.LastError = err.Error()
		logger.Error("Cron job failed", "name", job.Name, "error", err)
	} else {
		job.State.LastStatus = JobStatusOK
		job.State.LastError = ""
		job.State.LastResponse = response
		logger.Info("Cron job completed", "name", job.Name)
	}

	job.State.LastRunAtMs = startMs
	job.UpdatedAtMs = nowMs()

	// 处理一次性任务
	if job.Schedule.Kind == ScheduleKindAt {
		if job.DeleteAfterRun {
			s.store.Jobs = removeJob(s.store.Jobs, job.ID)
		} else {
			job.Enabled = false
			job.State.NextRunAtMs = 0
		}
	} else {
		// 计算下次执行时间
		job.State.NextRunAtMs = computeNextRun(&job.Schedule, nowMs())
	}
}

// removeJob 从切片中删除任务
func removeJob(jobs []*CronJob, id string) []*CronJob {
	for i, j := range jobs {
		if j.ID == id {
			return append(jobs[:i], jobs[i+1:]...)
		}
	}
	return jobs
}

// ========== Public API ==========

// ListJobs 列出所有任务
func (s *Service) ListJobs(includeDisabled bool) []*CronJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.store == nil {
		return nil
	}

	var jobs []*CronJob
	if includeDisabled {
		jobs = append(jobs, s.store.Jobs...)
	} else {
		for _, j := range s.store.Jobs {
			if j.Enabled {
				jobs = append(jobs, j)
			}
		}
	}

	// 按下次执行时间排序
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].State.NextRunAtMs == 0 {
			return false
		}
		if jobs[j].State.NextRunAtMs == 0 {
			return true
		}
		return jobs[i].State.NextRunAtMs < jobs[j].State.NextRunAtMs
	})

	return jobs
}

// GetJob 获取单个任务
func (s *Service) GetJob(id string) *CronJob {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.store == nil {
		return nil
	}

	for _, job := range s.store.Jobs {
		if job.ID == id {
			return job
		}
	}
	return nil
}

// AddJob 添加新任务
func (s *Service) AddJob(name string, schedule CronSchedule, message string, opts ...JobOption) (*CronJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.loadStore(); err != nil {
		return nil, err
	}

	now := nowMs()
	job := &CronJob{
		ID:          uuid.New().String()[:8],
		Name:        name,
		Enabled:     true,
		Schedule:    schedule,
		Payload:     CronPayload{Kind: PayloadKindAgentTurn, Message: message},
		State:       CronJobState{NextRunAtMs: computeNextRun(&schedule, now)},
		CreatedAtMs: now,
		UpdatedAtMs: now,
	}

	// 应用选项
	for _, opt := range opts {
		opt(job)
	}

	s.store.Jobs = append(s.store.Jobs, job)
	s.saveStore()
	s.armTimer()

	logger.Info("Cron added job", "name", name, "id", job.ID)
	return job, nil
}

// JobOption 任务选项函数
type JobOption func(*CronJob)

// WithDeliver 设置任务响应投递
func WithDeliver(channel, to string) JobOption {
	return func(j *CronJob) {
		j.Payload.Deliver = true
		j.Payload.Channel = channel
		j.Payload.To = to
	}
}

// WithDeleteAfterRun 设置执行后删除
func WithDeleteAfterRun() JobOption {
	return func(j *CronJob) {
		j.DeleteAfterRun = true
	}
}

// RemoveJob 删除任务
func (s *Service) RemoveJob(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store == nil {
		return false
	}

	before := len(s.store.Jobs)
	s.store.Jobs = removeJob(s.store.Jobs, id)
	removed := len(s.store.Jobs) < before

	if removed {
		s.saveStore()
		s.armTimer()
		logger.Info("Cron removed job", "id", id)
	}

	return removed
}

// EnableJob 启用/禁用任务
func (s *Service) EnableJob(id string, enabled bool) *CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store == nil {
		return nil
	}

	for _, job := range s.store.Jobs {
		if job.ID == id {
			job.Enabled = enabled
			job.UpdatedAtMs = nowMs()

			if enabled {
				job.State.NextRunAtMs = computeNextRun(&job.Schedule, nowMs())
			} else {
				job.State.NextRunAtMs = 0
			}

			s.saveStore()
			s.armTimer()
			return job
		}
	}

	return nil
}

// RunJob 手动执行任务
func (s *Service) RunJob(id string, force bool) (*CronJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store == nil {
		return nil, fmt.Errorf("store not loaded")
	}

	for _, job := range s.store.Jobs {
		if job.ID == id {
			if !force && !job.Enabled {
				return nil, fmt.Errorf("job is disabled")
			}

			s.executeJob(job)
			s.saveStore()
			s.armTimer()
			return job, nil
		}
	}

	return nil, fmt.Errorf("job not found: %s", id)
}

// Status 获取服务状态
func (s *Service) Status() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobCount := 0
	if s.store != nil {
		jobCount = len(s.store.Jobs)
	}

	return map[string]interface{}{
		"running":       s.running,
		"jobs":          jobCount,
		"nextWakeAtMs":  s.getNextWakeMs(),
		"nextWakeAtStr": formatTime(s.getNextWakeMs()),
	}
}

// formatTime 格式化时间戳
func formatTime(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
}

// ========== 便捷方法 ==========

// AddAtJob 添加一次性任务
func (s *Service) AddAtJob(name string, at time.Time, message string, opts ...JobOption) (*CronJob, error) {
	schedule := CronSchedule{
		Kind: ScheduleKindAt,
		AtMs: at.UnixMilli(),
	}
	return s.AddJob(name, schedule, message, opts...)
}

// AddEveryJob 添加重复任务
func (s *Service) AddEveryJob(name string, interval time.Duration, message string, opts ...JobOption) (*CronJob, error) {
	schedule := CronSchedule{
		Kind:    ScheduleKindEvery,
		EveryMs: interval.Milliseconds(),
	}
	return s.AddJob(name, schedule, message, opts...)
}

// AddCronJob 添加 cron 表达式任务（带时区）
func (s *Service) AddCronJob(name string, expr string, message string, opts ...JobOption) (*CronJob, error) {
	schedule := CronSchedule{
		Kind: ScheduleKindCron,
		Expr: expr,
	}
	return s.AddJob(name, schedule, message, opts...)
}

// AddCronJobWithTZ 添加 cron 表达式任务（指定时区）
func (s *Service) AddCronJobWithTZ(name string, expr string, tz string, message string, opts ...JobOption) (*CronJob, error) {
	schedule := CronSchedule{
		Kind: ScheduleKindCron,
		Expr: expr,
		TZ:   tz,
	}
	return s.AddJob(name, schedule, message, opts...)
}
