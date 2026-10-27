package router

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"mygin-api/internal/config"
	"mygin-api/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func New(cfg config.Config, db *gorm.DB, redisClient *redis.Client, logger *slog.Logger) http.Handler {
	gin.SetMode(cfg.Mode)
	engine := gin.New()
	engine.Use(
		middleware.RequestLogger(logger),
		middleware.Recovery(logger),
		middleware.CORS(cfg.CORSOrigins),
		middleware.RateLimit(redisClient, cfg.RateLimit, cfg.RateWindow, logger),
	)
	engine.SetTrustedProxies(nil)

	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	engine.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		sqlDB, err := db.DB()
		if err != nil {
			logger.Error("readiness check failed", "dependency", "mysql", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		if err := sqlDB.PingContext(ctx); err != nil {
			logger.Error("readiness check failed", "dependency", "mysql", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Error("readiness check failed", "dependency", "redis", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	api := engine.Group("/api/v1")
	api.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	api.GET("/me", middleware.JWTAuth(cfg.JWTSecret, cfg.JWTIssuer), func(c *gin.Context) {
		userID, _ := middleware.AuthenticatedUserID(c)
		c.JSON(http.StatusOK, gin.H{"user_id": userID})
	})

	return engine
}
