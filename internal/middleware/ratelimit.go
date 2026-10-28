package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// 使用示例:
// r.Use(GlobalRateLimitMiddleware(10, 20)) // 限制全局限速 10 QPS，突发容忍 20
// GlobalRateLimitMiddleware 创建全局限流中间件
// limit: 每秒允许的请求数 (QPS)
// burst: 允许的最大突发请求数
func GlobalRateLimitMiddleware(limit rate.Limit, burst int, logger *zap.SugaredLogger) gin.HandlerFunc {
	limiter := rate.NewLimiter(limit, burst)

	return func(c *gin.Context) {
		// Allow() 是非阻塞的，如果没有令牌立即返回 false
		// Wait() 是阻塞的，直到有令牌为止
		if !limiter.Allow() {
			logger.Info("too many requests")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
