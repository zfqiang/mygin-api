package api

import (
	"net/http"

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
}

func (r *ApiRouter) RegisterAPIRoutes() {
	api := r.appCfg.Engine.Group("/api/v1")
	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	api.GET("/me", middleware.JWTAuth(r.appCfg.Cfg.JWTSecret, r.appCfg.Cfg.JWTIssuer), func(c *gin.Context) {
		userID, _ := middleware.AuthenticatedUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})
	api.POST("/auth/login", auth.NewLoginHandler(r.appCfg).Login)

}
