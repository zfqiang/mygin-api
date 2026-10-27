package models

import "time"

const (
	StatusPaid      = 1 // 已支付
	StatusShipping  = 2 // 已发货
	StatusCompleted = 3 // 已完成
	StatusCancelled = 4 // 已取消
	StatusAfterSale = 5 // 售后中
	OrdersTableName = "orders"
)

type OrderTimeoutModel struct {
	ID         int       `json:"id"`
	OrderNo    string    `json:"order_no"`
	CancelTime time.Time `json:"cancel_time"`
	Status     int8      `json:"status"`
	Version    int64     `json:"version"`
}

type Order struct {
}
