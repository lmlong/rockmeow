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

// Agent 核心代理结构
type Agent struct {
	id            string
	provider      providers.Provider
	toolRegistry  *tools.Registry
	sessions      *session.Manager
	skillsMgr     *skills.Manager
	subagentMgr   *subagent.SubagentManager
	config        *config.AgentsConfig
	memoryStore   *memory.FileStore      // 文件持久化存储（参考 nanobot）
	memoryBuilder *memory.ContextBuilder // 记忆上下文构建器
}

// NewAgent 创建新代理
func NewAgent(cfg *config.AgentsConfig, provider providers.Provider, skillsLoader *skills.Loader) *Agent {
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
			logger.Warn("Failed to init file memory store: %v, using in-memory", err)
			sessionStore = memory.NewMemoryStore()
		} else {
			sessionStore = memStore
			memBuilder = memory.NewContextBuilder(memStore)
			logger.Info("File-based memory store initialized: %s", memDir)
		}
	} else {
		// 使用内存存储
		sessionStore = memory.NewMemoryStore()
	}

	agent := &Agent{
		id:            generateID(),
		provider:      provider,
		toolRegistry:  toolRegistry,
		sessions:      session.NewManager(sessionStore, cfg.MemoryWindow),
		skillsMgr:     skillsMgr,
		config:        cfg,
		memoryStore:   memStore,
		memoryBuilder: memBuilder,
	}

	// 初始化子代理管理器
	agent.subagentMgr = subagent.NewSubagentManager(provider, toolRegistry, nil)

	return agent
}

// RegisterTool 注册工具
func (a *Agent) RegisterTool(t tools.Tool) {
	a.toolRegistry.Register(t)
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
	// 1. 获取或创建会话并添加用户消息
	s := a.sessions.GetOrCreate(sessionID)
	s.AddMessage("user", userMessage)

	// 2. 构建上下文
	messages, err := a.buildContext(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to build context: %w", err)
	}

	// 3. 执行代理循环
	return a.runLoop(ctx, sessionID, messages)
}

// ProcessMessageStream 流式处理消息
func (a *Agent) ProcessMessageStream(ctx context.Context, sessionID, userMessage string, callback stream.StreamCallback) error {
	// 1. 获取或创建会话并添加用户消息
	s := a.sessions.GetOrCreate(sessionID)
	s.AddMessage("user", userMessage)

	// 2. 构建上下文
	messages, err := a.buildContext(sessionID)
	if err != nil {
		callback(stream.NewErrorEvent(fmt.Errorf("failed to build context: %w", err)))
		return err
	}

	// 3. 执行流式代理循环
	return a.runLoopStream(ctx, sessionID, messages, callback)
}

// buildContext 构建上下文
func (a *Agent) buildContext(sessionID string) ([]llm.Message, error) {
	messages := make([]llm.Message, 0)

	// 构建系统提示
	systemPrompt := a.config.SystemPrompt

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
	for _, msg := range s.GetHistory(a.config.MemoryWindow) {
		messages = append(messages, llm.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return messages, nil
}

// runLoop 代理执行循环
func (a *Agent) runLoop(ctx context.Context, sessionID string, messages []llm.Message) (string, error) {
	iterations := 0
	maxIterations := a.config.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}

	for iterations < maxIterations {
		iterations++

		// 构建 LLM 请求
		req := &llm.Request{
			Model:    a.provider.Model(),
			Messages: messages,
			Tools:    a.toolRegistry.GetToolDefinitions(),
		}

		// 调用 LLM
		resp, err := a.provider.Complete(ctx, req)
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
	}

	return "", fmt.Errorf("max iterations reached")
}

// runLoopStream 流式代理执行循环
func (a *Agent) runLoopStream(ctx context.Context, sessionID string, messages []llm.Message, callback stream.StreamCallback) error {
	iterations := 0
	maxIterations := a.config.MaxToolIterations
	if maxIterations <= 0 {
		maxIterations = 10
	}

	for iterations < maxIterations {
		iterations++

		// 构建 LLM 请求
		req := &llm.Request{
			Model:    a.provider.Model(),
			Messages: messages,
			Tools:    a.toolRegistry.GetToolDefinitions(),
			Stream:   true,
		}

		// 调用 LLM 流式接口
		eventChan, err := a.provider.Stream(ctx, req)
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
