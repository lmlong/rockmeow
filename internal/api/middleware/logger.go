package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingguard/pkg/logger"
)

// LoggerMiddleware 请求日志中间件
func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		logFields := []interface{}{
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency", latency.String(),
			"ip", c.ClientIP(),
		}

		if query != "" {
			logFields = append(logFields, "query", query)
		}

		if status >= 400 {
			logger.Warn("HTTP Request", logFields...)
		} else {
			logger.Debug("HTTP Request", logFields...)
		}
	}
}
