package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/config"
)

// CORSMiddleware CORS 中间件
func CORSMiddleware(cfg *config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowedOrigin := "*"

		if cfg != nil && len(cfg.AllowedOrigins) > 0 {
			for _, o := range cfg.AllowedOrigins {
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

		c.Header("Access-Control-Allow-Origin", allowedOrigin)

		// 设置允许的方法
		allowedMethods := "GET, POST, PUT, DELETE, OPTIONS"
		if cfg != nil && cfg.AllowedMethods != "" {
			allowedMethods = cfg.AllowedMethods
		}
		c.Header("Access-Control-Allow-Methods", allowedMethods)

		// 设置允许的头
		allowedHeaders := "Content-Type, Authorization"
		if cfg != nil && cfg.AllowedHeaders != "" {
			allowedHeaders = cfg.AllowedHeaders
		}
		c.Header("Access-Control-Allow-Headers", allowedHeaders)

		// 设置是否允许凭证
		if cfg != nil && cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
