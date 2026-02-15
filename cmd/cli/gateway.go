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
	"github.com/lingguard/internal/cron"
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

	// 创建基础 AgentAdapter
	baseAdapter := channels.NewAgentAdapter(ag)

	// 启动定时任务服务（先启动，这样 ContextAdapter 才能正确包装）
	var cronService *cron.Service
	var cronWrapper *tools.CronServiceWrapper
	if cfg.Cron != nil && cfg.Cron.Enabled {
		storePath := expandHomePath(cfg.Cron.StorePath)
		if storePath == "" {
			storePath = expandHomePath("~/.lingguard/cron/jobs.json")
		}

		// 创建任务执行回调
		onJob := createCronJobCallback(ag, mgr)

		cronService = cron.NewService(storePath, onJob)
		if err := cronService.Start(); err != nil {
			return fmt.Errorf("start cron service: %w", err)
		}
		logger.Info("Cron service started")

		// 创建包装器并注册到 Agent
		cronWrapper = tools.NewCronServiceWrapper(cronService)
		ag.RegisterCronTool(cronWrapper)
	}

	// 使用 ContextAdapter 包装 AgentAdapter（如果 cron 启用）
	var adapter channels.MessageHandler = baseAdapter
	if cronWrapper != nil {
		adapter = channels.NewContextAdapter(baseAdapter, cronWrapper)
		logger.Info("ContextAdapter enabled for cron delivery")
	}

	// 注册飞书渠道
	if cfg.Channels.Feishu != nil && cfg.Channels.Feishu.Enabled {
		if cfg.Channels.Feishu.AppID == "" || cfg.Channels.Feishu.AppSecret == "" {
			return fmt.Errorf("feishu channel enabled but appId or appSecret not configured")
		}
		fc := channels.NewFeishuChannel(cfg.Channels.Feishu, adapter)
		mgr.RegisterChannel(fc)
		logger.Info("Feishu channel registered")
	}

	// 注册 QQ 渠道
	if cfg.Channels.QQ != nil && cfg.Channels.QQ.Enabled {
		if cfg.Channels.QQ.AppID == "" || cfg.Channels.QQ.Secret == "" {
			return fmt.Errorf("qq channel enabled but appId or secret not configured")
		}
		qc := channels.NewQQChannel(cfg.Channels.QQ, adapter)
		mgr.RegisterChannel(qc)
		logger.Info("QQ channel registered")
	}

	// 检查是否有渠道注册
	hasChannel := (cfg.Channels.Feishu != nil && cfg.Channels.Feishu.Enabled) ||
		(cfg.Channels.QQ != nil && cfg.Channels.QQ.Enabled)
	if !hasChannel {
		return fmt.Errorf("no channels enabled, please configure at least one channel (feishu, qq)")
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

	// 停止定时任务服务
	if cronService != nil {
		cronService.Stop()
	}

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
	provider, spec := registry.MatchProvider(providerName)
	if provider == nil {
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}
	_ = spec // spec 可用于获取 DisplayName 等信息

	// 3. 设置默认 Provider
	registry.SetDefault(providerName)

	// 4. 创建 Skills Loader
	// 支持从多个目录加载技能：
	// 1. 程序执行目录的 skills/builtin/（内置技能）
	// 2. ~/.lingguard/skills/（用户技能）
	var skillDirs []string
	home, _ := os.UserHomeDir()

	// 内置技能目录：程序执行目录的 skills/builtin/
	execPath, _ := os.Executable()
	execDir := filepath.Dir(execPath)
	builtinDir := filepath.Join(execDir, "skills", "builtin")
	if _, err := os.Stat(builtinDir); err == nil {
		skillDirs = append(skillDirs, builtinDir)
		logger.Info("Built-in skills: %s", builtinDir)
	}

	// 用户技能目录：~/.lingguard/skills/
	userSkillsDir := filepath.Join(home, ".lingguard", "skills")
	if _, err := os.Stat(userSkillsDir); err == nil {
		skillDirs = append(skillDirs, userSkillsDir)
		logger.Info("User skills: %s", userSkillsDir)
	}

	var skillsLoader *skills.Loader
	if len(skillDirs) > 0 {
		skillsLoader = skills.NewLoader(skillDirs, "")
	}

	// 5. 创建 Agent
	ag := agent.NewAgent(&cfg.Agents, provider, skillsLoader)

	// 6. 创建工作目录管理器
	workspace := cfg.Agents.Workspace
	if workspace == "" {
		workspace = cfg.Tools.Workspace
	}
	workspaceMgr := tools.NewWorkspaceManager(workspace, cfgPath)

	// 7. 注册工具
	ag.RegisterTool(tools.NewShellTool(workspaceMgr, cfg.Tools.RestrictToWorkspace))
	ag.RegisterTool(tools.NewFileTool(workspaceMgr, cfg.Tools.RestrictToWorkspace))
	ag.RegisterTool(tools.NewWorkspaceTool(workspaceMgr))

	// 注册 Web 工具
	braveAPIKey := cfg.Tools.BraveAPIKey
	if braveAPIKey == "" {
		braveAPIKey = os.Getenv("BRAVE_API_KEY")
	}
	ag.RegisterTool(tools.NewWebSearchTool(braveAPIKey, 5))
	ag.RegisterTool(tools.NewWebFetchTool(cfg.Tools.WebMaxChars))

	// 7. 注册技能工具（支持按需加载技能）
	ag.RegisterSkillTool()

	// 8. 注册子代理工具（支持后台任务）
	ag.RegisterSubagentTools()

	// 9. 注册记忆工具（参考 nanobot）
	ag.RegisterMemoryTool()

	return ag, nil
}

// createCronJobCallback 创建定时任务执行回调
func createCronJobCallback(ag *agent.Agent, mgr *channels.Manager) cron.JobCallback {
	return func(job *cron.CronJob) (string, error) {
		ctx := context.Background()
		sessionID := fmt.Sprintf("cron-%s", job.ID)

		// 执行 Agent 处理消息
		response, err := ag.ProcessMessage(ctx, sessionID, job.Payload.Message)
		if err != nil {
			return "", err
		}

		// 如果需要投递到渠道
		if job.Payload.Deliver && job.Payload.Channel != "" && job.Payload.To != "" {
			if err := mgr.SendMessage(job.Payload.Channel, job.Payload.To, response); err != nil {
				logger.Error("Failed to deliver cron job response: %v", err)
			}
		}

		return response, nil
	}
}

// expandHomePath 展开路径中的 ~ 为用户主目录
func expandHomePath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
