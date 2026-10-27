package health

import (
	"context"
	"mygin-api/internal/config"
	"mygin-api/internal/middleware"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type HealthRouter struct {
	appCfg *config.AppConfig
}

func NewHealth(appCfg *config.AppConfig) *HealthRouter {
	return &HealthRouter{
		appCfg: appCfg,
	}
}

// RegisterHealthRoutes 注册健康检查相关的路由
// 该函数用于设置服务的健康检查和就绪检查端点
// RegisterHealthRoutes 注册健康检查相关的路由
// 这个方法为健康检查和就绪检查提供了HTTP端点
func (r *HealthRouter) RegisterHealthRoutes() {
	// 注册健康检查端点 /healthz
	// 这个端点简单地返回服务状态为"ok"
	h := r.appCfg.Engine.Group("/healthz").
		Use(middleware.RateLimitMiddleware(rate.Limit(r.appCfg.Cfg.RateLimitIP), r.appCfg.Cfg.RateBurstIP, r.appCfg.Logger))
	h.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// 注册就绪检查端点 /readyz
	// 这个端点会检查数据库和Redis的连接状态，确保服务完全就绪
	h.GET("/readyz", func(c *gin.Context) {
		// 创建一个2秒超时的上下文
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		// 获取底层的sql.DB实例
		sqlDB, err := r.appCfg.DB.DB()
		if err != nil {
			// 记录MySQL检查失败的日志
			r.appCfg.Logger.Error("readiness check failed", "dependency", "mysql", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		// 检查MySQL连接是否可用
		if err := sqlDB.PingContext(ctx); err != nil {
			// 记录MySQL连接检查失败的日志
			r.appCfg.Logger.Error("readiness check failed", "dependency", "mysql", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		// 检查Redis连接是否可用
		if err := r.appCfg.RedisClient.Ping(ctx).Err(); err != nil {
			// 记录Redis检查失败的日志
			r.appCfg.Logger.Error("readiness check failed", "dependency", "redis", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		// 所有检查通过，返回服务就绪状态
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}
