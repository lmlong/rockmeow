// Package middleware HTTP 中间件
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware Token 认证中间件
func AuthMiddleware(validTokens []string) gin.HandlerFunc {
	tokenSet := make(map[string]bool)
	for _, t := range validTokens {
		tokenSet[t] = true
	}

	return func(c *gin.Context) {
		// 跳过健康检查
		if c.Request.URL.Path == "/v1/health" {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(401, gin.H{
				"error": gin.H{
					"code":    "unauthorized",
					"message": "Missing Authorization header",
				},
			})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if !tokenSet[token] {
			c.JSON(401, gin.H{
				"error": gin.H{
					"code":    "unauthorized",
					"message": "Invalid token",
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
