package tools

import (
	"github.com/lingguard/internal/cron"
)

// CronServiceWrapper 包装 cron.Service 实现 CronService 接口
type CronServiceWrapper struct {
	Service   *cron.Service
	Channel   string // 当前渠道名称
	ChannelTo string // 当前用户/群组 ID
}

// NewCronServiceWrapper 创建包装器
func NewCronServiceWrapper(service *cron.Service) *CronServiceWrapper {
	return &CronServiceWrapper{Service: service}
}

// SetChannelContext 设置当前渠道上下文
func (w *CronServiceWrapper) SetChannelContext(channel, to string) {
	w.Channel = channel
	w.ChannelTo = to
}

// ListJobs 列出所有任务
func (w *CronServiceWrapper) ListJobs(includeDisabled bool) []*cron.CronJob {
	return w.Service.ListJobs(includeDisabled)
}

// AddJob 添加任务
func (w *CronServiceWrapper) AddJob(name string, schedule cron.CronSchedule, message string, opts ...cron.JobOption) (*cron.CronJob, error) {
	return w.Service.AddJob(name, schedule, message, opts...)
}

// RemoveJob 删除任务
func (w *CronServiceWrapper) RemoveJob(id string) bool {
	return w.Service.RemoveJob(id)
}

// EnableJob 启用/禁用任务
func (w *CronServiceWrapper) EnableJob(id string, enabled bool) *cron.CronJob {
	return w.Service.EnableJob(id, enabled)
}
