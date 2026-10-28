package auth

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"mygin-api/pkg/config"
	"mygin-api/internal/middleware"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	ID           uint64 `gorm:"primaryKey"`
	Username     string `gorm:"size:64;not null;uniqueIndex"`
	PasswordHash string `gorm:"size:60;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Migrate 是一个数据库迁移函数，用于自动创建或更新数据库表结构
// 参数:
//
//	db - 指向 GORM 数据库对象的指针，用于执行数据库操作
//
// 返回值:
//
//	error - 如果迁移过程中发生错误，返回包含详细信息的错误；否则返回 nil
func Migrate(db *gorm.DB) error {
	// 使用 AutoMigrate 方法自动迁移 User 结构体对应的数据库表
	// 如果迁移失败，则返回一个包含错误信息的格式化错误
	if err := db.AutoMigrate(&User{}); err != nil {
		return fmt.Errorf("migrate users table: %w", err)
	}
	// 迁移成功，返回 nil 表示无错误
	return nil
}

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

type LoginHandler struct {
	db     *gorm.DB
	secret []byte
	issuer string
	ttl    time.Duration
	logger *slog.Logger
}

func NewLoginHandler(appCfg *config.AppConfig) *LoginHandler {
	return &LoginHandler{
		db:     appCfg.DB,
		secret: appCfg.Cfg.JWTSecret,
		issuer: appCfg.Cfg.JWTIssuer,
		ttl:    appCfg.Cfg.JWTTTL,
		logger: appCfg.Logger,
	}
}

type loginRequest struct {
	Username string `json:"username" binding:"required,max=64"`
	Password string `json:"password" binding:"required"`
}

func (h *LoginHandler) Login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if len(request.Password) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var user User
	err := h.db.WithContext(c.Request.Context()).
		Where("username = ?", request.Username).
		First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	if err != nil {
		h.logger.Error("login user lookup failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}

	token, err := middleware.IssueToken(user.ID, h.secret, h.issuer, h.ttl)
	if err != nil {
		h.logger.Error("login token creation failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_in":   int64(h.ttl.Seconds()),
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}
