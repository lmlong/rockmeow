package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lingguard/internal/config"
)

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(cfg *config.RateLimitConfig) gin.HandlerFunc {
	type client struct {
		count    int
		lastSeen time.Time
	}

	var mu sync.Mutex
	clients := make(map[string]*client)

	// 清理过期记录
	go func() {
		for range time.Tick(time.Minute) {
			mu.Lock()
			for ip, c := range clients {
				if time.Since(c.lastSeen) > time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	limit := cfg.RequestsPer
	if limit <= 0 {
		limit = 60
	}

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		cl, exists := clients[ip]
		if !exists {
			cl = &client{count: 0, lastSeen: time.Now()}
			clients[ip] = cl
		}

		if cl.count >= limit {
			mu.Unlock()
			c.JSON(429, gin.H{
				"error": gin.H{
					"code":    "rate_limit_exceeded",
					"message": "Too many requests",
				},
			})
			c.Abort()
			return
		}

		cl.count++
		cl.lastSeen = time.Now()
		mu.Unlock()

		c.Next()
	}
}
