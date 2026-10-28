package fakermodel

import (
	"time"

	"github.com/shopspring/decimal"
)

type Product struct {
	// ID         int64           `gorm:"column:id;primaryKey;autoIncrement:true;comment:商品ID" json:"id"`                      // 商品ID
	Name       string          `gorm:"column:name;not null;comment:商品名称" json:"name"`                                       // 商品名称
	SubTitle   string          `gorm:"column:sub_title;comment:副标题/卖点" json:"sub_title"`                                    // 副标题/卖点
	MainImage  string          `gorm:"column:main_image;comment:主图URL" json:"main_image"`                                   // 主图URL
	CategoryID int64           `gorm:"column:category_id;not null;comment:分类ID" json:"category_id"`                         // 分类ID
	Price      decimal.Decimal `gorm:"column:price;not null;default:0.00;comment:当前销售价格" json:"price"`                      // 当前销售价格
	Stock      int32           `gorm:"column:stock;not null;comment:库存数量" json:"stock"`                                     // 库存数量
	Status     int32           `gorm:"column:status;not null;default:1;comment:状态: 1-在售, 2-下架, 3-删除" json:"status"`         // 状态: 1-在售, 2-下架, 3-删除
	CreatedAt  time.Time       `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"` // 创建时间
	UpdatedAt  time.Time       `gorm:"column:updated_at;not null;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"` // 更新时间
}
