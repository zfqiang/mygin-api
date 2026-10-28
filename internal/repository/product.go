package repository

import (
	"context"
	"fmt"
	"mygin-api/internal/model"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"
	"mygin-api/test/fakermodel"
	"time"

	"github.com/go-faker/faker/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type ProductRepository struct {
	appCfg *config.AppConfig
}

func NewProductRepository(appCfg *config.AppConfig) *ProductRepository {
	return &ProductRepository{appCfg: appCfg}
}

func (r *ProductRepository) DoSecKillSave(ctx context.Context, productID int64, userID int64, quantity int32) error {
	//抢购成功，记录到数据库
	r.appCfg.Logger.Info("order success", "productID", productID, "userID", userID, "quantity", quantity)
	//扣除商品库存
	// 1. 获取当前商品状态（用于日志或后续逻辑，非必须用于锁控制，但常用于获取版本号）
	var product model.Product
	if err := r.appCfg.DB.Table(model.ProductTableName).
		Where("ID = ?", productID).
		First(&product).Error; err != nil {
		r.appCfg.Logger.Error("get product failed:", "error", err)
		return fmt.Errorf("get product failed:%v", err)
	}

	// 2. 执行乐观锁更新
	// 关键：利用 WHERE 条件保证原子性
	// 方式 A：仅基于库存数量（推荐用于纯库存扣减场景，性能更高）
	result := r.appCfg.DB.Table(model.ProductTableName).
		Where("ID = ? AND stock >= ?", productID, quantity).
		Update("stock", gorm.Expr("stock - ?", quantity))
	if result.Error != nil {
		r.appCfg.Logger.Error("update product stock failed:", "error", result.Error)
		return fmt.Errorf("update product stock failed:%v", result.Error)
	}

	// 方式 B：基于版本号（适用于需要防止任意字段脏写的场景）
	// result := tx.Model(&Product{}).
	// 	Where("ID = ? AND version = ?", productID, product.Version).
	// 	Updates(map[string]interface{}{
	// 		"stock":   gorm.Expr("stock - ?", quantity),
	// 		"version": gorm.Expr("version + 1"),
	// 	})

	//计算总价格
	TotalPrice := product.Price.Mul(utils.Int32ToDecimal(quantity))
	currentAt := time.Now()

	//记录到订单报及附表
	var order fakermodel.Order
	faker.FakeData(&order)
	order.PayAmount = TotalPrice
	order.TotalAmount = TotalPrice
	order.CreateTime = currentAt
	if err := r.appCfg.DB.Create(&order).Error; err != nil {
		r.appCfg.Logger.Error("create order failed:", "error", err)
		return fmt.Errorf("create order failed:%v", err)
	}

	var orderItem = fakermodel.OrderItem{
		OrderNo:      order.OrderNo,
		ProductID:    productID,
		ProductName:  product.Name,
		ProductImage: product.MainImage,
		CurrentPrice: product.Price,
		Quantity:     quantity,
		TotalPrice:   TotalPrice,
		CreatedAt:    currentAt,
		UpdatedAt:    currentAt,
		Version:      1,
	}
	if err := r.appCfg.DB.Create(&orderItem).Error; err != nil {
		r.appCfg.Logger.Error("create order item failed:", "error", err)
		return fmt.Errorf("create order item failed:%v", err)
	}

	//推送到超时调度
	timeout := float64(time.Now().Add(20 * time.Second).UnixNano())
	r.appCfg.RedisClient.ZAdd(ctx, utils.OrderTimeoutKey, redis.Z{
		Score:  timeout,
		Member: order.OrderNo,
	})
	return nil
}
