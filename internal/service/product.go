package service

import (
	"context"
	"fmt"
	"mygin-api/internal/model"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"
	"mygin-api/test/fakermodel"
	"runtime/debug"
	"time"

	"github.com/go-faker/faker/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Lua 脚本，原子检查并扣除库存，返回抢购结果
// pscKey: 商品库存的key
// usckey: 用户抢购记录的key
// productID: 商品ID
// userID: 用户ID
// quantity: 抢购数量
// 返回值:	1: 下单成功 -1: 库存不足 -2: 抢购数量超过库存 -3: 重复下单
const luaScript = `
local pscKey = KEY[1]
local usckey = KEY[2]
local productID = tonumber(ARGV[1])
local userID = tonumber(ARGV[2])
local quantity = tonumber(ARGV[3])

-- 检查用户是否已经抢购过
if redis.call("SISMEMBER", usckey, userID) == 1 then
    return -3 -- 重复下单
end

-- 检查库存
local stock = tonumber(redis.call("GET", pscKey))
if stock <= 0 then
    return -1 -- 库存不足
end

-- 检查购买数量是否超过限制
if stock < quantity then
    return -2 -- 抢购数量超过库存
end

-- 扣减库存
redis.call("DECRBY", pscKey, quantity)

-- 记录用户购买记录
redis.call("SADD", usckey, userID)

return 1 -- 下单成功
`

type ProductService struct {
	appCfg *config.AppConfig
}

func NewProductService(appCfg *config.AppConfig) *ProductService {
	return &ProductService{appCfg: appCfg}
}

// 并发抢购商品
func (o *ProductService) DoSecKill(ctx context.Context, productID int64, userID int64, quantity int32) (int, error) {
	defer func() {
		if r := recover(); r != nil {
			o.appCfg.Logger.Error("panic recovered:", "error", r, "stack", debug.Stack)
		}
	}()

	// 执行lua脚本
	pscKey := utils.GetProductSeckillKey(productID)
	usckey := utils.GetUserSeckillKey(productID)
	resp, err := o.appCfg.RedisClient.Eval(ctx, luaScript, []string{pscKey, usckey}, productID, userID, quantity).Result()
	if err != nil {
		o.appCfg.Logger.Error("redis eval failed:", "error", err)
		return 0, err
	}
	switch resp.(int64) {
	case -1:
		o.appCfg.Logger.Error("stock not enough", "productID", productID, "userID", userID, "quantity", quantity)
		return -1, fmt.Errorf("stock not enough")
	case -2:
		o.appCfg.Logger.Error("buy quantity over stock", "productID", productID, "userID", userID, "quantity", quantity)
		return -2, fmt.Errorf("buy quantity over stock")
	case -3:
		o.appCfg.Logger.Error("repeat order", "productID", productID, "userID", userID, "quantity", quantity)
		return -3, fmt.Errorf("repeat order")
	}

	//抢购成功，记录到数据库
	o.appCfg.Logger.Info("order success", "productID", productID, "userID", userID, "quantity", quantity)
	//扣除商品库存
	// 1. 获取当前商品状态（用于日志或后续逻辑，非必须用于锁控制，但常用于获取版本号）
	var product model.Product
	if err := o.appCfg.DB.Table(model.ProductTableName).
		Where("ID = ?", productID).
		First(&product).Error; err != nil {
		o.appCfg.Logger.Error("get product failed:", "error", err)
		return 0, fmt.Errorf("get product failed:%v", err)
	}

	// 2. 执行乐观锁更新
	// 关键：利用 WHERE 条件保证原子性
	// 方式 A：仅基于库存数量（推荐用于纯库存扣减场景，性能更高）
	result := o.appCfg.DB.Table(model.ProductTableName).
		Where("ID = ? AND stock >= ?", productID, quantity).
		Update("stock", gorm.Expr("stock - ?", quantity))
	if result.Error != nil {
		o.appCfg.Logger.Error("update product stock failed:", "error", result.Error)
		return 0, fmt.Errorf("update product stock failed:%v", result.Error)
	}

	// 方式 B：基于版本号（适用于需要防止任意字段脏写的场景）
	// result := tx.Model(&Product{}).
	// 	Where("ID = ? AND version = ?", productID, product.Version).
	// 	Updates(map[string]interface{}{
	// 		"stock":   gorm.Expr("stock - ?", quantity),
	// 		"version": gorm.Expr("version + 1"),
	// 	})

	//记录到订单报及附表
	var order fakermodel.Order
	faker.FakeData(&order)
	if err := o.appCfg.DB.Create(&order).Error; err != nil {
		o.appCfg.Logger.Error("create order failed:", "error", err)
		return 0, fmt.Errorf("create order failed:%v", err)
	}

	//计算总价格
	TotalPrice := product.Price.Mul(utils.Int32ToDecimal(quantity))
	currentAt := time.Now()
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
	if err := o.appCfg.DB.Create(&orderItem).Error; err != nil {
		o.appCfg.Logger.Error("create order item failed:", "error", err)
		return 0, fmt.Errorf("create order item failed:%v", err)
	}

	//推送到超时调度
	timeout := float64(time.Now().Add(20 * time.Second).UnixNano())
	o.appCfg.RedisClient.ZAdd(ctx, utils.OrderTimeoutKey, redis.Z{
		Score:  timeout,
		Member: order.OrderNo,
	})

	return 1, nil
}
