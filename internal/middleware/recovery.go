package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"mygin-api/pkg/logging"
)

func Recovery(logger *logging.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		logger.Errorf("HTTP 请求处理中发生 panic | panic=%v | method=%s | path=%s | stack=%s",
			recovered,
			c.Request.Method,
			c.Request.URL.Path,
			string(debug.Stack()),
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	})
}
