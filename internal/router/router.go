package router

import (
	"net/http"

	"mygin-api/internal/config"
	"mygin-api/internal/middleware"
	"mygin-api/internal/router/api"
	"mygin-api/internal/router/health"

	"github.com/gin-gonic/gin"
)

type RouterCfg struct {
}

func NewRouter() *RouterCfg {
	return &RouterCfg{}
}

func (router *RouterCfg) New(appCfg *config.AppConfig) http.Handler {

	gin.SetMode(appCfg.Cfg.Mode)
	engine := gin.New()

	// 中间件
	engine.Use(
		middleware.RequestLogger(appCfg.Logger), // 请求日志
		middleware.Recovery(appCfg.Logger),      // 错误恢复
		middleware.CORS(appCfg.Cfg.CORSOrigins), // 跨域
		middleware.RateLimit(appCfg.RedisClient, appCfg.Cfg.RateLimit, appCfg.Cfg.RateWindow, appCfg.Logger), // 限流
	)
	engine.SetTrustedProxies(nil) // 不信任任何代理
	appCfg.Engine = engine

	// 注册健康检查路由
	health := health.NewHealth(appCfg)
	health.RegisterHealthRoutes()

	// 注册API路由
	api := api.NewApiRouter(appCfg)
	api.RegisterAPIRoutes()

	return engine
}
