package service

import (
	"context"
	"fmt"

	"mygin-api/internal/event"
	"mygin-api/pkg/config"
)

type UserService struct {
	appCfg *config.AppConfig
}

func NewUserService(appCfg *config.AppConfig) *UserService {
	return &UserService{appCfg: appCfg}
}

func (uc *UserService) PublishRegistrationEvent(ctx context.Context, registrationEvent event.RegisterEvent) error {
	if registrationEvent.UserID <= 0 {
		return fmt.Errorf("注册事件用户 ID 无效")
	}
	if err := uc.appCfg.RabbitMQ.PublishJSON(ctx, event.UserRegisteredQueue, registrationEvent); err != nil {
		return fmt.Errorf("发布用户注册事件: %w", err)
	}
	return nil
}
