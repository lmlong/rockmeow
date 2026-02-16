package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/skills"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/logger"
)

// AgentBuilder Agent 构建器，统一 Agent 创建逻辑
type AgentBuilder struct {
	cfg            *config.Config
	registry       *providers.Registry
	provider       providers.Provider
	skillsLoader   *skills.Loader
	workspaceMgr   *tools.WorkspaceManager
	mcpManager     *tools.MCPManager
	cronService    tools.CronService
	enableMCP      bool
	enableCron     bool
	enableMessage  bool
	channelManager *tools.MessageTool
}

// NewAgentBuilder 创建 Agent 构建器
func NewAgentBuilder(cfg *config.Config) *AgentBuilder {
	return &AgentBuilder{cfg: cfg}
}

// InitProvider 初始化 Provider
func (b *AgentBuilder) InitProvider() error {
	b.registry = providers.NewRegistry()
	if err := b.registry.InitFromConfig(b.cfg); err != nil {
		return err
	}

	providerName := b.cfg.Agents.Provider
	provider, _ := b.registry.MatchProvider(providerName)
	if provider == nil {
		return fmt.Errorf("provider not found: %s", providerName)
	}

	b.registry.SetDefault(providerName)
	b.provider = provider
	return nil
}

// InitSkills 初始化技能加载器
func (b *AgentBuilder) InitSkills(verbose bool) {
	var skillDirs []string
	home, _ := os.UserHomeDir()

	// 内置技能目录
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	builtinDir := filepath.Join(execDir, "skills", "builtin")
	if _, err := os.Stat(builtinDir); err == nil {
		skillDirs = append(skillDirs, builtinDir)
		if verbose {
			fmt.Printf("Built-in skills: %s\n", builtinDir)
		}
	}

	// 用户技能目录
	userSkillsDir := filepath.Join(home, ".lingguard", "skills")
	if _, err := os.Stat(userSkillsDir); err == nil {
		skillDirs = append(skillDirs, userSkillsDir)
		if verbose {
			fmt.Printf("User skills: %s\n", userSkillsDir)
		}
	}

	if len(skillDirs) > 0 {
		b.skillsLoader = skills.NewLoader(skillDirs, "")
	}
}

// InitWorkspace 初始化工作空间
func (b *AgentBuilder) InitWorkspace() {
	workspace := b.cfg.Agents.Workspace
	if workspace == "" {
		workspace = b.cfg.Tools.Workspace
	}
	b.workspaceMgr = tools.NewWorkspaceManager(workspace, cfgPath)
}

// EnableMCP 启用 MCP 工具
func (b *AgentBuilder) EnableMCP() *AgentBuilder {
	b.enableMCP = true
	return b
}

// EnableCron 启用定时任务工具
func (b *AgentBuilder) EnableCron(service tools.CronService) *AgentBuilder {
	b.enableCron = true
	b.cronService = service
	return b
}

// EnableMessage 启用消息工具
func (b *AgentBuilder) EnableMessage(mgr *tools.MessageTool) *AgentBuilder {
	b.enableMessage = true
	b.channelManager = mgr
	return b
}

// SetMCPManager 设置 MCP 管理器（用于后续关闭）
func (b *AgentBuilder) SetMCPManager(mgr *tools.MCPManager) {
	b.mcpManager = mgr
}

// Build 构建 Agent
func (b *AgentBuilder) Build() (*agent.Agent, error) {
	if b.provider == nil {
		if err := b.InitProvider(); err != nil {
			return nil, err
		}
	}

	if b.workspaceMgr == nil {
		b.InitWorkspace()
	}

	ag := agent.NewAgent(&b.cfg.Agents, b.provider, b.skillsLoader)

	// 注册基础工具
	ag.RegisterTool(tools.NewShellTool(b.workspaceMgr, b.cfg.Tools.RestrictToWorkspace))
	ag.RegisterTool(tools.NewFileTool(b.workspaceMgr, b.cfg.Tools.RestrictToWorkspace))
	ag.RegisterTool(tools.NewWorkspaceTool(b.workspaceMgr))

	// 注册 Web 工具
	tavilyAPIKey := b.cfg.Tools.TavilyAPIKey
	if tavilyAPIKey == "" {
		tavilyAPIKey = os.Getenv("TAVILY_API_KEY")
	}
	ag.RegisterTool(tools.NewWebSearchTool(tavilyAPIKey, 5))
	ag.RegisterTool(tools.NewWebFetchTool(b.cfg.Tools.WebMaxChars))

	// 注册技能工具
	ag.RegisterSkillTool()

	// 注册子代理工具
	ag.RegisterSubagentTools()

	// 注册记忆工具
	ag.RegisterMemoryTool()

	// 注册 OpenCode 工具
	if b.cfg.Tools.OpenCode != nil && b.cfg.Tools.OpenCode.Enabled {
		openCodeCfg := tools.DefaultOpenCodeConfig()
		if b.cfg.Tools.OpenCode.BaseURL != "" {
			openCodeCfg.BaseURL = b.cfg.Tools.OpenCode.BaseURL
		}
		if b.cfg.Tools.OpenCode.Timeout > 0 {
			openCodeCfg.Timeout = time.Duration(b.cfg.Tools.OpenCode.Timeout) * time.Second
		}
		ag.RegisterTool(tools.NewOpenCodeTool(openCodeCfg))
		logger.Info("OpenCode tool enabled", "baseURL", openCodeCfg.BaseURL)
	} else {
		if b.cfg.Tools.OpenCode == nil {
			logger.Debug("OpenCode config is nil, tool not registered")
		} else {
			logger.Debug("OpenCode not enabled, tool not registered")
		}
	}

	// 注册可选工具
	if b.enableCron && b.cronService != nil {
		ag.RegisterCronTool(b.cronService)
	}

	if b.enableMessage && b.channelManager != nil {
		ag.RegisterTool(b.channelManager)
	}

	return ag, nil
}

// ConnectMCP 连接 MCP 服务器
func (b *AgentBuilder) ConnectMCP(ag *agent.Agent) (*tools.MCPManager, error) {
	if len(b.cfg.Tools.MCPServers) == 0 {
		return nil, nil
	}

	// 确保工作空间已初始化
	if b.workspaceMgr == nil {
		b.InitWorkspace()
	}

	mcpManager := tools.NewMCPManager()
	ctx := context.Background()
	workspace := b.workspaceMgr.Get()
	if err := mcpManager.ConnectServers(ctx, b.cfg.Tools.MCPServers, workspace); err != nil {
		return nil, err
	}

	// 注册 MCP 工具
	for name, tool := range mcpManager.GetTools() {
		ag.RegisterTool(tool)
		logger.Debug("Registered MCP tool", "name", name)
	}

	b.mcpManager = mcpManager
	return mcpManager, nil
}

// GetMCPManager 获取 MCP 管理器
func (b *AgentBuilder) GetMCPManager() *tools.MCPManager {
	return b.mcpManager
}

// GetProvider 获取 Provider
func (b *AgentBuilder) GetProvider() providers.Provider {
	return b.provider
}

// GetRegistry 获取 Provider 注册表
func (b *AgentBuilder) GetRegistry() *providers.Registry {
	return b.registry
}

// GetWorkspaceManager 获取工作空间管理器
func (b *AgentBuilder) GetWorkspaceManager() *tools.WorkspaceManager {
	return b.workspaceMgr
}
