// Package agent 核心代理逻辑
package agent

import (
	"context"
	"fmt"
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
)

// Agent 核心代理结构
type Agent struct {
	id           string
	provider     providers.Provider
	toolRegistry *tools.Registry
	sessions     *session.Manager
	skillsMgr    *skills.Manager
	subagentMgr  *subagent.SubagentManager
	config       *config.AgentsConfig
}

// NewAgent 创建新代理
func NewAgent(cfg *config.AgentsConfig, provider providers.Provider, skillsLoader *skills.Loader) *Agent {
	var skillsMgr *skills.Manager
	if skillsLoader != nil {
		skillsMgr = skills.NewManager(skillsLoader)
	}

	toolRegistry := tools.NewRegistry()

	agent := &Agent{
		id:           generateID(),
		provider:     provider,
		toolRegistry: toolRegistry,
		sessions:     session.NewManager(memory.NewMemoryStore(), cfg.MemoryWindow),
		skillsMgr:    skillsMgr,
		config:       cfg,
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

// buildContext 构建上下文
func (a *Agent) buildContext(sessionID string) ([]llm.Message, error) {
	messages := make([]llm.Message, 0)

	// 构建系统提示
	systemPrompt := a.config.SystemPrompt
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
