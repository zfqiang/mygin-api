package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"mygin-api/pkg/logging"
)

func RequestLogger(logger *logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		logger.LogF("HTTP 请求完成 | method=%s | path=%s | status=%d | duration=%s | client_ip=%s",
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Since(start),
			c.ClientIP(),
		)
	}
}
