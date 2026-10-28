package router

import (
	"net/http"

	"mygin-api/internal/middleware"
	"mygin-api/internal/router/api"
	"mygin-api/internal/router/gen"
	"mygin-api/internal/router/health"
	"mygin-api/pkg/config"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type RouterCfg struct {
}

func NewRouter() *RouterCfg {
	return &RouterCfg{}
}

func (router *RouterCfg) New(appCfg *config.AppConfig) http.Handler {

	gin.SetMode(appCfg.Cfg.Mode)
	engine := gin.New()

	// 中间件, 全局注册
	engine.Use(
		middleware.RequestLogger(appCfg.Logger), // 请求日志
		middleware.Recovery(appCfg.Logger),      // 错误恢复
		middleware.CORS(appCfg.Cfg.CORSOrigins), // 跨域
		middleware.GlobalRateLimitMiddleware(rate.Limit(appCfg.Cfg.RateLimit), appCfg.Cfg.RateBurst, appCfg.Logger), // 限流
	)
	engine.SetTrustedProxies(nil) // 不信任任何代理
	appCfg.Engine = engine

	// 注册健康检查相关路由
	health := health.NewHealth(appCfg)
	health.RegisterRoutes()

	// 注册API路由
	api := api.NewApiRouter(appCfg)
	api.RegisterRoutes()

	// 注册虚拟数据路由
	gen := gen.NewGenRouter(appCfg)
	gen.RegisterRoutes()

	return engine
}
