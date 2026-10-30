package event

import (
	"context"
	"encoding/json"
	"fmt"
	"mygin-api/internal/service/auth"
	"mygin-api/pkg/config"
	"net/mail"
	"net/smtp"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"gorm.io/gorm/clause"
)

const UserRegistered = "user_registered"
const UserRegisteredQueue = "user.registered"

type RegisterEvent struct {
	Type      string `json:"type"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Timestamp int64  `json:"timestamp"`
}

type EventMQ struct {
	appCfg *config.AppConfig
}

func NewEvent(appCfg *config.AppConfig) *EventMQ {
	return &EventMQ{
		appCfg: appCfg,
	}
}

// ConsumeRegistrationEvents 是一个消费者方法，用于处理用户注册事件
// 它从RabbitMQ队列中消费注册事件，并执行后续处理逻辑
func (ent *EventMQ) ConsumeRegistrationEvents(ctx context.Context) error {
	handler := func(ctx context.Context, delivery amqp.Delivery) error {
		var registrationEvent RegisterEvent
		if err := json.Unmarshal(delivery.Body, &registrationEvent); err != nil {
			return fmt.Errorf("解析用户注册事件: %w", err)
		}
		if registrationEvent.Type != UserRegistered || registrationEvent.UserID <= 0 {
			return fmt.Errorf("无效的用户注册事件")
		}

		account := auth.UserPoints{UserID: uint64(registrationEvent.UserID), Balance: 0}
		if err := ent.appCfg.DB.WithContext(ctx).
			Clauses(clause.OnConflict{DoNothing: true}).
			Create(&account).Error; err != nil {
			return fmt.Errorf("初始化用户积分账户: %w", err)
		}
		ent.appCfg.Logger.WarnF("初始化用户积分账户处理完成 | user_id=%d", registrationEvent.UserID)
		if err := ent.sendWelcomeEmail(registrationEvent); err != nil {
			return err
		}

		ent.appCfg.Logger.WarnF("短信服务商尚未配置，跳过注册短信通知 | user_id=%d", registrationEvent.UserID)
		ent.appCfg.Logger.LogF("用户注册异步任务处理完成 | user_id=%d | points=0", registrationEvent.UserID)
		return nil
	}
	return ent.appCfg.RabbitMQ.Consume(ctx, UserRegisteredQueue, 10, handler)
}

func (ent *EventMQ) sendWelcomeEmail(registrationEvent RegisterEvent) error {
	emailCfg := ent.appCfg.Cfg.Email
	if emailCfg.Host == "" || emailCfg.Port == "" || emailCfg.User == "" || emailCfg.Password == "" {
		ent.appCfg.Logger.WarnF("SMTP 配置不完整，跳过欢迎邮件 | user_id=%d | email=%s",
			registrationEvent.UserID, registrationEvent.Email)
		return nil
	}
	parsedEmail, err := mail.ParseAddress(registrationEvent.Email)
	if err != nil || parsedEmail.Address != registrationEvent.Email {
		return fmt.Errorf("注册事件中的邮箱地址无效")
	}

	message := strings.Join([]string{
		"To: " + registrationEvent.Email,
		"From: " + emailCfg.User,
		"Subject: =?UTF-8?B?5qyi6L+O5rOo5YaM?=",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		fmt.Sprintf("%s，欢迎注册！您的账户已创建成功。", registrationEvent.Username),
	}, "\r\n")
	authenticator := smtp.PlainAuth("", emailCfg.User, emailCfg.Password, emailCfg.Host)
	if err := smtp.SendMail(
		emailCfg.Host+":"+emailCfg.Port,
		authenticator,
		emailCfg.User,
		[]string{registrationEvent.Email},
		[]byte(message),
	); err != nil {
		return fmt.Errorf("发送注册欢迎邮件: %w", err)
	}
	ent.appCfg.Logger.LogF("注册欢迎邮件已发送 | user_id=%d | email=%s",
		registrationEvent.UserID, registrationEvent.Email)
	return nil
}
