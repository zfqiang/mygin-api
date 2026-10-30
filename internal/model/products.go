package model

import (
	"mygin-api/pkg/config"

	"github.com/shopspring/decimal"
)

const (
	ProductTableName = "products"
)

type ProductModel struct {
	appCfg *config.AppConfig
}

type Product struct {
	ID         int             `json:"id"`
	Stock      string          `json:"stock"`
	Status     int8            `json:"status"`
	Version    int64           `json:"version"`
	Name       string          `json:"name"`        // 商品名称
	SubTitle   string          `json:"sub_title"`   // 副标题/卖点
	MainImage  string          `json:"main_image"`  // 主图URL
	CategoryID int64           `json:"category_id"` // 分类ID
	Price      decimal.Decimal `json:"price"`       // 当前销售价格
}
