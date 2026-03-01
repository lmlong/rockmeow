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
	taskboard          *taskboard.Service     // 定时任务看板服务（仅用于定时任务跟踪和分析）
	cronWrapper        CronWrapper            // 定时任务服务包装器
	traceCollector     trace.Collector        // 追踪采集器，可选
	sessionLockTimeout time.Duration          // 会话锁超时时间
	steerMgr           *steerManager          // Steer 模式管理器
}

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
	var sessionStore memory.Store

	if cfg.MemoryConfig != nil && cfg.MemoryConfig.Enabled {
		// 使用文件存储，目录固定为 ~/.lingguard/memory
		home, _ := os.UserHomeDir()
		memDir := filepath.Join(home, ".lingguard", "memory")

		// 检查是否启用向量检索
		if cfg.MemoryConfig.Vector != nil && cfg.MemoryConfig.Vector.Enabled {
			// 使用混合存储（文件+向量）
			var providers map[string]config.ProviderConfig
			if fullConfig != nil {
				providers = fullConfig.Providers
			}

			hybridCfg := &memory.HybridStoreConfig{
				MemoryDir:    memDir,
				VectorConfig: cfg.MemoryConfig.Vector,
				Providers:    providers,
			}

			var err error
			hybridStore, err = memory.NewHybridStore(hybridCfg)
			if err != nil {
				logger.Warn("Failed to init hybrid store, falling back to file store", "error", err)
				hybridStore = nil
			} else {
				memStore = hybridStore.FileStore()
				sessionStore = memStore
				memBuilder = memory.NewContextBuilderWithHybrid(hybridStore)
				logger.Info("Hybrid memory store initialized with vector search", "path", memDir)
			}
		}

		// 如果向量存储初始化失败或未启用，使用纯文件存储
		if hybridStore == nil {
			memStore = memory.NewFileStore(memDir)
			if err := memStore.Init(); err != nil {
				logger.Warn("Failed to init file memory store, using in-memory", "error", err)
				sessionStore = memory.NewMemoryStore()
			} else {
				sessionStore = memStore
				memBuilder = memory.NewContextBuilder(memStore)
				logger.Info("File-based memory store initialized", "path", memDir)
			}
		}
	} else {
		// 使用内存存储
		sessionStore = memory.NewMemoryStore()
	}

	// 如果没有配置多模态 provider，使用主 provider
	if multimodalProvider == nil {
		multimodalProvider = provider
	}

	agent := &Agent{
		id:                 generateID(),
		provider:           provider,
		multimodalProvider: multimodalProvider,
		toolRegistry:       toolRegistry,
		sessions:           session.NewManager(sessionStore, cfg.MemoryWindow),
		skillsMgr:          skillsMgr,
		config:             cfg,
		memoryStore:        memStore,
		hybridStore:        hybridStore,
		memoryBuilder:      memBuilder,
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
		a.toolRegistry.Register(tools.NewSkillTool(a.skillsMgr))
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

// RecordEvent 记录事件到历史（参考 nanobot）
func (a *Agent) RecordEvent(eventType, summary string, details map[string]string) error {
	if a.memoryStore == nil {
		return nil
	}
	return a.memoryStore.AddHistory(eventType, summary, details)
}

// GetMemoryStore 获取记忆存储
func (a *Agent) GetMemoryStore() *memory.FileStore {
	return a.memoryStore
}

// GetHybridStore 获取混合存储（如果启用）
func (a *Agent) GetHybridStore() *memory.HybridStore {
	return a.hybridStore
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
		s.AddMessageWithMedia("user", userMessage, mediaPaths)
		logger.Info("Processing multimodal message", "session", sessionID, "mediaCount", len(mediaPaths), "provider", a.multimodalProvider.Name())
	} else {
		s.AddMessage("user", userMessage)
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
		s.AddMessageWithMedia("user", userMessage, mediaPaths)
		logger.Info("Processing multimodal message (stream)", "session", sessionID, "mediaCount", len(mediaPaths), "provider", a.multimodalProvider.Name())
	} else {
		s.AddMessage("user", userMessage)
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

	for iterations < maxIterations {
		iterations++

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
			req.Tools = a.toolRegistry.GetToolDefinitions()
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
		s := a.sessions.GetOrCreate(sessionID)
		if assistantMsg.Content != "" {
			s.AddMessage("assistant", assistantMsg.Content)
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
			result, err := a.executeTool(ctx, &tc)

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
			req.Tools = a.toolRegistry.GetToolDefinitions()
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
		s := a.sessions.GetOrCreate(sessionID)
		if assistantContent != "" {
			s.AddMessage("assistant", assistantContent)
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
			result, err := a.executeTool(ctx, &tc)

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
func (a *Agent) executeTool(ctx context.Context, tc *llm.ToolCall) (string, error) {
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

	return result, err
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
