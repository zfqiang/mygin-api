package utils

const (

	// 1 在售 2 下架  3 删除
	StatusOnSale  = 1
	StatusOffSale = 2
	StatusDeleted = 3

	// 1 已删除 0 未删除
	IsDeleted  = 1
	NotDeleted = 0

	OrderStatusCreated   = 0 // 已创建
	OrderStatusPaid      = 1 // 已支付
	OrderStatusShipped   = 2 // 已发货
	OrderStatusCompleted = 3 // 已完成
	OrderStatusCancelled = 4 // 已取消
	OrderStatusAfterSale = 5 // 售后中

	SourceApp = 1 // APP
	SourcePC  = 2 // PC
	SourceH5  = 3 // H5

	// 标准日期时间格式: 2026-10-28 14:30:00
	LayoutDateTime = "2006-01-02 15:04:05"

	// 纯日期格式: 2026-10-28
	LayoutDateOnly = "2006-01-02"

	// 纯时间格式: 14:30:00
	LayoutTimeOnly = "15:04:05"

	// 紧凑格式: 20261028143000
	LayoutCompact = "20060102150405"
)
