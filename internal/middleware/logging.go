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

		logger.Info("HTTP 请求完成",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start),
			"client_ip", c.ClientIP(),
		)
	}
}
