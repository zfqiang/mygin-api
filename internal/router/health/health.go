package health

import (
	"context"
	"mygin-api/internal/middleware"
	"mygin-api/pkg/config"
	"net/http"
	"time"

	"github.com/gin-contrib/pprof"
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

func (r *HealthRouter) RegisterRoutes() {
	// 注册健康检查路由
	r.registerHealthRoutes()
	// 注册pprof路由
	// r.registerPprofRoutes()
}

// RegisterHealthRoutes 注册健康检查相关的路由
// 该函数用于设置服务的健康检查和就绪检查端点
// RegisterHealthRoutes 注册健康检查相关的路由
// 这个方法为健康检查和就绪检查提供了HTTP端点
func (r *HealthRouter) registerHealthRoutes() {
	// 注册健康检查端点 /healthz
	// 这个端点简单地返回服务状态为"ok"
	h := r.appCfg.Engine.Group("/healthz").
		Use(middleware.RateLimitMiddleware(rate.Limit(r.appCfg.Cfg.RateLimitIP), r.appCfg.Cfg.RateBurstIP, r.appCfg.Logger)).
		Use(middleware.LocalhostOnlyMiddleware())
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
			r.appCfg.Logger.Errorf("就绪检查失败 | dependency=MySQL | error=%v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		// 检查MySQL连接是否可用
		if err := sqlDB.PingContext(ctx); err != nil {
			// 记录MySQL连接检查失败的日志
			r.appCfg.Logger.Errorf("就绪检查失败 | dependency=MySQL | error=%v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		// 检查Redis连接是否可用
		if err := r.appCfg.RedisClient.Ping(ctx).Err(); err != nil {
			// 记录Redis检查失败的日志
			r.appCfg.Logger.Errorf("就绪检查失败 | dependency=Redis | error=%v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		// 所有检查通过，返回服务就绪状态
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
}

func (r *HealthRouter) registerPprofRoutes() {
	//创建受保护的路由组 -- 正式建议这种
	// debugGroup := r.appCfg.Engine.Group("/debug", func(c *gin.Context) {
	// 	// 简单的 Token 验证示例
	// 	if c.GetHeader("Authorization") != "Bearer secret-token" {
	// 		c.AbortWithStatus(http.StatusForbidden)
	// 		return
	// 	}
	// 	c.Next()
	// })

	// 在受保护组中注册 pprof
	// pprof.RouteRegister(debugGroup, "pprof")
	r.appCfg.Engine.Group("/debug").Use(middleware.LocalhostOnlyMiddleware())
	pprof.RouteRegister(r.appCfg.Engine, "/pprof")

	/**
	采集 30 秒的 CPU 采样数据，找出最耗时的函数。
	# 采集并进入交互式终端
	go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

	# 进入终端后输入 web 生成 SVG 火焰图（需安装 graphviz）
	(pprof) web
	# 或者输入 top 查看耗时最高的前几个函数
	(pprof) top 10
	*/
}
