package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/channels"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/providers"
	"github.com/lingguard/internal/skills"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/logger"
	"github.com/spf13/cobra"
)

var gatewayCmd = &cobra.Command{
	Use:   "gateway",
	Short: "Start the messaging gateway",
	Long:  `Start the messaging gateway to receive and respond to messages from various platforms.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runGateway(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(gatewayCmd)
}

func runGateway() error {
	// 加载配置
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 初始化日志
	logger.Init(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)

	// 创建 Agent
	ag, err := createGatewayAgent(cfg)
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// 创建 Channel Manager
	mgr := channels.NewManager()
	adapter := channels.NewAgentAdapter(ag)

	// 注册飞书渠道
	if cfg.Channels.Feishu != nil && cfg.Channels.Feishu.Enabled {
		if cfg.Channels.Feishu.AppID == "" || cfg.Channels.Feishu.AppSecret == "" {
			return fmt.Errorf("feishu channel enabled but appId or appSecret not configured")
		}
		fc := channels.NewFeishuChannel(cfg.Channels.Feishu, adapter)
		mgr.RegisterChannel(fc)
		logger.Info("Feishu channel registered")
	}

	// 检查是否有渠道注册
	if cfg.Channels.Feishu == nil || !cfg.Channels.Feishu.Enabled {
		return fmt.Errorf("no channels enabled, please configure at least one channel")
	}

	// 启动
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.StartAll(ctx); err != nil {
		return fmt.Errorf("start channels: %w", err)
	}

	fmt.Println("Gateway started, press Ctrl+C to stop")
	logger.Info("Gateway started successfully")

	// 等待信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	logger.Info("Gateway shutting down")
	return mgr.StopAll()
}

func createGatewayAgent(cfg *config.Config) (*agent.Agent, error) {
	// 1. 创建 Provider 注册表
	registry := providers.NewRegistry()
	if err := registry.InitFromConfig(cfg); err != nil {
		return nil, err
	}

	// 2. 通过 provider 配置获取 Provider
	providerName := cfg.Agents.Provider
	provider, ok := registry.MatchProvider(providerName)
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}

	// 3. 设置默认 Provider
	registry.SetDefault(providerName)

	// 4. 创建 Skills Loader
	var skillsLoader *skills.Loader
	var builtinDirs []string

	// 如果配置了内置技能目录，使用配置的
	if cfg.Agents.SkillsBuiltinDir != "" {
		builtinDirs = append(builtinDirs, cfg.Agents.SkillsBuiltinDir)
	} else {
		// 自动发现内置技能目录
		execPath, _ := os.Executable()
		home, _ := os.UserHomeDir()
		cwd, _ := os.Getwd()

		// 候选路径（按优先级排序）
		candidatePaths := []string{
			// 1. 相对于可执行文件的 skills 目录
			filepath.Join(filepath.Dir(execPath), "skills"),
			// 2. 相对于可执行文件的上级目录
			filepath.Join(filepath.Dir(execPath), "..", "skills"),
			// 3. 用户主目录下的 .lingguard/skills
			filepath.Join(home, ".lingguard", "skills"),
			// 4. 当前工作目录下的 skills
			filepath.Join(cwd, "skills"),
		}

		// 去重：使用 map 记录已添加的绝对路径
		seen := make(map[string]bool)
		for _, p := range candidatePaths {
			absPath, err := filepath.Abs(p)
			if err != nil {
				absPath = p
			}
			if _, err := os.Stat(p); err == nil && !seen[absPath] {
				seen[absPath] = true
				builtinDirs = append(builtinDirs, p)
			}
		}
	}

	workspaceSkills := cfg.Agents.SkillsWorkspace

	if len(builtinDirs) > 0 || workspaceSkills != "" {
		skillsLoader = skills.NewLoader(builtinDirs, workspaceSkills)
		if len(builtinDirs) > 0 {
			logger.Info("Skills loaded from: %v", builtinDirs)
		}
	}

	// 5. 创建 Agent
	ag := agent.NewAgent(&cfg.Agents, provider, skillsLoader)

	// 6. 注册工具
	workspace := cfg.Agents.Workspace
	if workspace == "" {
		workspace = cfg.Tools.Workspace
	}
	ag.RegisterTool(tools.NewShellTool(workspace, cfg.Tools.RestrictToWorkspace))
	ag.RegisterTool(tools.NewFileTool(workspace, cfg.Tools.RestrictToWorkspace))

	// 7. 注册技能工具（支持按需加载技能）
	ag.RegisterSkillTool()

	// 8. 注册子代理工具（支持后台任务）
	ag.RegisterSubagentTools()

	// 9. 注册记忆工具（参考 nanobot）
	ag.RegisterMemoryTool()

	return ag, nil
}
