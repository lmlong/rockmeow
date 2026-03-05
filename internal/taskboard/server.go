package taskboard

import (
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/lingguard/internal/config"
	"github.com/lingguard/internal/taskboard/web"
	"github.com/lingguard/internal/trace"
	"github.com/lingguard/pkg/logger"
)

// Server Web UI 服务器
type Server struct {
	host         string
	port         int
	handler      *HTTPHandler
	traceHandler *TraceHTTPHandler
	server       *http.Server
	corsConfig   *config.CORSConfig

	// WebChat WebSocket 处理器
	webSocketHandler WebSocketHandler

	// WebChat API 处理器 (用于会话持久化)
	webChatAPIHandler WebChatAPIHandler
}

// WebSocketHandler WebSocket 处理器接口
type WebSocketHandler interface {
	HandleWebSocket(conn *websocket.Conn, sessionID string)
}

// WebChatAPIHandler WebChat API 处理器接口
type WebChatAPIHandler interface {
	RegisterRoutes(mux *http.ServeMux)
}

// NewServer 创建 Web UI 服务器
func NewServer(host string, port int, service *Service) *Server {
	return &Server{
		host:    host,
		port:    port,
		handler: NewHTTPHandler(service),
	}
}

// NewServerWithTrace 创建带追踪功能的 Web UI 服务器
func NewServerWithTrace(host string, port int, service *Service, traceService *trace.Service) *Server {
	return &Server{
		host:         host,
		port:         port,
		handler:      NewHTTPHandler(service),
		traceHandler: NewTraceHTTPHandler(traceService),
	}
}

// NewServerWithConfig 创建带配置的 Web UI 服务器
func NewServerWithConfig(host string, port int, service *Service, corsConfig *config.CORSConfig) *Server {
	return &Server{
		host:       host,
		port:       port,
		handler:    NewHTTPHandler(service),
		corsConfig: corsConfig,
	}
}

// NewServerWithTraceAndConfig 创建带追踪功能和配置的 Web UI 服务器
func NewServerWithTraceAndConfig(host string, port int, service *Service, traceService *trace.Service, corsConfig *config.CORSConfig) *Server {
	return &Server{
		host:         host,
		port:         port,
		handler:      NewHTTPHandler(service),
		traceHandler: NewTraceHTTPHandler(traceService),
		corsConfig:   corsConfig,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// 注册 TaskBoard API 路由
	s.handler.RegisterRoutes(mux)

	// 注册 Trace API 路由（如果启用）
	if s.traceHandler != nil {
		s.traceHandler.RegisterRoutes(mux)
	}

	// 注册 WebChat API 路由（如果启用）
	if s.webChatAPIHandler != nil {
		s.webChatAPIHandler.RegisterRoutes(mux)
	}

	// 静态文件服务
	staticFS, err := fs.Sub(web.StaticFiles, "static")
	if err != nil {
		return fmt.Errorf("sub static fs: %w", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	// 首页路由
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.StripPrefix("/", fileServer).ServeHTTP(w, r)
			return
		}
		// 其他静态文件
		http.StripPrefix("/", fileServer).ServeHTTP(w, r)
	})

	// WebSocket 路由（用于 WebChat）
	if s.webSocketHandler != nil {
		mux.HandleFunc("/ws/chat", s.handleWebSocket)
		logger.Info("WebSocket route registered", "path", "/ws/chat")
	}

	// CORS 中间件
	handler := s.corsMiddleware(mux)

	// Request ID 中间件
	handler = s.requestIDMiddleware(handler)
	// 创建服务器
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Info("Web UI server starting", "addr", addr)

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Web UI server error", "error", err)
		}
	}()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	if s.server == nil {
		return nil
	}

	logger.Info("Web UI server stopping")
	return s.server.Close()
}

// Address 获取服务器地址
func (s *Server) Address() string {
	return fmt.Sprintf("http://%s:%d", s.host, s.port)
}

// SetCronDeleter 设置 cron 删除器
func (s *Server) SetCronDeleter(deleter CronDeleter) {
	s.handler.SetCronDeleter(deleter)
}

// SetWebSocketHandler 设置 WebSocket 处理器（用于 WebChat）
func (s *Server) SetWebSocketHandler(handler WebSocketHandler) {
	s.webSocketHandler = handler
}

// SetWebChatAPIHandler 设置 WebChat API 处理器
func (s *Server) SetWebChatAPIHandler(handler WebChatAPIHandler) {
	s.webChatAPIHandler = handler
}

// websocketUpgrader WebSocket 升级器
var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，CORS 由 corsMiddleware 处理
	},
}

// handleWebSocket 处理 WebSocket 连接请求
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.webSocketHandler == nil {
		http.Error(w, "WebSocket not enabled", http.StatusServiceUnavailable)
		return
	}

	// 从查询参数获取 session ID
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// 升级为 WebSocket
	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Warn("WebSocket upgrade failed", "error", err)
		return
	}

	logger.Info("WebSocket connection established", "sessionId", sessionID)

	// 调用处理器
	s.webSocketHandler.HandleWebSocket(conn, sessionID)
}

// corsMiddleware CORS 中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 获取允许的源
		allowedOrigin := "*"
		if s.corsConfig != nil && len(s.corsConfig.AllowedOrigins) > 0 {
			origin := r.Header.Get("Origin")
			for _, o := range s.corsConfig.AllowedOrigins {
				if o == "*" || o == origin {
					if o == "*" {
						allowedOrigin = "*"
					} else {
						allowedOrigin = origin
					}
					break
				}
			}
		}
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)

		// 设置允许的方法
		allowedMethods := "GET, POST, PUT, DELETE, OPTIONS"
		if s.corsConfig != nil && s.corsConfig.AllowedMethods != "" {
			allowedMethods = s.corsConfig.AllowedMethods
		}
		w.Header().Set("Access-Control-Allow-Methods", allowedMethods)

		// 设置允许的头
		allowedHeaders := "Content-Type, Authorization"
		if s.corsConfig != nil && s.corsConfig.AllowedHeaders != "" {
			allowedHeaders = s.corsConfig.AllowedHeaders
		}
		w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)

		// 设置是否允许凭证
		if s.corsConfig != nil && s.corsConfig.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// requestIDMiddleware Request ID 中间件
func (s *Server) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 获取或生成 request id
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()[:8]
		}

		// 设置响应头
		w.Header().Set("X-Request-ID", requestID)

		next.ServeHTTP(w, r)
	})
}
