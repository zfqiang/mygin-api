package middleware

import (
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"mygin-api/pkg/utils"
)

func LocalhostOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取客户端真实 IP
		// 注意：如果服务前有 Nginx/代理，需确保配置了 ForwardedByClientIP
		clientIP := c.ClientIP()

		// 2. 解析 IP
		ip := net.ParseIP(clientIP)
		if ip == nil {
			utils.AbortFailure(c, http.StatusForbidden, "Invalid IP address")
			return
		}

		// 3. 判断是否为本地回环地址 (127.0.0.1 或 ::1)
		if !ip.IsLoopback() {
			utils.AbortFailure(c, http.StatusForbidden, "Access denied: Only localhost is allowed")
			return
		}

		// 4. 如果是 localhost，继续处理后续逻辑
		c.Next()
	}

}
