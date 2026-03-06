package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/api"
	"github.com/lingguard/internal/api/handlers"
	"github.com/lingguard/internal/channels"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/cron"
	"github.com/lingguard/internal/heartbeat"
	"github.com/lingguard/internal/session"
	"github.com/lingguard/internal/taskboard"
	"github.com/lingguard/internal/tools"
	"github.com/lingguard/internal/trace"
	"github.com/lingguard/internal/webchat"
	"github.com/lingguard/pkg/httpclient"
	"github.com/lingguard/pkg/logger"
	"github.com/lingguard/pkg/memory"
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

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation: %w", err)
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

	// 初始化 HTTP 客户端池
	if cfg.Timeouts != nil {
		httpclient.Init(&httpclient.Config{
			HTTPDefault:   time.Duration(cfg.Timeouts.HTTPDefault) * time.Second,
			HTTPLong:      time.Duration(cfg.Timeouts.HTTPLong) * time.Second,
			HTTPExtraLong: time.Duration(cfg.Timeouts.HTTPExtraLong) * time.Second,
		})
	}

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
	if cfg.Tools.Cron != nil && cfg.Tools.Cron.Enabled {
		storePath := utils.ExpandHome(cfg.Tools.Cron.StorePath)
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
		ag.SetCronWrapper(cronWrapper)
	}

	// 初始化追踪服务
	var traceService *trace.Service
	var traceCollector trace.Collector
	if cfg.WebUI != nil && cfg.WebUI.Enabled && cfg.WebUI.Trace != nil && cfg.WebUI.Trace.Enabled {
		traceDBPath := utils.ExpandHome(cfg.WebUI.Trace.DBPath)
		if traceDBPath == "" {
			traceDBPath = utils.ExpandHome("~/.lingguard/webui/trace.db")
		}

		traceStore, err := trace.NewSQLiteStore(traceDBPath)
		if err != nil {
			return fmt.Errorf("create trace store: %w", err)
		}

		traceService = trace.NewService(traceStore)
		traceCollector = trace.NewCollector(traceStore)
		ag.SetTraceCollector(traceCollector)
		logger.Info("Trace service initialized", "db", traceDBPath)
	}

	// 启动 Web UI 和任务看板服务
	var apiServer *api.Server
	var taskboardService *taskboard.Service
	if cfg.WebUI != nil && cfg.WebUI.Enabled {
		// 初始化任务看板（只要配置了 taskboard 就启用）
		if cfg.WebUI.TaskBoard != nil {
			dbPath := utils.ExpandHome(cfg.WebUI.TaskBoard.DBPath)
			if dbPath == "" {
				dbPath = utils.ExpandHome("~/.lingguard/webui/taskboard.db")
			}

			store, err := taskboard.NewSQLiteStore(dbPath)
			if err != nil {
				return fmt.Errorf("create taskboard store: %w", err)
			}

			taskboardService = taskboard.NewService(store)
			ag.SetTaskboard(taskboardService)
			logger.Info("TaskBoard service initialized", "db", dbPath)

			// 同步定时任务到看板
			if cfg.WebUI.TaskBoard.SyncCron && cronService != nil {
				cronAdapter := taskboard.NewCronAdapter(taskboardService)

				// 为现有的定时任务创建看板任务
				existingJobs := cronService.ListJobs(true)
				for _, job := range existingJobs {
					if job.Enabled {
						cronAdapter.OnCronJobCreated(job)
					}
				}

				// 设置事件回调
				cronService.SetEventCallback(func(job *cron.CronJob, eventType string, result string, errMsg string) {
					if eventType == "before" {
						cronAdapter.OnCronJobExecuting(job)
					} else if eventType == "after" {
						cronAdapter.OnCronJobCompleted(job, result, errMsg)
					} else if eventType == "created" {
						cronAdapter.OnCronJobCreated(job)
					} else if eventType == "updated" {
						cronAdapter.OnCronJobUpdated(job)
					} else if eventType == "removed" {
						cronAdapter.OnCronJobRemoved(job)
					}
				})
				logger.Info("Cron to TaskBoard sync enabled")
			}
		}

		// 创建统一 Gin 服务器
		serverOpts := []api.ServerOption{
			api.WithTaskboardService(taskboardService),
			api.WithTraceService(traceService),
			api.WithSessionManager(ag.GetSessionManager()),
		}
		if cronService != nil {
			serverOpts = append(serverOpts, api.WithCronDeleter(cronService))
		}
		// Agent 必须在 SessionManager 之后设置
		serverOpts = append(serverOpts, api.WithAgent(ag))

		apiServer = api.NewServer(cfg, serverOpts...)
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

	// 创建 LaneManager 和 LaneAdapter（Steer 模式）
	laneManager := session.NewLaneManager(ag)
	laneAdapter := channels.NewLaneAdapter(laneManager, baseAdapter)

	// 使用 ContextAdapter 包装 LaneAdapter
	contextAdapter := channels.NewContextAdapter(laneAdapter, cronWrapper)
	contextAdapter.SetMessageTool(messageTool)

	var handler channels.MessageHandler = contextAdapter

	logger.Info("Steer mode enabled", "feature", "session-lane")

	// 获取工作目录（用于媒体文件存储）
	workspace := cfg.Agents.Workspace
	if workspace == "" {
		workspace = cfg.Tools.Workspace
	}
	workspace = utils.ExpandHome(workspace)

	// 注册渠道
	webChatChannel, err := registerChannels(cfg, mgr, workspace, handler, ag.GetProfileStore())
	if err != nil {
		return err
	}

	// 如果启用了 WebChat，设置 WebSocket 处理器和 API 处理器
	if webChatChannel != nil && apiServer != nil {
		apiServer.SetWebSocketHandler(webChatChannel)
		logger.Info("WebChat WebSocket handler registered")

		// 初始化 WebChat API 处理器（从 LLM 会话文件读取）
		webchatMemoryDir := utils.ExpandHome("~/.lingguard/memory/sessions")
		webchatHTTPHandler := webchat.NewHTTPHandler(webchatMemoryDir)
		webchatHandler := handlers.NewWebChatHandler(webchatHTTPHandler)
		apiServer.SetWebChatAPIHandler(webchatHandler)
		logger.Info("WebChat API handler registered", "dir", webchatMemoryDir)
	}

	// 启动 API 服务器（在设置好 WebSocket handler 之后）
	// 注意：必须在 goroutine 中启动，否则会阻塞后续的渠道启动
	if apiServer != nil {
		go func() {
			if err := apiServer.Start(); err != nil && err != http.ErrServerClosed {
				logger.Error("API server error", "error", err)
			}
		}()
		logger.Info("API server starting", "addr", apiServer.Address())
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
		if apiServer != nil {
			apiServer.Stop(shutdownCtx)
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
// 返回 WebChat channel 以便设置 WebSocket 处理器
func registerChannels(cfg *config.Config, mgr *channels.Manager, workspace string, handler channels.MessageHandler, profileStore *memory.ProfileStore) (*channels.WebChatChannel, error) {
	var webChatChannel *channels.WebChatChannel

	// 飞书渠道
	if cfg.Channels.Feishu != nil && cfg.Channels.Feishu.Enabled {
		if cfg.Channels.Feishu.AppID == "" || cfg.Channels.Feishu.AppSecret == "" {
			return nil, fmt.Errorf("feishu channel enabled but appId or appSecret not configured")
		}
		mgr.RegisterChannel(channels.NewFeishuChannel(cfg.Channels.Feishu, cfg.Tools.Speech, cfg.Providers, workspace, handler, profileStore, cfg.Agents.Soul))
		logger.Info("Feishu channel registered")
	}

	// QQ 渠道
	if cfg.Channels.QQ != nil && cfg.Channels.QQ.Enabled {
		if cfg.Channels.QQ.AppID == "" || cfg.Channels.QQ.Secret == "" {
			return nil, fmt.Errorf("qq channel enabled but appId or secret not configured")
		}
		mgr.RegisterChannel(channels.NewQQChannel(cfg.Channels.QQ, handler))
		logger.Info("QQ channel registered")
	}

	// WebChat 渠道（随 WebUI 自动启用，无需额外配置）
	if cfg.WebUI != nil && cfg.WebUI.Enabled {
		webChatChannel = channels.NewWebChatChannel(&config.WebChatConfig{}, handler)
		mgr.RegisterChannel(webChatChannel)
		logger.Info("WebChat channel registered (auto-enabled with WebUI)")
	}

	// 检查是否有渠道
	if (cfg.Channels.Feishu == nil || !cfg.Channels.Feishu.Enabled) &&
		(cfg.Channels.QQ == nil || !cfg.Channels.QQ.Enabled) &&
		(cfg.WebUI == nil || cfg.WebUI.WebChat == nil) {
		return nil, fmt.Errorf("no channels enabled, please configure at least one channel")
	}

	return webChatChannel, nil
}

// createCronJobCallback 创建定时任务执行回调
func createCronJobCallback(ag *agent.Agent, mgr *channels.Manager) cron.JobCallback {
	return func(job *cron.CronJob) (string, error) {
		logger.Info("Cron job executing",
			"name", job.Name,
			"execute", job.Payload.Execute,
			"deliver", job.Payload.Deliver,
			"channel", job.Payload.Channel,
			"to", job.Payload.To)

		var result string
		var err error

		// 执行模式：先执行 Agent，再发送通知
		if job.Payload.Execute {
			logger.Info("Executing agent for cron job", "name", job.Name, "message", job.Payload.Message)
			result, err = ag.ProcessMessage(context.Background(), "cron-"+job.ID, job.Payload.Message)
			if err != nil {
				logger.Error("Agent execution failed for cron job", "name", job.Name, "error", err)
			} else {
				logger.Info("Agent execution completed for cron job", "name", job.Name, "resultLen", len(result))
			}
		} else {
			// 纯通知模式：直接使用消息内容
			result = job.Payload.Message
		}

		// 发送通知
		if job.Payload.Deliver && job.Payload.Channel != "" && job.Payload.To != "" {
			var content string
			if job.Payload.Execute {
				if err != nil {
					// 执行失败：通知用户失败原因
					content = fmt.Sprintf("❌ **%s** 执行失败\n\n**错误信息**：%s\n\n**原始任务**：%s", job.Name, err.Error(), job.Payload.Message)
				} else if result == "" {
					// 执行成功但结果为空
					content = fmt.Sprintf("⚠️ **%s** 执行完成（无返回结果）\n\n**原始任务**：%s", job.Name, job.Payload.Message)
				} else {
					// 执行成功：显示任务名和执行结果
					content = fmt.Sprintf("✅ **%s**\n\n%s", job.Name, result)
				}
			} else {
				// 纯通知模式：显示任务名和预设消息
				content = fmt.Sprintf("⏰ **%s**\n\n%s", job.Name, job.Payload.Message)
			}

			logger.Info("Sending cron notification", "channel", job.Payload.Channel, "to", job.Payload.To, "execute", job.Payload.Execute, "hasError", err != nil)
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

		return result, err
	}
}

// createHeartbeatCallback 创建心跳回调
func createHeartbeatCallback(ag *agent.Agent) heartbeat.AgentCallback {
	return func(ctx context.Context, prompt string) (string, error) {
		return ag.ProcessMessage(ctx, "heartbeat-main", prompt)
	}
}
