package taskboard

import (
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/lingguard/internal/taskboard/web"
	"github.com/lingguard/pkg/logger"
)

// Server Web UI 服务器
type Server struct {
	host    string
	port    int
	handler *HTTPHandler
	server  *http.Server
}

// NewServer 创建 Web UI 服务器
func NewServer(host string, port int, service *Service) *Server {
	return &Server{
		host:    host,
		port:    port,
		handler: NewHTTPHandler(service),
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// 注册 API 路由
	s.handler.RegisterRoutes(mux)

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

	// CORS 中间件
	handler := s.corsMiddleware(mux)

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

// corsMiddleware CORS 中间件
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
