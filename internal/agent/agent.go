// Package agent 核心代理逻辑
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/session"
	"github.com/lingguard/internal/skills"
	"github.com/lingguard/internal/subagent"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/llm"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/memory"
	"github.com/lingguard/pkg/stream"
)

// ReflectPrompt 反思提示（参考 nanobot）
const ReflectPrompt = "Reflect on the results and decide next steps."

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
	memoryBuilder      *memory.ContextBuilder // 记忆上下文构建器
}

// NewAgent 创建新代理
func NewAgent(cfg *config.AgentsConfig, provider providers.Provider, skillsLoader *skills.Loader) *Agent {
	return NewAgentWithMultimodal(cfg, provider, nil, skillsLoader)
}

// NewAgentWithMultimodal 创建带多模态支持的代理
func NewAgentWithMultimodal(cfg *config.AgentsConfig, provider providers.Provider, multimodalProvider providers.Provider, skillsLoader *skills.Loader) *Agent {
	var skillsMgr *skills.Manager
	if skillsLoader != nil {
		skillsMgr = skills.NewManager(skillsLoader)
	}

	toolRegistry := tools.NewRegistry()

	// 初始化文件持久化存储（参考 nanobot）
	// 记忆目录固定在 ~/.lingguard/memory/
	var memStore *memory.FileStore
	var memBuilder *memory.ContextBuilder
	var sessionStore memory.Store

	if cfg.MemoryConfig != nil && cfg.MemoryConfig.Enabled {
		// 使用文件存储，目录固定为 ~/.lingguard/memory
		home, _ := os.UserHomeDir()
		memDir := filepath.Join(home, ".lingguard", "memory")

		memStore = memory.NewFileStore(memDir)
		if err := memStore.Init(); err != nil {
			logger.Warn("Failed to init file memory store, using in-memory", "error", err)
			sessionStore = memory.NewMemoryStore()
		} else {
			sessionStore = memStore
			memBuilder = memory.NewContextBuilder(memStore)
			logger.Info("File-based memory store initialized", "path", memDir)
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
		memoryBuilder:      memBuilder,
	}

	// 初始化子代理管理器
	agent.subagentMgr = subagent.NewSubagentManager(provider, toolRegistry, nil)

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
	if a.memoryStore != nil {
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

// SubagentManager 返回子代理管理器
func (a *Agent) SubagentManager() *subagent.SubagentManager {
	return a.subagentMgr
}

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
	// 1. 获取或创建会话并添加用户消息
	s := a.sessions.GetOrCreate(sessionID)

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
	messages, err := a.buildContextWithMedia(sessionID, hasMedia)
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
	return a.runLoopWithProvider(ctx, sessionID, messages, provider)
}

// ProcessMessageStream 流式处理消息
func (a *Agent) ProcessMessageStream(ctx context.Context, sessionID, userMessage string, callback stream.StreamCallback) error {
	return a.ProcessMessageStreamWithMedia(ctx, sessionID, userMessage, nil, callback)
}

// ProcessMessageStreamWithMedia 流式处理带媒体的消息
func (a *Agent) ProcessMessageStreamWithMedia(ctx context.Context, sessionID, userMessage string, mediaPaths []string, callback stream.StreamCallback) error {
	// 1. 获取或创建会话并添加用户消息
	s := a.sessions.GetOrCreate(sessionID)

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
	messages, err := a.buildContextWithMedia(sessionID, hasMedia)
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
	return a.runLoopStreamWithProvider(ctx, sessionID, messages, provider, callback)
}

// buildContext 构建上下文
func (a *Agent) buildContext(sessionID string) ([]llm.Message, error) {
	return a.buildContextWithMedia(sessionID, false)
}

// buildContextWithMedia 构建上下文（支持多模态）
// hasMedia 表示当前消息是否包含媒体，用于决定是否为最后一条用户消息构建多模态内容
func (a *Agent) buildContextWithMedia(sessionID string, hasMedia bool) ([]llm.Message, error) {
	messages := make([]llm.Message, 0)
	historyLen := 0

	// 构建系统提示
	systemPrompt := a.config.SystemPrompt

	// 添加当前时间信息（让 LLM 能准确计算相对时间）
	currentTime := time.Now().Format("2006-01-02 15:04:05 Monday")
	timeInfo := fmt.Sprintf("当前时间: %s", currentTime)
	if systemPrompt != "" {
		systemPrompt = systemPrompt + "\n\n" + timeInfo
	} else {
		systemPrompt = timeInfo
	}

	// 添加工作目录信息
	if a.config.Workspace != "" {
		workspaceInfo := fmt.Sprintf("工作目录: %s\n\n重要规则:\n- 所有文件操作都应该相对于工作目录进行\n- git clone 时先 cd %s\n- 下载的代码应该放在工作目录下", a.config.Workspace, a.config.Workspace)
		if systemPrompt != "" {
			systemPrompt = systemPrompt + "\n\n" + workspaceInfo
		} else {
			systemPrompt = workspaceInfo
		}
	}

	// 添加记忆上下文（参考 nanobot）
	if a.memoryBuilder != nil {
		recentDays := 3
		if a.config.MemoryConfig != nil && a.config.MemoryConfig.RecentDays > 0 {
			recentDays = a.config.MemoryConfig.RecentDays
		}
		memContext, err := a.memoryBuilder.BuildContext(recentDays)
		if err == nil && memContext != "" {
			if systemPrompt != "" {
				systemPrompt = systemPrompt + "\n\n" + memContext
			} else {
				systemPrompt = memContext
			}
		}
	}

	// 添加技能上下文
	if a.skillsMgr != nil {
		skillsContext := a.skillsMgr.GetSkillsContext()
		if skillsContext != "" {
			if systemPrompt != "" {
				systemPrompt = systemPrompt + "\n\n" + skillsContext
			} else {
				systemPrompt = skillsContext
			}
		}
	}

	// 添加系统提示
	if systemPrompt != "" {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// 获取会话历史消息（使用 MemoryWindow）
	s := a.sessions.GetOrCreate(sessionID)
	history := s.GetHistory(a.config.MemoryWindow)
	historyLen = len(history)

	for i, msg := range history {
		llmMsg := llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}

		// 如果这是最后一条用户消息且有多模态内容，构建多模态消息
		if hasMedia && i == historyLen-1 && msg.Role == "user" && len(msg.Media) > 0 {
			contentParts, err := a.buildMultimodalContent(msg.Content, msg.Media)
			if err != nil {
				logger.Warn("Failed to build multimodal content", "error", err)
			} else {
				llmMsg.ContentParts = contentParts
				llmMsg.Content = "" // 清空 Content，使用 ContentParts
			}
		}

		messages = append(messages, llmMsg)
	}

	return messages, nil
}

// buildMultimodalContent 构建多模态内容
func (a *Agent) buildMultimodalContent(text string, mediaPaths []string) ([]llm.ContentPart, error) {
	parts := make([]llm.ContentPart, 0)

	// 添加图片/视频
	for _, path := range mediaPaths {
		// 读取媒体文件并转换为 base64
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read media %s: %w", path, err)
		}

		ext := strings.ToLower(filepath.Ext(path))

		// 如果是视频文件，使用 video_url 格式（Qwen-Omni 模型）
		if isVideoFile(ext) {
			mimeType := detectVideoMimeType(ext)
			base64Data := encodeBase64(data)
			logger.Info("Processing video file for multimodal", "path", path, "mimeType", mimeType, "size", len(data))

			// 使用 video_url 格式（支持 Qwen-Omni 模型）
			parts = append(parts, llm.ContentPart{
				Type: "video_url",
				VideoURL: &llm.VideoURL{
					URL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data),
				},
			})
		} else {
			// 图片使用 image_url 格式
			mimeType := detectMimeType(data)
			base64Data := encodeBase64(data)

			parts = append(parts, llm.ContentPart{
				Type: "image_url",
				ImageURL: &llm.ImageURL{
					URL:    fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data),
					Detail: "auto",
				},
			})
		}
	}

	// 添加文本（放在最后）
	if text != "" {
		parts = append(parts, llm.ContentPart{
			Type: "text",
			Text: text,
		})
	}

	return parts, nil
}

// isVideoFile 检查文件扩展名是否为视频格式
func isVideoFile(ext string) bool {
	videoExts := map[string]bool{
		".mp4":  true,
		".mov":  true,
		".avi":  true,
		".mkv":  true,
		".webm": true,
		".flv":  true,
		".wmv":  true,
		".m4v":  true,
		".3gp":  true,
	}
	return videoExts[ext]
}

// detectVideoMimeType 根据扩展名检测视频 MIME 类型
func detectVideoMimeType(ext string) string {
	videoMimes := map[string]string{
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",
		".webm": "video/webm",
		".flv":  "video/x-flv",
		".wmv":  "video/x-ms-wmv",
		".m4v":  "video/mp4",
		".3gp":  "video/3gpp",
	}
	if mime, ok := videoMimes[ext]; ok {
		return mime
	}
	return "video/mp4"
}

// detectMimeType 检测图片 MIME 类型
func detectMimeType(data []byte) string {
	if len(data) < 8 {
		return "image/jpeg"
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}
	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	// GIF: 47 49 46 38
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return "image/gif"
	}
	// WebP: 52 49 46 46 ... 57 45 42 50
	if data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 {
		if len(data) > 11 && data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
			return "image/webp"
		}
	}

	return "image/jpeg"
}

// encodeBase64 编码为 base64
func encodeBase64(data []byte) string {
	return encodeBase64String(data)
}

// encodeBase64String 编码为 base64 字符串
func encodeBase64String(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

	result := make([]byte, 0, (len(data)+2)/3*4)

	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i

		if remaining >= 3 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				base64Chars[n>>6&0x3F],
				base64Chars[n&0x3F],
			)
		} else if remaining == 2 {
			n = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				base64Chars[n>>6&0x3F],
				'=',
			)
		} else {
			n = uint32(data[i]) << 16
			result = append(result,
				base64Chars[n>>18&0x3F],
				base64Chars[n>>12&0x3F],
				'=',
				'=',
			)
		}
	}

	return string(result)
}

// runLoop 代理执行循环
func (a *Agent) runLoop(ctx context.Context, sessionID string, messages []llm.Message) (string, error) {
	return a.runLoopWithProvider(ctx, sessionID, messages, a.provider)
}

// runLoopWithProvider 代理执行循环（指定 provider）
func (a *Agent) runLoopWithProvider(ctx context.Context, sessionID string, messages []llm.Message, provider providers.Provider) (string, error) {
	iterations := 0
	maxIterations := a.config.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}

	for iterations < maxIterations {
		iterations++

		// 构建 LLM 请求
		req := &llm.Request{
			Model:    provider.Model(),
			Messages: messages,
		}

		// 只有支持工具的 provider 才发送工具定义
		if provider.SupportsTools() {
			req.Tools = a.toolRegistry.GetToolDefinitions()
		}

		// 调用 LLM
		resp, err := provider.Complete(ctx, req)
		if err != nil {
			return "", fmt.Errorf("LLM call failed: %w", err)
		}

		// 获取响应消息
		assistantMsg := resp.ToMessage()

		// 存储助手消息到会话
		s := a.sessions.GetOrCreate(sessionID)
		if assistantMsg.Content != "" || len(assistantMsg.ToolCalls) > 0 {
			s.AddMessage("assistant", assistantMsg.Content)
		}

		// 检查是否有工具调用
		if !resp.HasToolCalls() {
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
	iterations := 0
	maxIterations := a.config.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}

	// 如果 provider 不支持工具，最多只执行 1 次迭代（不执行工具调用循环）
	if !provider.SupportsTools() {
		maxIterations = 1
	}

	for iterations < maxIterations {
		iterations++

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

		// 调用 LLM 流式接口
		eventChan, err := provider.Stream(ctx, req)
		if err != nil {
			return fmt.Errorf("LLM stream call failed: %w", err)
		}

		// 累积响应内容
		var contentBuilder strings.Builder
		// 用于累积流式工具调用（按 index 组织）
		toolCallsAccumulator := make(map[int]*llm.ToolCall)

		// 处理流式事件
		for event := range eventChan {
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
		assistantMsg := llm.Message{
			Role:      "assistant",
			Content:   assistantContent,
			ToolCalls: toolCalls,
		}

		// 存储助手消息到会话
		s := a.sessions.GetOrCreate(sessionID)
		if assistantContent != "" || len(toolCalls) > 0 {
			s.AddMessage("assistant", assistantContent)
		}

		// 检查是否有工具调用
		if len(toolCalls) == 0 {
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

	tool, exists := a.toolRegistry.Get(tc.Function.Name)
	if !exists {
		return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
	}

	result, err := tool.Execute(ctx, tc.Function.Arguments)
	duration := time.Since(start)

	// 记录工具调用
	logger.ToolCall(tc.Function.Name, tc.Function.Arguments, result, duration, err)

	return result, err
}

func generateID() string {
	return uuid.New().String()[:8]
}
