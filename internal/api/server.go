// Package api 统一 Gin HTTP 服务器
package api

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/lingguard/internal/agent"
	"github.com/lingguard/internal/api/handlers"
	"github.com/lingguard/internal/api/middleware"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/session"
	"github.com/lingguard/internal/taskboard"
	"github.com/lingguard/internal/taskboard/web"
	"github.com/lingguard/internal/trace"
	"github.com/lingguard/pkg/logger"
)

// WebSocketHandler WebSocket 处理器接口
type WebSocketHandler interface {
	HandleWebSocket(conn *websocket.Conn, sessionID string)
}

// WebChatAPIHandler WebChat API 处理器接口
type WebChatAPIHandler interface {
	RegisterRoutes(router *gin.RouterGroup)
}

// Server 统一 Gin 服务器
type Server struct {
	config     *config.Config
	httpServer *http.Server
	router     *gin.Engine

	// Services
	taskboardSvc *taskboard.Service
	traceSvc     *trace.Service

	// Handlers
	taskboardHandler  *handlers.TaskboardHandler
	traceHandler      *handlers.TraceHandler
	webchatAPIHandler WebChatAPIHandler
	chatHandler       *handlers.ChatHandler
	sessionHandler    *handlers.SessionHandler
	taskHandler       *handlers.TaskHandler

	// WebSocket
	wsHandler WebSocketHandler
}

// ServerOption 服务器选项
type ServerOption func(*Server)

// WithTaskboardService 设置任务看板服务
func WithTaskboardService(svc *taskboard.Service) ServerOption {
	return func(s *Server) { s.taskboardSvc = svc }
}

// WithTraceService 设置追踪服务
func WithTraceService(svc *trace.Service) ServerOption {
	return func(s *Server) { s.traceSvc = svc }
}

// WithWebSocketHandler 设置 WebSocket 处理器
func WithWebSocketHandler(h WebSocketHandler) ServerOption {
	return func(s *Server) { s.wsHandler = h }
}

// WithCronDeleter 设置 cron 删除器
func WithCronDeleter(deleter taskboard.CronDeleter) ServerOption {
	return func(s *Server) {
		if s.taskboardHandler != nil {
			s.taskboardHandler.SetCronDeleter(deleter)
		}
	}
}

// WithAgent 设置 Agent（用于 Chat API）
func WithAgent(ag *agent.Agent) ServerOption {
	return func(s *Server) {
		if ag != nil && s.sessionHandler != nil {
			s.chatHandler = handlers.NewChatHandler(ag, s.sessionHandler.GetSessionManager())
		}
	}
}

// WithSessionManager 设置会话管理器
func WithSessionManager(sessionMgr *session.Manager) ServerOption {
	return func(s *Server) {
		if sessionMgr != nil {
			s.sessionHandler = handlers.NewSessionHandler(sessionMgr)
		}
	}
}

// WithTaskHandler 设置任务处理器
func WithTaskHandler(handler *handlers.TaskHandler) ServerOption {
	return func(s *Server) {
		s.taskHandler = handler
	}
}

// SetWebSocketHandler 设置 WebSocket 处理器
func (s *Server) SetWebSocketHandler(h WebSocketHandler) {
	s.wsHandler = h
	// 动态注册 WebSocket 路由
	s.router.GET("/ws/chat", handlers.HandleWebSocket(h))
}

// SetWebChatAPIHandler 设置 WebChat API 处理器
func (s *Server) SetWebChatAPIHandler(h WebChatAPIHandler) {
	s.webchatAPIHandler = h
	// 动态注册 WebChat API 路由
	h.RegisterRoutes(s.router.Group(""))
}

// NewServer 创建统一服务器
func NewServer(cfg *config.Config, opts ...ServerOption) *Server {
	// 设置 Gin 模式
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// 内置中间件
	router.Use(gin.Recovery())
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.LoggerMiddleware())

	// CORS 中间件（全局）
	corsConfig := cfg.WebUI.CORS
	if corsConfig == nil && cfg.WebUI != nil {
		corsConfig = &config.CORSConfig{
			AllowedOrigins: []string{"*"},
		}
	}
	if corsConfig != nil {
		router.Use(middleware.CORSMiddleware(corsConfig))
	}

	s := &Server{
		config: cfg,
		router: router,
	}

	// 应用选项
	for _, opt := range opts {
		opt(s)
	}

	// 创建处理器
	if s.taskboardSvc != nil {
		s.taskboardHandler = handlers.NewTaskboardHandler(s.taskboardSvc)
	}
	if s.traceSvc != nil {
		s.traceHandler = handlers.NewTraceHandler(s.traceSvc)
	}

	// 注册路由
	s.registerRoutes()

	return s
}

// registerRoutes 注册所有路由
func (s *Server) registerRoutes() {
	// ========== Agent API (/v1/*) ==========
	if s.config.API != nil && s.config.API.Enabled {
		v1 := s.router.Group("/v1")

		// 认证中间件（仅对 /v1/* 路由）
		if s.config.API.Auth != nil && s.config.API.Auth.Type == "token" && len(s.config.API.Auth.Tokens) > 0 {
			v1.Use(middleware.AuthMiddleware(s.config.API.Auth.Tokens))
		}

		// 限流中间件
		if s.config.API.RateLimit != nil && s.config.API.RateLimit.Enabled {
			v1.Use(middleware.RateLimitMiddleware(s.config.API.RateLimit))
		}

		// Health (无需认证)
		v1.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})

		// Chat API
		if s.chatHandler != nil {
			s.chatHandler.RegisterRoutes(v1)
		}

		// Session API
		if s.sessionHandler != nil {
			s.sessionHandler.RegisterRoutes(v1)
		}

		// Task API
		if s.taskHandler != nil {
			s.taskHandler.RegisterRoutes(v1)
		}

		// TODO: Phase 5 - Tool/Agent API
		// v1.GET("/tools", s.handleListTools)
		// v1.POST("/tools/:tool_name/execute", s.handleExecuteTool)
		// v1.GET("/agents", s.handleListAgents)
		// v1.GET("/agents/:agent_id", s.handleGetAgent)
	}

	// ========== TaskBoard & Trace API (/api/*) ==========
	if s.taskboardHandler != nil {
		s.taskboardHandler.RegisterRoutes(s.router.Group(""))
	}
	if s.traceHandler != nil {
		s.traceHandler.RegisterRoutes(s.router.Group(""))
	}

	// 注意：WebChat API 路由在 SetWebChatAPIHandler 中动态注册
	// 注意：WebSocket 路由在 SetWebSocketHandler 中动态注册

	// ========== 静态文件 (Web UI) ==========
	s.registerStaticFiles()
}

// registerStaticFiles 注册静态文件服务
func (s *Server) registerStaticFiles() {
	// 从嵌入的 FS 读取静态文件
	staticFS, err := fs.Sub(web.StaticFiles, "static")
	if err != nil {
		logger.Error("Failed to load static files", "error", err)
		return
	}

	// 静态资源目录
	s.router.StaticFS("/static", http.FS(staticFS))

	// Favicon
	s.router.GET("/favicon.ico", func(c *gin.Context) {
		c.FileFromFS("favicon.ico", http.FS(staticFS))
	})

	// 静态 HTML 页面路由
	serveHTML := func(filename string) gin.HandlerFunc {
		return func(c *gin.Context) {
			data, err := fs.ReadFile(staticFS, filename)
			if err != nil {
				logger.Error("Failed to read html file", "file", filename, "error", err)
				c.String(500, "Failed to load %s", filename)
				return
			}
			c.Data(200, "text/html; charset=utf-8", data)
		}
	}

	// 首页
	s.router.GET("/", serveHTML("index.html"))
	// WebChat 页面
	s.router.GET("/webchat.html", serveHTML("webchat.html"))
	// Trace 页面
	s.router.GET("/trace.html", serveHTML("trace.html"))

	// 其他未匹配路由（SPA fallback）
	s.router.NoRoute(func(c *gin.Context) {
		// 如果是 API 路由但未匹配，返回 404
		path := c.Request.URL.Path
		if len(path) >= 4 && path[:4] == "/api" {
			c.JSON(404, gin.H{
				"error": gin.H{
					"code":    "not_found",
					"message": "API endpoint not found",
				},
			})
			return
		}
		if len(path) >= 3 && path[:3] == "/v1" {
			c.JSON(404, gin.H{
				"error": gin.H{
					"code":    "not_found",
					"message": "API endpoint not found",
				},
			})
			return
		}

		// 其他路由返回首页（SPA）
		data, err := fs.ReadFile(staticFS, "index.html")
		if err != nil {
			logger.Error("Failed to read index.html", "error", err)
			c.String(500, "Failed to load index.html")
			return
		}
		c.Data(200, "text/html; charset=utf-8", data)
	})
}

// Start 启动服务器
func (s *Server) Start() error {
	// 确定端口
	port := 8080
	host := "127.0.0.1"

	if s.config.WebUI != nil {
		if s.config.WebUI.Port > 0 {
			port = s.config.WebUI.Port
		}
		if s.config.WebUI.Host != "" {
			host = s.config.WebUI.Host
		}
	}

	// API 配置可以覆盖端口
	if s.config.API != nil && s.config.API.Port > 0 {
		port = s.config.API.Port
	}
	if s.config.API != nil && s.config.API.Host != "" {
		host = s.config.API.Host
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	logger.Info("Server starting", "addr", addr)
	return s.httpServer.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	logger.Info("Server stopping")
	return s.httpServer.Shutdown(ctx)
}

// Address 获取服务器地址
func (s *Server) Address() string {
	if s.httpServer == nil {
		return ""
	}
	return fmt.Sprintf("http://%s", s.httpServer.Addr)
}

// Helper function for error responses
func respondError(c *gin.Context, code int, errCode string, message string) {
	c.JSON(code, gin.H{
		"error": gin.H{
			"code":    errCode,
			"message": message,
		},
	})
}

// Helper function for success responses
func respondSuccess(c *gin.Context, data interface{}) {
	c.JSON(200, data)
}
