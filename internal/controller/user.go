package controller

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"mygin-api/internal/event"
	"mygin-api/internal/service"
	"mygin-api/internal/service/auth"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"
)

var phonePattern = regexp.MustCompile(`^\+?[1-9][0-9]{6,14}$`)

type RegisterParam struct {
	Username string `json:"username" binding:"required,max=64"`
	Email    string `json:"email" binding:"required,email,max=254"`
	Phone    string `json:"phone" binding:"required,max=20"`
	Password string `json:"password" binding:"required,min=8,max=72"`
}

type UserController struct {
	appCfg *config.AppConfig
}

func NewUserController(appCfg *config.AppConfig) *UserController {
	return &UserController{appCfg: appCfg}
}

// UserController 的 Register 方法处理用户注册请求
// 参数:
//   - c: gin.Context 请求上下文对象
func (uc *UserController) Register(c *gin.Context) {
	// 声明 RegisterParam 结构体变量用于接收请求参数
	var param RegisterParam
	// 使用 ShouldBindJSON 绑定请求体参数到结构体
	if err := c.ShouldBindJSON(&param); err != nil {
		utils.Fail(c, "注册参数无效")
		return
	}

	// 去除用户名、邮箱和手机号的前后空格
	param.Username = strings.TrimSpace(param.Username)
	param.Email = strings.TrimSpace(param.Email)
	param.Phone = strings.TrimSpace(param.Phone)
	// 检查用户名是否为空
	if param.Username == "" {
		utils.Fail(c, "用户名不能为空")
		return
	}
	// 解析并验证邮箱格式，同时验证手机号格式
	parsedEmail, err := mail.ParseAddress(param.Email)
	if err != nil || parsedEmail.Address != param.Email || !phonePattern.MatchString(param.Phone) {
		utils.Fail(c, "邮箱或手机号格式无效")
		return
	}

	// 检查用户名、邮箱或手机号是否已被注册
	var existing auth.User
	err = uc.appCfg.DB.Where("username = ?", param.Username).First(&existing).Error
	if err == nil {
		utils.Fail(c, "用户名已被注册")
		return
	}
	// 如果错误不是记录未找到，则记录日志并返回错误
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		uc.appCfg.Logger.Errorf("注册时检查用户信息失败 | error=%v", err)
		utils.Fail(c, "注册失败")
		return
	}

	// 对密码进行哈希加密
	passwordHash, err := auth.HashPassword(param.Password)
	if err != nil {
		uc.appCfg.Logger.Errorf("注册时密码加密失败 | error=%v", err)
		utils.Fail(c, "注册失败")
		return
	}
	// 创建用户实例
	email := param.Email
	phone := param.Phone
	user := auth.User{
		Username:     param.Username,
		Email:        email,
		Phone:        phone,
		PasswordHash: passwordHash,
	}
	// 使用事务创建用户
	if err := uc.appCfg.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return fmt.Errorf("创建注册用户: %w", err)
		}
		return nil
	}); err != nil {
		uc.appCfg.Logger.Errorf("用户注册事务失败 | username=%s | error=%v", param.Username, err)
		utils.Fail(c, "注册失败，请稍后重试")
		return
	}

	// 创建注册事件
	registrationEvent := event.RegisterEvent{
		Type:      event.UserRegistered,
		UserID:    int64(user.ID),
		Username:  user.Username,
		Email:     param.Email,
		Phone:     param.Phone,
		Timestamp: time.Now().Unix(),
	}
	// 异步发布注册事件
	go func(registrationEvent event.RegisterEvent) {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := service.NewUserService(uc.appCfg).PublishRegistrationEvent(publishCtx, registrationEvent); err != nil {
			uc.appCfg.Logger.Errorf("异步发布用户注册任务失败 | user_id=%d | error=%v", registrationEvent.UserID, err)
		}
	}(registrationEvent)

	// 返回注册成功响应
	utils.Success(c, "注册成功")
}
