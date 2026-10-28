package api

import (
	"net/http"

	"mygin-api/internal/controller"
	"mygin-api/internal/middleware"
	"mygin-api/internal/service/auth"
	"mygin-api/pkg/config"

	"github.com/gin-gonic/gin"
)

type ApiRouter struct {
	appCfg *config.AppConfig
}

func NewApiRouter(appCfg *config.AppConfig) *ApiRouter {
	return &ApiRouter{
		appCfg: appCfg,
	}
}

func (r *ApiRouter) RegisterRoutes() {
	// 注册API路由
	r.RegisterAPIRoutes()
	// 注册活动路由
	r.RegisterActivityRoutes()
}

func (r *ApiRouter) RegisterAPIRoutes() {
	api := r.appCfg.Engine.Group("/api")
	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	api.GET("/me", middleware.JWTAuth(r.appCfg.Cfg.JWTSecret, r.appCfg.Cfg.JWTIssuer), func(c *gin.Context) {
		userID, _ := middleware.AuthenticatedUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})
	api.POST("/auth/login", auth.NewLoginHandler(r.appCfg).Login)
}

func (r *ApiRouter) RegisterActivityRoutes() {
	activity := r.appCfg.Engine.Group("/activity")
	activity.GET("/secKill/init", controller.NewProductController(r.appCfg).SecKillInit)
	activity.POST("/secKill/start", controller.NewProductController(r.appCfg).HandleSecKill)
}
