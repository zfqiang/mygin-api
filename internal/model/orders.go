package model

import (
	"time"

	"mygin-api/pkg/config"
)

const (
	StatusCreated   = 0 // 已创建
	StatusPaid      = 1 // 已支付
	StatusShipping  = 2 // 已发货
	StatusCompleted = 3 // 已完成
	StatusCancelled = 4 // 已取消
	StatusAfterSale = 5 // 售后中

	SourceApp = 1 // APP
	SourcePC  = 2 // PC
	SourceH5  = 3 // H5

	IsDeleted  = 1 // 已删除
	NotDeleted = 0 // 未删除

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
