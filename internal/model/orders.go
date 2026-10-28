package model

import (
	"time"

	"mygin-api/pkg/config"
)

const (
	OrdersTableName = "orders"
)

type OrderTimeoutModel struct {
	ID         int       `json:"id"`
	OrderNo    string    `json:"order_no"`
	CancelTime time.Time `json:"cancel_time"`
	Status     int8      `json:"status"`
	Version    int64     `json:"version"`
}

type OrderModel struct {
	appCfg *config.AppConfig
}
