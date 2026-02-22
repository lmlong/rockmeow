package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/channels"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/cron"
	"github.com/lingguard/internal/heartbeat"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/utils"
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
	// 单实例检查
	lock, err := utils.NewSingletonLock("gateway")
	if err != nil {
		return fmt.Errorf("singleton check failed: %w", err)
	}
	defer lock.Release()

	// 加载配置
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 初始化日志
	logger.InitWithConfig(logger.Config{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		Output:     cfg.Logging.Output,
		MaxSize:    cfg.Logging.MaxSize,
		MaxAge:     cfg.Logging.MaxAge,
		MaxBackups: cfg.Logging.MaxBackups,
		Compress:   cfg.Logging.Compress,
	})

	// 创建 Agent（使用 AgentBuilder）
	builder := NewAgentBuilder(cfg)
	builder.InitSkills(false)
	if err := builder.InitProvider(); err != nil {
		return fmt.Errorf("init provider: %w", err)
	}
	builder.InitWorkspace()

	ag, err := builder.Build()
	if err != nil {
		return fmt.Errorf("create agent: %w", err)
	}

	// 创建 Channel Manager
	mgr := channels.NewManager()

	// 创建 MessageTool
	messageTool := tools.NewMessageTool(mgr)
	ag.RegisterTool(messageTool)

	// 连接 MCP 服务器
	mcpManager, err := builder.ConnectMCP(ag)
	if err != nil {
		logger.Error("Failed to connect MCP servers", "error", err)
	}

	// 启动定时任务服务
	var cronService *cron.Service
	var cronWrapper *tools.CronServiceWrapper
	if cfg.Cron != nil && cfg.Cron.Enabled {
		storePath := utils.ExpandHome(cfg.Cron.StorePath)
		if storePath == "" {
			storePath = utils.ExpandHome("~/.lingguard/cron/jobs.json")
		}

		cronService = cron.NewService(storePath, createCronJobCallback(ag, mgr))
		if err := cronService.Start(); err != nil {
			return fmt.Errorf("start cron service: %w", err)
		}
		logger.Info("Cron service started")

		cronWrapper = tools.NewCronServiceWrapper(cronService)
		ag.RegisterCronTool(cronWrapper)
	}

	// 启动心跳服务
	var heartbeatService *heartbeat.Service
	if cfg.Heartbeat != nil && cfg.Heartbeat.Enabled {
		interval := time.Duration(cfg.Heartbeat.Interval) * time.Minute
		if interval <= 0 {
			interval = 30 * time.Minute
		}

		heartbeatService = heartbeat.NewService(&heartbeat.Config{
			Enabled:  true,
			Interval: interval,
		}, createHeartbeatCallback(ag))

		hbWorkspace := cfg.Agents.Workspace
		if hbWorkspace == "" {
			hbWorkspace = cfg.Tools.Workspace
		}
		heartbeatService.SetWorkspace(utils.ExpandHome(hbWorkspace))
		heartbeatService.Start()
		logger.Info("Heartbeat service started", "interval", interval)
	}

	// 创建消息处理器
	baseAdapter := channels.NewAgentAdapter(ag)
	contextAdapter := channels.NewContextAdapter(baseAdapter, cronWrapper)
	contextAdapter.SetMessageTool(messageTool)
	var handler channels.MessageHandler = contextAdapter

	// 获取工作目录（用于媒体文件存储）
	workspace := cfg.Agents.Workspace
	if workspace == "" {
		workspace = cfg.Tools.Workspace
	}
	workspace = utils.ExpandHome(workspace)

	// 注册渠道
	if err := registerChannels(cfg, mgr, workspace, handler); err != nil {
		return err
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

	// 创建带超时的关闭上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// 清理资源（使用 goroutine 并行关闭，避免阻塞）
	done := make(chan struct{})
	go func() {
		if mcpManager != nil {
			mcpManager.Close()
		}
		if cronService != nil {
			cronService.Stop()
		}
		if heartbeatService != nil {
			heartbeatService.Stop()
		}
		if err := mgr.StopAll(); err != nil {
			logger.Warn("Error stopping channels", "error", err)
		}
		close(done)
	}()

	// 等待关闭完成或超时
	select {
	case <-done:
		logger.Info("Gateway shutdown complete")
	case <-shutdownCtx.Done():
		logger.Warn("Gateway shutdown timed out, some resources may not be cleaned up")
	}

	return nil
}

// registerChannels 注册所有渠道
func registerChannels(cfg *config.Config, mgr *channels.Manager, workspace string, handler channels.MessageHandler) error {
	// 飞书渠道
	if cfg.Channels.Feishu != nil && cfg.Channels.Feishu.Enabled {
		if cfg.Channels.Feishu.AppID == "" || cfg.Channels.Feishu.AppSecret == "" {
			return fmt.Errorf("feishu channel enabled but appId or appSecret not configured")
		}
		mgr.RegisterChannel(channels.NewFeishuChannel(cfg.Channels.Feishu, cfg.Speech, cfg.Providers, workspace, handler))
		logger.Info("Feishu channel registered")
	}

	// QQ 渠道
	if cfg.Channels.QQ != nil && cfg.Channels.QQ.Enabled {
		if cfg.Channels.QQ.AppID == "" || cfg.Channels.QQ.Secret == "" {
			return fmt.Errorf("qq channel enabled but appId or secret not configured")
		}
		mgr.RegisterChannel(channels.NewQQChannel(cfg.Channels.QQ, handler))
		logger.Info("QQ channel registered")
	}

	// 检查是否有渠道
	if (cfg.Channels.Feishu == nil || !cfg.Channels.Feishu.Enabled) &&
		(cfg.Channels.QQ == nil || !cfg.Channels.QQ.Enabled) {
		return fmt.Errorf("no channels enabled, please configure at least one channel")
	}

	return nil
}

// createCronJobCallback 创建定时任务执行回调
func createCronJobCallback(ag *agent.Agent, mgr *channels.Manager) cron.JobCallback {
	return func(job *cron.CronJob) (string, error) {
		logger.Info("Cron job executing",
			"name", job.Name,
			"deliver", job.Payload.Deliver,
			"channel", job.Payload.Channel,
			"to", job.Payload.To)

		// 直接发送通知（不经过 LLM）
		if job.Payload.Deliver && job.Payload.Channel != "" && job.Payload.To != "" {
			content := fmt.Sprintf("⏰ **%s**\n\n%s", job.Name, job.Payload.Message)
			logger.Info("Sending cron notification", "channel", job.Payload.Channel, "to", job.Payload.To)

			if sendErr := mgr.SendMessage(job.Payload.Channel, job.Payload.To, content); sendErr != nil {
				logger.Error("Failed to send cron notification", "error", sendErr)
			} else {
				logger.Info("Cron notification sent", "name", job.Name)
			}
		} else {
			logger.Warn("Cron notification skipped",
				"name", job.Name,
				"deliver", job.Payload.Deliver,
				"channel", job.Payload.Channel,
				"to", job.Payload.To)
		}

		// 返回成功，不调用 LLM
		return job.Payload.Message, nil
	}
}

// createHeartbeatCallback 创建心跳回调
func createHeartbeatCallback(ag *agent.Agent) heartbeat.AgentCallback {
	return func(ctx context.Context, prompt string) (string, error) {
		return ag.ProcessMessage(ctx, "heartbeat-main", prompt)
	}
}
