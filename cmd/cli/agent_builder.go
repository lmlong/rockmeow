package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/skills"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/logger"
	ttspkg "github.com/lingguard/pkg/tts"
)

// AgentBuilder Agent 构建器，统一 Agent 创建逻辑
type AgentBuilder struct {
	cfg                *config.Config
	registry           *providers.Registry
	provider           providers.Provider
	multimodalProvider providers.Provider // 多模态 Provider
	skillsLoader       *skills.Loader
	workspaceMgr       *tools.WorkspaceManager
	mcpManager         *tools.MCPManager
	cronService        tools.CronService
	enableMCP          bool
	enableCron         bool
	enableMessage      bool
	channelManager     *tools.MessageTool
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

	// 初始化多模态 Provider（如果配置了）
	if b.cfg.Agents.MultimodalProvider != "" {
		mmProvider, _ := b.registry.MatchProvider(b.cfg.Agents.MultimodalProvider)
		if mmProvider == nil {
			logger.Warn("Multimodal provider not found, using default provider", "name", b.cfg.Agents.MultimodalProvider)
		} else {
			b.multimodalProvider = mmProvider
			logger.Info("Multimodal provider initialized", "name", b.cfg.Agents.MultimodalProvider)
		}
	}

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
	userSkillsDir := filepath.Join(home, ".lingguard", "skills", "builtin")
	if _, err := os.Stat(userSkillsDir); err == nil {
		skillDirs = append(skillDirs, userSkillsDir)
		if verbose {
			fmt.Printf("User skills: %s\n", userSkillsDir)
		}
	}

	// 工作区技能目录 (ClawHub 安装的技能)
	workspaceSkillsDir := filepath.Join(home, ".lingguard", "workspace", "skills")
	if _, err := os.Stat(workspaceSkillsDir); err == nil {
		if verbose {
			fmt.Printf("Workspace skills: %s\n", workspaceSkillsDir)
		}
	}

	if len(skillDirs) > 0 || workspaceSkillsDir != "" {
		b.skillsLoader = skills.NewLoader(skillDirs, workspaceSkillsDir)
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

	// 使用带完整配置的 Agent 创建方法（支持向量存储）
	ag := agent.NewAgentWithMultimodalAndConfig(&b.cfg.Agents, b.provider, b.multimodalProvider, b.skillsLoader, b.cfg)

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

	// 注册 AI 内容生成工具（图像/视频）
	if b.cfg.Tools.AIGC != nil && b.cfg.Tools.AIGC.Enabled {
		aigcCfg := tools.DefaultAIGCConfig()
		aigcCfg.APIKey = b.cfg.Tools.AIGC.APIKey

		// 从 Provider 配置继承 API Key
		if aigcCfg.APIKey == "" {
			providerName := b.cfg.Tools.AIGC.Provider
			if providerName == "" {
				providerName = "qwen"
			}
			if p, ok := b.cfg.Providers[providerName]; ok {
				aigcCfg.APIKey = p.APIKey
			}
		}

		// 从配置读取模型
		if b.cfg.Tools.AIGC.TextToImage != "" {
			aigcCfg.TextToImage = b.cfg.Tools.AIGC.TextToImage
		}
		if b.cfg.Tools.AIGC.TextToVideo != "" {
			aigcCfg.TextToVideo = b.cfg.Tools.AIGC.TextToVideo
		}
		if b.cfg.Tools.AIGC.ImageToVideo != "" {
			aigcCfg.ImageToVideo = b.cfg.Tools.AIGC.ImageToVideo
		}
		if b.cfg.Tools.AIGC.VideoToVideo != "" {
			aigcCfg.VideoToVideo = b.cfg.Tools.AIGC.VideoToVideo
		}
		if b.cfg.Tools.AIGC.ImageToVideoDuration > 0 {
			aigcCfg.ImageToVideoDuration = b.cfg.Tools.AIGC.ImageToVideoDuration
		}
		if b.cfg.Tools.AIGC.OutputDir != "" {
			aigcCfg.OutputDir = b.cfg.Tools.AIGC.OutputDir
		}
		// 设置工作区路径用于沙盒验证
		if b.workspaceMgr != nil {
			aigcCfg.Workspace = b.workspaceMgr.Get()
		}
		aigcCfg.Sandboxed = b.cfg.Tools.RestrictToWorkspace

		ag.RegisterTool(tools.NewAIGCTool(aigcCfg))
		logger.Info("AIGC tool enabled", "textToImage", aigcCfg.TextToImage, "textToVideo", aigcCfg.TextToVideo, "imageToVideo", aigcCfg.ImageToVideo, "videoToVideo", aigcCfg.VideoToVideo, "imageToVideoDuration", aigcCfg.ImageToVideoDuration)
	}

	// 注册 TTS 语音合成工具
	if b.cfg.Tools.TTS != nil && b.cfg.Tools.TTS.Enabled {
		ttsCfg := &ttspkg.Config{
			Provider:  b.cfg.Tools.TTS.Provider,
			APIKey:    b.cfg.Tools.TTS.APIKey,
			APIBase:   b.cfg.Tools.TTS.APIBase,
			Model:     b.cfg.Tools.TTS.Model,
			Voice:     b.cfg.Tools.TTS.Voice,
			Language:  b.cfg.Tools.TTS.Language,
			Timeout:   b.cfg.Tools.TTS.Timeout,
			OutputDir: b.cfg.Tools.TTS.OutputDir,
		}

		// 从 Provider 配置继承 API Key
		if ttsCfg.APIKey == "" {
			providerName := b.cfg.Tools.TTS.Provider
			if providerName == "" {
				providerName = "qwen"
			}
			if p, ok := b.cfg.Providers[providerName]; ok {
				ttsCfg.APIKey = p.APIKey
			}
		}

		// 设置默认值
		if ttsCfg.Provider == "" {
			ttsCfg.Provider = "qwen"
		}
		if ttsCfg.Model == "" {
			ttsCfg.Model = "qwen3-tts-flash"
		}
		if ttsCfg.Voice == "" {
			ttsCfg.Voice = "Cherry"
		}
		if ttsCfg.OutputDir == "" {
			home, _ := os.UserHomeDir()
			ttsCfg.OutputDir = filepath.Join(home, ".lingguard", "workspace", "generated")
		}

		ag.RegisterTool(tools.NewTTSTool(ttsCfg))
		logger.Info("TTS tool enabled", "model", ttsCfg.Model, "voice", ttsCfg.Voice)
	}

	// 注册 OpenCode 工具（即使 disabled 也注册，会返回原生工具提示）
	{
		openCodeCfg := tools.DefaultOpenCodeConfig()
		enabled := b.cfg.Tools.OpenCode != nil && b.cfg.Tools.OpenCode.Enabled

		if b.cfg.Tools.OpenCode != nil {
			if b.cfg.Tools.OpenCode.BaseURL != "" {
				openCodeCfg.BaseURL = b.cfg.Tools.OpenCode.BaseURL
			}
			if b.cfg.Tools.OpenCode.Timeout > 0 {
				openCodeCfg.Timeout = time.Duration(b.cfg.Tools.OpenCode.Timeout) * time.Second
			}
		}

		// Set workspace from agents config, fallback to default
		workspace := b.cfg.Agents.Workspace
		if workspace == "" {
			// 默认使用 ~/.lingguard/workspace
			home, _ := os.UserHomeDir()
			workspace = filepath.Join(home, ".lingguard", "workspace")
			logger.Debug("Using default workspace for OpenCode", "path", workspace)
		}
		// 展开 ~ 路径
		if strings.HasPrefix(workspace, "~") {
			home, _ := os.UserHomeDir()
			workspace = filepath.Join(home, workspace[1:])
		}
		openCodeCfg.Workspace = workspace
		openCodeCfg.Enabled = enabled

		ag.RegisterTool(tools.NewOpenCodeTool(openCodeCfg))
		if enabled {
			logger.Info("OpenCode tool enabled", "baseURL", openCodeCfg.BaseURL, "workspace", openCodeCfg.Workspace)
		} else {
			logger.Info("OpenCode tool registered in fallback mode (will use native tools)")
		}
	}

	// 注册 Moltbook AI 社交网络工具
	if b.cfg.Tools.Moltbook != nil && b.cfg.Tools.Moltbook.Enabled {
		ag.RegisterTool(tools.NewMoltbookTool(
			b.cfg.Tools.Moltbook.APIKey,
			b.cfg.Tools.Moltbook.AgentName,
		))
		logger.Info("Moltbook tool enabled", "agentName", b.cfg.Tools.Moltbook.AgentName)
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
