// TODO(configuration): This file contains several magic numbers that should be configurable:
// - Default maxToolIterations: 10 (line ~496, ~650) - should use config value
// - Default sessionLockTimeout: 10 minutes (line ~273) - hardcoded default
// - Default memoryWindow: 50 messages - could be configurable per use case
// Recommended configuration structure in config.json:
// {
//   "agents": {
//     "maxToolIterations": 20,     // Current default
//     "sessionLockTimeout": 10,    // Minutes
//     "memoryWindow": 50,          // Number of messages
//     "reflectionPrompt": "..."    // Customizable prompt
//   }
// }
// Priority: P1 - Estimated effort: 2-3 days
// Related: #configuration #agent #magic-numbers

// Package agent 核心代理逻辑
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/cron"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/session"
	"github.com/lingguard/internal/skills"
	"github.com/lingguard/internal/subagent"
	"github.com/lingguard/internal/taskboard"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/internal/trace"
	"github.com/lingguard/pkg/llm"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/memory"
	"github.com/lingguard/pkg/stream"
)

// ErrSessionBusy 会话正在处理其他消息
var ErrSessionBusy = errors.New("会话正在处理上一条消息，请稍后再试")

// ReflectPrompt 反思提示（参考 nanobot）
const ReflectPrompt = "Reflect on the results and decide next steps."

// CronWrapper 定时任务服务包装器接口
type CronWrapper interface {
	SetChannelContext(channel, to string)
	ListJobs(includeDisabled bool) []*cron.CronJob
	AddJob(name string, schedule cron.CronSchedule, message string, opts ...cron.JobOption) (*cron.CronJob, error)
	RemoveJob(id string) bool
	EnableJob(id string, enabled bool) *cron.CronJob
}

// Agent 核心代理结构
type Agent struct {
	id                 string
	provider           providers.Provider // 主 Provider（文本）
	multimodalProvider providers.Provider // 多模态 Provider（图片/视频），可选
	toolRegistry       *tools.Registry
	sessions           *session.Manager
	skillsMgr          *skills.Manager
	subagentMgr        *subagent.SubagentManager
	config             *config.AgentsConfig
	memoryStore        *memory.FileStore      // 文件持久化存储（参考 nanobot）
	hybridStore        *memory.HybridStore    // 混合存储（文件+向量），可选
	memoryBuilder      *memory.ContextBuilder // 记忆上下文构建器
	memoryRefiner      *memory.Refiner        // 记忆提炼器
	sessionCompressor  *session.Compressor    // 会话压缩器
	traceCollector     trace.Collector        // 追踪采集器，可选
	taskboard          *taskboard.Service     // 定时任务看板服务（仅用于定时任务跟踪和分析）
	cronWrapper        CronWrapper            // 定时任务服务包装器
	sessionLockTimeout time.Duration          // 会话锁超时时间
	steerMgr           *steerManager          // Steer 模式管理器
	profileStore       *memory.ProfileStore   // 用户档案存储
	// 会话级别动态工具管理
	sessionDynamicTools   map[string]*sessionDynamicToolsInfo
	sessionDynamicToolsMu sync.RWMutex
}

// sessionDynamicToolsInfo 会话级别的动态工具信息（含老化机制）
type sessionDynamicToolsInfo struct {
	tools      []map[string]interface{} // 动态加载的工具定义
	lastUsed   map[string]int           // 工具名 -> 上次使用的请求序号
	requestSeq int                      // 当前会话的请求序号
}

const dynamicToolAgingThreshold = 10 // 连续10次请求未使用则卸载

// NewAgent 创建新代理
func NewAgent(cfg *config.AgentsConfig, provider providers.Provider, skillsLoader *skills.Loader) *Agent {
	return NewAgentWithMultimodal(cfg, provider, nil, skillsLoader)
}

// NewAgentWithMultimodal 创建带多模态支持的代理
func NewAgentWithMultimodal(cfg *config.AgentsConfig, provider providers.Provider, multimodalProvider providers.Provider, skillsLoader *skills.Loader) *Agent {
	return NewAgentWithMultimodalAndConfig(cfg, provider, multimodalProvider, skillsLoader, nil)
}

// NewAgentWithMultimodalAndConfig 创建带多模态支持和完整配置的代理
func NewAgentWithMultimodalAndConfig(cfg *config.AgentsConfig, provider providers.Provider, multimodalProvider providers.Provider, skillsLoader *skills.Loader, fullConfig *config.Config) *Agent {
	var skillsMgr *skills.Manager
	if skillsLoader != nil {
		skillsMgr = skills.NewManager(skillsLoader)
	}

	toolRegistry := tools.NewRegistry()

	// 初始化存储（参考 nanobot）
	// 记忆目录固定在 ~/.lingguard/memory/
	var memStore *memory.FileStore
	var hybridStore *memory.HybridStore
	var memBuilder *memory.ContextBuilder
	var refiner *memory.Refiner
	var sessionStore memory.Store

	// 记忆目录
	home, _ := os.UserHomeDir()
	memDir := filepath.Join(home, ".lingguard", "memory")

	// 始终初始化会话存储（每个会话一个JSON文件）
	sessionStore = memory.NewSessionStore(memDir)
	logger.Info("Session store initialized", "path", memDir+"/sessions/")

	if cfg.MemoryConfig != nil && cfg.MemoryConfig.Enabled {
		// 检查是否启用向量检索
		if cfg.MemoryConfig.Vector != nil && cfg.MemoryConfig.Vector.Enabled {
			// 使用混合存储（文件+向量）
			var providers map[string]config.ProviderConfig
			if fullConfig != nil {
				providers = fullConfig.Providers
			}

			hybridCfg := &memory.HybridStoreConfig{
				MemoryDir:      memDir,
				VectorConfig:   cfg.MemoryConfig.Vector,
				Providers:      providers,
				MaxDailyLogAge: cfg.MemoryConfig.MaxDailyLogAge,
			}

			var err error
			hybridStore, err = memory.NewHybridStore(hybridCfg)
			if err != nil {
				logger.Warn("Failed to init hybrid store, falling back to file store", "error", err)
				hybridStore = nil
			} else {
				memStore = hybridStore.FileStore()
				memBuilder = memory.NewContextBuilderWithHybrid(hybridStore)
				logger.Info("Hybrid memory store initialized with vector search", "path", memDir)
			}
		}

		// 如果向量存储初始化失败或未启用，使用纯文件存储
		if hybridStore == nil {
			memStore = memory.NewFileStore(memDir)
			if err := memStore.Init(); err != nil {
				logger.Warn("Failed to init file memory store", "error", err)
			} else {
				memBuilder = memory.NewContextBuilder(memStore)
				logger.Info("File-based memory store initialized", "path", memDir)
				// 清理过期日志
				if cfg.MemoryConfig.MaxDailyLogAge > 0 {
					if deleted, err := memStore.CleanOldDailyLogs(cfg.MemoryConfig.MaxDailyLogAge); err != nil {
						logger.Warn("Failed to clean old daily logs", "error", err)
					} else if deleted > 0 {
						logger.Info("Cleaned old daily logs", "deleted", deleted, "maxAge", cfg.MemoryConfig.MaxDailyLogAge)
					}
				}
			}
		}

		// 初始化记忆提炼器
		if cfg.MemoryConfig.Refine != nil && cfg.MemoryConfig.Refine.Enabled {
			refiner = memory.NewRefiner(memStore, hybridStore, cfg.MemoryConfig.Refine)
			logger.Info("Memory refiner initialized", "threshold", cfg.MemoryConfig.Refine.Threshold)
		}
	}

	// 初始化会话压缩器
	var sessionCompressor *session.Compressor
	if cfg.SessionCompress != nil && cfg.SessionCompress.Enabled {
		sessionCompressor = session.NewCompressor(provider, cfg.SessionCompress)
		logger.Info("Session compressor initialized",
			"threshold", cfg.SessionCompress.Threshold,
			"keepRecent", cfg.SessionCompress.KeepRecent)
	}

	// 如果没有配置多模态 provider，使用主 provider
	if multimodalProvider == nil {
		multimodalProvider = provider
	}

	agent := &Agent{
		id:                  generateID(),
		provider:            provider,
		multimodalProvider:  multimodalProvider,
		toolRegistry:        toolRegistry,
		sessions:            session.NewManager(sessionStore, cfg.MemoryWindow),
		skillsMgr:           skillsMgr,
		config:              cfg,
		memoryStore:         memStore,
		hybridStore:         hybridStore,
		memoryBuilder:       memBuilder,
		memoryRefiner:       refiner,
		sessionCompressor:   sessionCompressor,
		sessionDynamicTools: make(map[string]*sessionDynamicToolsInfo),
		profileStore:        memory.NewProfileStore(""),
	}

	// 初始化子代理管理器
	agent.subagentMgr = subagent.NewSubagentManager(provider, toolRegistry, nil)

	// 初始化 Steer 管理器
	agent.steerMgr = newSteerManager()

	return agent
}

// RegisterTool 注册工具
func (a *Agent) RegisterTool(t tools.Tool) {
	a.toolRegistry.Register(t)
}

// UnregisterTool 注销工具
func (a *Agent) UnregisterTool(name string) {
	a.toolRegistry.Unregister(name)
}

// RegisterSkillTool 注册技能加载工具
func (a *Agent) RegisterSkillTool() {
	if a.skillsMgr != nil {
		skillTool := tools.NewSkillTool(a.skillsMgr)
		skillTool.SetRegistry(a.toolRegistry)
		a.toolRegistry.Register(skillTool)
	}
}

// RegisterSubagentTools 注册子代理工具
func (a *Agent) RegisterSubagentTools() {
	if a.subagentMgr != nil {
		a.toolRegistry.Register(subagent.NewTaskTool(a.subagentMgr))
		a.toolRegistry.Register(subagent.NewTaskStatusTool(a.subagentMgr))
	}
}

// RegisterMemoryTool 注册记忆工具
func (a *Agent) RegisterMemoryTool() {
	if a.hybridStore != nil {
		// 使用混合存储（支持向量检索）
		memTool := tools.NewMemoryToolFromHybridStore(a.hybridStore)
		a.toolRegistry.Register(memTool)
	} else if a.memoryStore != nil {
		// 使用纯文件存储
		memTool := tools.NewMemoryToolFromStore(a.memoryStore)
		a.toolRegistry.Register(memTool)
	}
}

// RegisterCronTool 注册定时任务工具
func (a *Agent) RegisterCronTool(service tools.CronService) {
	if service != nil {
		a.toolRegistry.Register(tools.NewCronTool(service))
	}
}

// SetTaskboard 设置任务看板服务
func (a *Agent) SetTaskboard(service *taskboard.Service) {
	a.taskboard = service
}

// SetCronWrapper 设置 cron 包装器
func (a *Agent) SetCronWrapper(wrapper CronWrapper) {
	a.cronWrapper = wrapper
}

// SetTraceCollector 设置追踪采集器
func (a *Agent) SetTraceCollector(collector trace.Collector) {
	a.traceCollector = collector
}

// GetTraceCollector 获取追踪采集器
func (a *Agent) GetTraceCollector() trace.Collector {
	return a.traceCollector
}

// GetTaskboard 获取任务看板服务
func (a *Agent) GetTaskboard() *taskboard.Service {
	return a.taskboard
}

// RegisterTaskBoardTool 注册任务看板工具
func (a *Agent) RegisterTaskBoardTool() {
	if a.taskboard != nil {
		a.toolRegistry.Register(taskboard.NewTaskBoardTool(a.taskboard))
	}
}

// GetMemoryStore 获取记忆存储
func (a *Agent) GetMemoryStore() *memory.FileStore {
	return a.memoryStore
}

// GetHybridStore 获取混合存储（如果启用）
func (a *Agent) GetHybridStore() *memory.HybridStore {
	return a.hybridStore
}

// GetProfileStore 获取用户档案存储
func (a *Agent) GetProfileStore() *memory.ProfileStore {
	return a.profileStore
}

// GetSessionManager 获取会话管理器
func (a *Agent) GetSessionManager() *session.Manager {
	return a.sessions
}

// IsVectorSearchEnabled 检查是否启用向量检索
func (a *Agent) IsVectorSearchEnabled() bool {
	return a.hybridStore != nil && a.hybridStore.IsVectorEnabled()
}

// getSessionLockTimeout 获取会话锁超时时间
func (a *Agent) getSessionLockTimeout() time.Duration {
	if a.config != nil && a.config.SessionLockTimeout > 0 {
		return time.Duration(a.config.SessionLockTimeout) * time.Minute
	}
	return 10 * time.Minute // 默认 10 分钟
}

// SubagentManager 返回子代理管理器

// ToolRegistry 返回工具注册表
func (a *Agent) ToolRegistry() *tools.Registry {
	return a.toolRegistry
}

// getOrCreateSessionDynamicTools 获取或创建会话的动态工具信息
func (a *Agent) getOrCreateSessionDynamicTools(sessionID string) *sessionDynamicToolsInfo {
	a.sessionDynamicToolsMu.Lock()
	defer a.sessionDynamicToolsMu.Unlock()

	info, exists := a.sessionDynamicTools[sessionID]
	if !exists {
		info = &sessionDynamicToolsInfo{
			tools:      make([]map[string]interface{}, 0),
			lastUsed:   make(map[string]int),
			requestSeq: 0,
		}
		a.sessionDynamicTools[sessionID] = info
	}
	return info
}

// incrementSessionRequestSeq 增加会话请求序号
func (a *Agent) incrementSessionRequestSeq(sessionID string) int {
	a.sessionDynamicToolsMu.Lock()
	defer a.sessionDynamicToolsMu.Unlock()

	info, exists := a.sessionDynamicTools[sessionID]
	if !exists {
		info = &sessionDynamicToolsInfo{
			tools:      make([]map[string]interface{}, 0),
			lastUsed:   make(map[string]int),
			requestSeq: 0,
		}
		a.sessionDynamicTools[sessionID] = info
	}
	info.requestSeq++
	return info.requestSeq
}

// addDynamicToolsForSession 为指定会话添加动态工具
func (a *Agent) addDynamicToolsForSession(sessionID string, toolDefs []map[string]interface{}) {
	info := a.getOrCreateSessionDynamicTools(sessionID)

	a.sessionDynamicToolsMu.Lock()
	defer a.sessionDynamicToolsMu.Unlock()

	// 获取当前请求序号
	currentSeq := info.requestSeq

	// 合并工具（去重：以工具名称为准）
	existingNames := make(map[string]bool)
	for _, t := range info.tools {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				existingNames[name] = true
			}
		}
	}

	addedCount := 0
	for _, t := range toolDefs {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				if !existingNames[name] {
					info.tools = append(info.tools, t)
					info.lastUsed[name] = currentSeq
					addedCount++
					logger.Info("[DynamicTools] Tool added to session", "session", sessionID, "tool", name)
				}
			}
		}
	}

	logger.Info("[DynamicTools] Session tools updated", "session", sessionID, "added", addedCount, "total", len(info.tools))
}

// markDynamicToolUsed 标记动态工具被使用（更新 lastUsed）
func (a *Agent) markDynamicToolUsed(sessionID, toolName string) {
	a.sessionDynamicToolsMu.Lock()
	defer a.sessionDynamicToolsMu.Unlock()

	info, exists := a.sessionDynamicTools[sessionID]
	if !exists {
		return
	}

	// 检查是否是动态工具
	for _, t := range info.tools {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok && name == toolName {
				info.lastUsed[toolName] = info.requestSeq
				logger.Debug("[DynamicTools] Tool usage updated", "session", sessionID, "tool", toolName, "seq", info.requestSeq)
				return
			}
		}
	}
}

// ageDynamicToolsForSession 老化检查：移除长时间未使用的动态工具
func (a *Agent) ageDynamicToolsForSession(sessionID string) {
	a.sessionDynamicToolsMu.Lock()
	defer a.sessionDynamicToolsMu.Unlock()

	info, exists := a.sessionDynamicTools[sessionID]
	if !exists || len(info.tools) == 0 {
		return
	}

	currentSeq := info.requestSeq
	var newTools []map[string]interface{}
	removedCount := 0

	for _, t := range info.tools {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				lastUsed := info.lastUsed[name]
				unusedCount := currentSeq - lastUsed
				if unusedCount > dynamicToolAgingThreshold {
					logger.Info("[DynamicTools] Tool aged out", "session", sessionID, "tool", name, "unused_requests", unusedCount)
					delete(info.lastUsed, name)
					removedCount++
					continue
				}
			}
		}
		newTools = append(newTools, t)
	}

	if removedCount > 0 {
		info.tools = newTools
		logger.Info("[DynamicTools] Session tools after aging", "session", sessionID, "removed", removedCount, "remaining", len(info.tools))
	}
}

// getToolDefinitionsWithDynamic 获取指定会话的默认工具 + 动态工具
func (a *Agent) getToolDefinitionsWithDynamic(sessionID string) []map[string]interface{} {
	// 获取默认工具
	defaultTools := a.toolRegistry.GetToolDefinitions()

	a.sessionDynamicToolsMu.RLock()
	info, exists := a.sessionDynamicTools[sessionID]
	a.sessionDynamicToolsMu.RUnlock()

	// 如果没有动态工具，直接返回默认工具
	if !exists || len(info.tools) == 0 {
		return defaultTools
	}

	// 合并工具（去重：以工具名称为准）
	toolMap := make(map[string]map[string]interface{})
	for _, t := range defaultTools {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				toolMap[name] = t
			}
		}
	}
	for _, t := range info.tools {
		if fn, ok := t["function"].(map[string]interface{}); ok {
			if name, ok := fn["name"].(string); ok {
				toolMap[name] = t
			}
		}
	}

	// 转换为列表
	result := make([]map[string]interface{}, 0, len(toolMap))
	for _, t := range toolMap {
		result = append(result, t)
	}

	logger.Info("[DynamicTools] Tool definitions for session", "session", sessionID, "default", len(defaultTools), "dynamic", len(info.tools), "total", len(result))
	return result
}

// GetSkillInstruction 获取技能指令
func (a *Agent) GetSkillInstruction(name string) (string, error) {
	if a.skillsMgr == nil {
		return "", fmt.Errorf("skills manager not initialized")
	}
	return a.skillsMgr.GetSkillInstruction(name)
}

// ListSkills 列出可用技能
func (a *Agent) ListSkills() ([]*skills.Skill, error) {
	if a.skillsMgr == nil {
		return nil, fmt.Errorf("skills manager not initialized")
	}
	return a.skillsMgr.ListSkills()
}

// ProcessMessage 处理消息
func (a *Agent) ProcessMessage(ctx context.Context, sessionID, userMessage string) (string, error) {
	return a.ProcessMessageWithMedia(ctx, sessionID, userMessage, nil)
}

// ProcessMessageWithMedia 处理带媒体的消息
func (a *Agent) ProcessMessageWithMedia(ctx context.Context, sessionID, userMessage string, mediaPaths []string) (string, error) {
	// 开始追踪
	var tr *trace.Trace
	if a.traceCollector != nil {
		traceType := trace.TraceTypeChat
		if len(mediaPaths) > 0 {
			traceType = trace.TraceTypeStream
		}
		// 截取用户消息前 50 字符作为名称
		name := userMessage
		if len(name) > 50 {
			name = name[:50] + "..."
		}
		tr, ctx = a.traceCollector.StartTrace(ctx, sessionID, traceType, name, userMessage)
	}

	// 追踪结束处理
	defer func() {
		if tr != nil && a.traceCollector != nil {
			a.traceCollector.EndTrace(tr, "", nil)
		}
	}()

	// 增加会话请求序号（用于动态工具老化机制）
	a.incrementSessionRequestSeq(sessionID)
	// 老化检查：移除长时间未使用的动态工具
	a.ageDynamicToolsForSession(sessionID)

	// 1. 获取或创建会话
	s := a.sessions.GetOrCreate(sessionID)

	// 尝试锁定会话，如果锁被持有超过配置的时间则强制释放
	// 这可以处理长时间运行操作（如视频生成）后的会话阻塞问题
	sessionLockTimeout := a.getSessionLockTimeout()
	if !s.TryLockWithTimeout(sessionLockTimeout) {
		logger.Warn("Session is busy, please wait", "session", sessionID)
		return "⏳ 正在处理上一条消息，请稍后再试...", nil
	}
	defer s.UnlockAfterProcessing()

	// 检查是否有多模态内容
	hasMedia := len(mediaPaths) > 0
	if hasMedia && a.multimodalProvider.SupportsVision() {
		// 使用多模态消息格式
		a.sessions.AddMessageWithMediaAndPersist(sessionID, "user", userMessage, mediaPaths)
		logger.Info("Processing multimodal message", "session", sessionID, "mediaCount", len(mediaPaths), "provider", a.multimodalProvider.Name())
	} else {
		a.sessions.AddMessageWithPersist(sessionID, "user", userMessage)
		if hasMedia {
			logger.Warn("Multimodal provider does not support vision, falling back to text mode", "session", sessionID)
		}
		hasMedia = false // 如果 provider 不支持视觉，退化为文本模式
	}

	// 2. 构建上下文
	var ctxSpan *trace.Span
	var ctxSpanCtx context.Context
	if tr != nil && a.traceCollector != nil {
		ctxSpan, ctxSpanCtx = a.traceCollector.StartContextSpan(ctx, tr.ID, "session: "+sessionID)
		ctx = ctxSpanCtx
	}
	messages, err := a.buildContextWithMedia(sessionID, hasMedia)
	if ctxSpan != nil && a.traceCollector != nil {
		a.traceCollector.EndContextSpan(ctxSpan, fmt.Sprintf("messages: %d", len(messages)), err)
	}
	if err != nil {
		return "", fmt.Errorf("failed to build context: %w", err)
	}

	// 3. 选择 provider：多模态消息使用 multimodalProvider
	provider := a.provider
	if hasMedia {
		provider = a.multimodalProvider
		logger.Info("Using multimodal provider", "provider", provider.Name(), "model", provider.Model())
	}

	// 4. 执行代理循环
	result, err := a.runLoopWithProvider(ctx, sessionID, messages, provider)

	// 更新追踪结果
	if tr != nil && a.traceCollector != nil {
		if err != nil {
			a.traceCollector.EndTrace(tr, "", err)
		} else {
			a.traceCollector.EndTrace(tr, result, nil)
		}
		tr = nil // 防止 defer 再次结束
	}

	return result, err
}

// ProcessMessageStream 流式处理消息
func (a *Agent) ProcessMessageStream(ctx context.Context, sessionID, userMessage string, callback stream.StreamCallback) error {
	return a.ProcessMessageStreamWithMedia(ctx, sessionID, userMessage, nil, callback)
}

// ProcessMessageStreamWithMedia 流式处理带媒体的消息
func (a *Agent) ProcessMessageStreamWithMedia(ctx context.Context, sessionID, userMessage string, mediaPaths []string, callback stream.StreamCallback) error {
	// 开始追踪
	var tr *trace.Trace
	if a.traceCollector != nil {
		traceType := trace.TraceTypeStream
		name := userMessage
		if len(name) > 50 {
			name = name[:50] + "..."
		}
		tr, ctx = a.traceCollector.StartTrace(ctx, sessionID, traceType, name, userMessage)
	}

	// 追踪结束处理
	defer func() {
		if tr != nil && a.traceCollector != nil {
			a.traceCollector.EndTrace(tr, "", nil)
		}
	}()

	// 增加会话请求序号（用于动态工具老化机制）
	a.incrementSessionRequestSeq(sessionID)
	// 老化检查：移除长时间未使用的动态工具
	a.ageDynamicToolsForSession(sessionID)

	// 1. 获取或创建会话
	s := a.sessions.GetOrCreate(sessionID)

	// 尝试锁定会话，如果锁被持有超过配置的时间则强制释放
	sessionLockTimeout := a.getSessionLockTimeout()
	if !s.TryLockWithTimeout(sessionLockTimeout) {
		logger.Warn("Session is busy, please wait", "session", sessionID)
		// 发送友好提示给用户
		callback(stream.NewTextEvent("⏳ 正在处理上一条消息，请稍后再试..."))
		callback(stream.NewDoneEvent())
		return ErrSessionBusy
	}
	defer s.UnlockAfterProcessing()

	// 检查是否有多模态内容
	hasMedia := len(mediaPaths) > 0
	if hasMedia && a.multimodalProvider.SupportsVision() {
		a.sessions.AddMessageWithMediaAndPersist(sessionID, "user", userMessage, mediaPaths)
		logger.Info("Processing multimodal message (stream)", "session", sessionID, "mediaCount", len(mediaPaths), "provider", a.multimodalProvider.Name())
	} else {
		a.sessions.AddMessageWithPersist(sessionID, "user", userMessage)
		if hasMedia {
			logger.Warn("Multimodal provider does not support vision, falling back to text mode", "session", sessionID)
		}
		hasMedia = false // 如果 provider 不支持视觉，退化为文本模式
	}

	// 2. 构建上下文
	var ctxSpan *trace.Span
	var ctxSpanCtx context.Context
	if tr != nil && a.traceCollector != nil {
		ctxSpan, ctxSpanCtx = a.traceCollector.StartContextSpan(ctx, tr.ID, "session: "+sessionID)
		ctx = ctxSpanCtx
	}
	messages, err := a.buildContextWithMedia(sessionID, hasMedia)
	if ctxSpan != nil && a.traceCollector != nil {
		a.traceCollector.EndContextSpan(ctxSpan, fmt.Sprintf("messages: %d", len(messages)), err)
	}
	if err != nil {
		callback(stream.NewErrorEvent(fmt.Errorf("failed to build context: %w", err)))
		return err
	}

	// 3. 选择 provider：多模态消息使用 multimodalProvider
	provider := a.provider
	if hasMedia {
		provider = a.multimodalProvider
		logger.Info("Using multimodal provider (stream)", "provider", provider.Name(), "model", provider.Model())
	}

	// 4. 执行流式代理循环
	runErr := a.runLoopStreamWithProvider(ctx, sessionID, messages, provider, callback)

	// 更新追踪结果
	if tr != nil && a.traceCollector != nil {
		a.traceCollector.EndTrace(tr, "", runErr)
		tr = nil // 防止 defer 再次结束
	}

	// 如果执行失败（如 LLM 请求超时或报错），通过 callback 通知 channel，
	// 确保 gateway 收到响应，表明通讯通道正常工作。
	if runErr != nil {
		callback(stream.NewErrorEvent(runErr))
	}

	return runErr
}

// runLoop 代理执行循环
func (a *Agent) runLoop(ctx context.Context, sessionID string, messages []llm.Message) (string, error) {
	return a.runLoopWithProvider(ctx, sessionID, messages, a.provider)
}

// runLoopWithProvider 代理执行循环（指定 provider）
func (a *Agent) runLoopWithProvider(ctx context.Context, sessionID string, messages []llm.Message, provider providers.Provider) (string, error) {
	// 设置 steer 状态为执行中
	if a.steerMgr != nil {
		a.steerMgr.setExecuting(sessionID, true)
		defer a.steerMgr.setExecuting(sessionID, false)
	}

	iterations := 0
	maxIterations := a.config.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}

	// 确保在函数结束时执行自动捕获和历史记录
	defer func() {
		if a.config.MemoryConfig != nil && a.config.MemoryConfig.AutoCapture {
			go a.captureMemories(sessionID, messages)
		}
	}()

	for iterations < maxIterations {
		iterations++

		// 记录迭代进度
		logger.Info("Agent iteration", "session", sessionID, "iteration", iterations, "maxIterations", maxIterations)

		// 检查是否有注入的消息（Steer 模式）
		if a.steerMgr != nil {
			if injected := a.steerMgr.checkInjection(sessionID); injected != nil {
				// 将注入的消息添加到对话历史
				injectedMsg := llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("【补充说明】\n%s", injected.Content),
				}
				messages = append(messages, injectedMsg)
				logger.Info("Injected message added to conversation", "session", sessionID, "content", injected.Content[:min(50, len(injected.Content))])
			}
		}

		// 构建 LLM 请求
		req := &llm.Request{
			Model:    provider.Model(),
			Messages: messages,
		}

		// 只有支持工具的 provider 才发送工具定义
		if provider.SupportsTools() {
			req.Tools = a.getToolDefinitionsWithDynamic(sessionID)
		}

		// 开始 LLM Span
		var llmSpan *trace.Span
		if a.traceCollector != nil {
			if tr := trace.GetTrace(ctx); tr != nil {
				// 构建完整的输入信息（包含消息和工具定义）
				inputData := map[string]interface{}{
					"messages": messages,
				}
				if req.Tools != nil && len(req.Tools) > 0 {
					inputData["tools"] = req.Tools
				}
				inputJSON, _ := json.MarshalIndent(inputData, "", "  ")
				llmSpan, ctx = a.traceCollector.StartLLMSpan(ctx, tr.ID, provider.Name(), provider.Model(), string(inputJSON))
			}
		}

		// 调用 LLM
		resp, err := provider.Complete(ctx, req)

		// 结束 LLM Span
		if llmSpan != nil && a.traceCollector != nil {
			var inputTokens, outputTokens int
			var outputStr string
			if resp != nil {
				inputTokens = resp.Usage.PromptTokens
				outputTokens = resp.Usage.CompletionTokens
				// 记录完整输出（包含工具调用信息）
				outputData := map[string]interface{}{
					"content": resp.GetContent(),
				}
				if resp.HasToolCalls() {
					outputData["toolCalls"] = resp.GetToolCalls()
				}
				if outputJSON, err := json.MarshalIndent(outputData, "", "  "); err == nil {
					outputStr = string(outputJSON)
				} else {
					outputStr = resp.GetContent()
				}
			}
			a.traceCollector.EndLLMSpan(llmSpan, outputStr, inputTokens, outputTokens, err)
		}

		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// 获取响应消息
		assistantMsg := resp.ToMessage()

		// 存储助手消息到会话（只有内容不为空时才保存）
		if assistantMsg.Content != "" {
			a.sessions.AddMessageWithPersist(sessionID, "assistant", assistantMsg.Content)
		}

		// 检查是否有工具调用
		if !resp.HasToolCalls() {
			// 在结束前检查是否有注入的消息
			// 如果有，继续循环处理这些消息
			if a.steerMgr != nil {
				if injected := a.steerMgr.checkInjection(sessionID); injected != nil {
					// 将注入的消息添加到对话历史
					injectedMsg := llm.Message{
						Role:    "user",
						Content: fmt.Sprintf("【补充说明】\n%s", injected.Content),
					}
					messages = append(messages, assistantMsg, injectedMsg)
					logger.Info("Injected message retrieved before loop end (non-stream), continuing", "session", sessionID, "content", injected.Content[:min(50, len(injected.Content))])
					continue // 继续循环，处理注入的消息
				}
			}
			return resp.GetContent(), nil
		}

		// 添加助手消息到历史
		messages = append(messages, assistantMsg)

		// 执行工具调用
		for _, tc := range resp.GetToolCalls() {
			result, err := a.executeTool(ctx, sessionID, &tc)

			var resultStr string
			if err != nil {
				resultStr = fmt.Sprintf("Error: %s", err)
			} else {
				resultStr = result
			}

			// 添加工具结果到消息
			toolMsg := llm.Message{
				Role:       "tool",
				Content:    resultStr,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolMsg)
		}

		// 添加反思提示（参考 nanobot）
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: ReflectPrompt,
		})
	}

	return "", fmt.Errorf("max iterations reached")
}

// runLoopStream 流式代理执行循环
func (a *Agent) runLoopStream(ctx context.Context, sessionID string, messages []llm.Message, callback stream.StreamCallback) error {
	return a.runLoopStreamWithProvider(ctx, sessionID, messages, a.provider, callback)
}

// runLoopStreamWithProvider 流式代理执行循环（指定 provider）
func (a *Agent) runLoopStreamWithProvider(ctx context.Context, sessionID string, messages []llm.Message, provider providers.Provider, callback stream.StreamCallback) error {
	// 设置 steer 状态为执行中
	if a.steerMgr != nil {
		a.steerMgr.setExecuting(sessionID, true)
		defer a.steerMgr.setExecuting(sessionID, false)
	}

	iterations := 0
	maxIterations := a.config.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}

	// 如果 provider 不支持工具，最多只执行 1 次迭代（不执行工具调用循环）
	if !provider.SupportsTools() {
		maxIterations = 1
	}

	// 确保在函数结束时执行自动捕获
	defer func() {
		if a.config.MemoryConfig != nil && a.config.MemoryConfig.AutoCapture {
			go a.captureMemories(sessionID, messages)
		}
	}()

	for iterations < maxIterations {
		iterations++

		// 记录迭代进度
		logger.Info("Agent iteration (stream)", "session", sessionID, "iteration", iterations, "maxIterations", maxIterations)

		// 检查是否有注入的消息（Steer 模式）
		if a.steerMgr != nil {
			if injected := a.steerMgr.checkInjection(sessionID); injected != nil {
				// 将注入的消息添加到对话历史
				injectedMsg := llm.Message{
					Role:    "user",
					Content: fmt.Sprintf("【补充说明】\n%s", injected.Content),
				}
				messages = append(messages, injectedMsg)
				logger.Info("Injected message added to conversation (stream)", "session", sessionID, "content", injected.Content[:min(50, len(injected.Content))])
			}
		}

		// 构建 LLM 请求
		req := &llm.Request{
			Model:    provider.Model(),
			Messages: messages,
			Stream:   true,
		}

		// 只有支持工具的 provider 才发送工具定义
		if provider.SupportsTools() {
			req.Tools = a.getToolDefinitionsWithDynamic(sessionID)
		}

		// 开始 LLM Span
		var llmSpan *trace.Span
		if a.traceCollector != nil {
			if tr := trace.GetTrace(ctx); tr != nil {
				// 构建完整的输入信息（包含消息和工具定义）
				inputData := map[string]interface{}{
					"messages": messages,
				}
				if req.Tools != nil && len(req.Tools) > 0 {
					inputData["tools"] = req.Tools
				}
				inputJSON, _ := json.MarshalIndent(inputData, "", "  ")
				llmSpan, ctx = a.traceCollector.StartLLMSpan(ctx, tr.ID, provider.Name(), provider.Model(), string(inputJSON))
			}
		}

		// 调用 LLM 流式接口
		eventChan, err := provider.Stream(ctx, req)
		if err != nil {
			if llmSpan != nil && a.traceCollector != nil {
				a.traceCollector.EndLLMSpan(llmSpan, "", 0, 0, err)
			}
			return fmt.Errorf("LLM stream call failed: %w", err)
		}

		// 累积响应内容
		var contentBuilder strings.Builder
		// 用于累积流式工具调用（按 index 组织）
		toolCallsAccumulator := make(map[int]*llm.ToolCall)
		// 累积 token 使用（如果可用）
		var totalInputTokens, totalOutputTokens int

		// 处理流式事件
		for event := range eventChan {
			// 提取 usage 信息（某些 API 在流式结束时会返回）
			if event.Usage != nil {
				totalInputTokens = event.Usage.PromptTokens
				totalOutputTokens = event.Usage.CompletionTokens
			}

			if len(event.Choices) == 0 {
				continue
			}

			choice := event.Choices[0]
			delta := choice.Delta

			// 累积文本内容
			if delta.Content != "" {
				contentBuilder.WriteString(delta.Content)
				callback(stream.NewTextEvent(delta.Content))
			}

			// 累积工具调用
			for _, dtc := range delta.ToolCalls {
				idx := dtc.Index
				if existing, ok := toolCallsAccumulator[idx]; ok {
					// 累积参数（字符串拼接）
					existing.Function.Arguments = append(existing.Function.Arguments, []byte(dtc.Function.Arguments)...)
				} else {
					// 新工具调用
					toolCallsAccumulator[idx] = &llm.ToolCall{
						ID:   dtc.ID,
						Type: dtc.Type,
						Function: llm.FunctionCall{
							Name:      dtc.Function.Name,
							Arguments: json.RawMessage(dtc.Function.Arguments),
						},
					}
				}
			}
		}

		// 将累积的工具调用转换为数组（按 index 排序）
		// 注意：index 可能不从 0 开始（例如 thinking block 是 0，tool_use 是 1）
		var toolCalls []llm.ToolCall
		maxIndex := 0
		for idx := range toolCallsAccumulator {
			if idx > maxIndex {
				maxIndex = idx
			}
		}
		for i := 0; i <= maxIndex; i++ {
			if tc, ok := toolCallsAccumulator[i]; ok {
				toolCalls = append(toolCalls, *tc)
			}
		}

		// 构建助手消息
		assistantContent := contentBuilder.String()

		// 结束 LLM Span
		if llmSpan != nil && a.traceCollector != nil {
			// 构建完整输出（包含工具调用信息）
			outputData := map[string]interface{}{
				"content": assistantContent,
			}
			if len(toolCalls) > 0 {
				outputData["toolCalls"] = toolCalls
			}
			var outputStr string
			if outputJSON, err := json.MarshalIndent(outputData, "", "  "); err == nil {
				outputStr = string(outputJSON)
			} else {
				outputStr = assistantContent
			}
			a.traceCollector.EndLLMSpan(llmSpan, outputStr, totalInputTokens, totalOutputTokens, nil)
		}

		assistantMsg := llm.Message{
			Role:      "assistant",
			Content:   assistantContent,
			ToolCalls: toolCalls,
		}

		// 存储助手消息到会话（只有内容不为空时才保存）
		if assistantContent != "" {
			a.sessions.AddMessageWithPersist(sessionID, "assistant", assistantContent)
		}

		// 检查是否有工具调用
		if len(toolCalls) == 0 {
			// 在结束前检查是否有注入的消息
			// 如果有，继续循环处理这些消息
			if a.steerMgr != nil {
				if injected := a.steerMgr.checkInjection(sessionID); injected != nil {
					// 将注入的消息添加到对话历史
					injectedMsg := llm.Message{
						Role:    "user",
						Content: fmt.Sprintf("【补充说明】\n%s", injected.Content),
					}
					messages = append(messages, assistantMsg, injectedMsg)
					logger.Info("Injected message retrieved before loop end, continuing", "session", sessionID, "content", injected.Content[:min(50, len(injected.Content))])
					continue // 继续循环，处理注入的消息
				}
			}
			callback(stream.NewDoneEvent())
			return nil
		}

		// 添加助手消息到历史
		messages = append(messages, assistantMsg)

		// 执行工具调用
		for _, tc := range toolCalls {
			// 发送工具开始事件
			callback(stream.NewToolStartEvent(tc.Function.Name))

			// 执行工具
			result, err := a.executeTool(ctx, sessionID, &tc)

			// 发送工具结束事件
			callback(stream.NewToolEndEvent(tc.Function.Name, result, err))

			var resultStr string
			if err != nil {
				resultStr = fmt.Sprintf("Error: %s", err)
			} else {
				resultStr = result
			}

			// 添加工具结果到消息
			toolMsg := llm.Message{
				Role:       "tool",
				Content:    resultStr,
				ToolCallID: tc.ID,
			}
			messages = append(messages, toolMsg)
		}

		// 添加反思提示（参考 nanobot）
		messages = append(messages, llm.Message{
			Role:    "user",
			Content: ReflectPrompt,
		})
	}

	return fmt.Errorf("max iterations reached")
}

// executeTool 执行工具
func (a *Agent) executeTool(ctx context.Context, sessionID string, tc *llm.ToolCall) (string, error) {
	start := time.Now()

	// 开始 Tool Span
	var toolSpan *trace.Span
	if a.traceCollector != nil {
		if tr := trace.GetTrace(ctx); tr != nil {
			toolSpan, ctx = a.traceCollector.StartToolSpan(ctx, tr.ID, tc.Function.Name, string(tc.Function.Arguments))
		}
	}

	tool, exists := a.toolRegistry.Get(tc.Function.Name)
	if !exists {
		if toolSpan != nil && a.traceCollector != nil {
			a.traceCollector.EndToolSpan(toolSpan, "", fmt.Errorf("unknown tool: %s", tc.Function.Name))
		}
		return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
	}

	result, err := tool.Execute(ctx, tc.Function.Arguments)
	duration := time.Since(start)

	// 结束 Tool Span
	if toolSpan != nil && a.traceCollector != nil {
		// 记录完整结果
		a.traceCollector.EndToolSpan(toolSpan, result, err)
	}

	// 记录工具调用
	logger.ToolCall(tc.Function.Name, tc.Function.Arguments, result, duration, err)

	// 标记动态工具被使用（用于老化机制）
	a.markDynamicToolUsed(sessionID, tc.Function.Name)

	// 如果是 skill 工具，解析返回结果并注入动态工具
	if tc.Function.Name == "skill" && err == nil {
		a.parseAndInjectSkillTools(sessionID, result)
	}

	return result, err
}

// parseAndInjectSkillTools 解析 skill 工具返回的工具定义并注入到指定会话
func (a *Agent) parseAndInjectSkillTools(sessionID, result string) {
	var response struct {
		Content string                   `json:"content"`
		Tools   []map[string]interface{} `json:"tools,omitempty"`
	}

	if err := json.Unmarshal([]byte(result), &response); err != nil {
		logger.Warn("[DynamicTools] Failed to parse skill response", "error", err)
		return
	}

	if len(response.Tools) > 0 {
		logger.Info("[DynamicTools] Injecting tools from skill", "session", sessionID, "count", len(response.Tools))
		for _, t := range response.Tools {
			if fn, ok := t["function"].(map[string]interface{}); ok {
				if name, ok := fn["name"].(string); ok {
					logger.Info("[DynamicTools] Tool injected", "session", sessionID, "tool", name)
				}
			}
		}
		a.addDynamicToolsForSession(sessionID, response.Tools)
	}
}

func generateID() string {
	return uuid.New().String()[:8]
}

// InjectMessage 实现 StreamInjector 接口
// 向正在执行的对话注入消息（Steer 模式）
func (a *Agent) InjectMessage(sessionID, content string, media []string) bool {
	if a.steerMgr == nil {
		return false
	}
	return a.steerMgr.Inject(sessionID, content, media)
}

// IsExecuting 实现 StreamInjector 接口
// 检查指定会话是否正在执行
func (a *Agent) IsExecuting(sessionID string) bool {
	if a.steerMgr == nil {
		return false
	}
	return a.steerMgr.IsExecuting(sessionID)
}

// DrainInjectionChannel 实现 StreamInjector 接口
// 清空注入通道中的消息并返回，用于取回未被处理的消息
func (a *Agent) DrainInjectionChannel(sessionID string) []session.InjectionMessage {
	if a.steerMgr == nil {
		return nil
	}
	// 调用 steerManager 的 DrainInjectionChannel 方法
	internalMsgs := a.steerMgr.DrainInjectionChannel(sessionID)
	if len(internalMsgs) == 0 {
		return nil
	}
	// 转换为 session.InjectionMessage 类型
	result := make([]session.InjectionMessage, 0, len(internalMsgs))
	for _, msg := range internalMsgs {
		result = append(result, session.InjectionMessage{
			Content: msg.Content,
			Media:   msg.Media,
		})
	}
	return result
}
