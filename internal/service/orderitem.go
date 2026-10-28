package service

import (
	"mygin-api/pkg/config"
)

type OrderItemService struct {
	appCfg *config.AppConfig
}

func NewOrderItemService(appCfg *config.AppConfig) *OrderItemService {
	return &OrderItemService{appCfg: appCfg}
}
