package service

import (
	"context"
	"fmt"
	"mygin-api/internal/repository"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"
	"runtime/debug"
)

// Lua 脚本，原子检查并扣除库存，返回抢购结果
// pscKey: 商品库存的key
// usckey: 用户抢购记录的key
// productID: 商品ID
// userID: 用户ID
// quantity: 抢购数量
// 返回值:	1: 下单成功 -1: 库存不足 -2: 抢购数量超过库存 -3: 重复下单
const luaScript = `
local pscKey = KEYS[1]
local usckey = KEYS[2]
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
			o.appCfg.Logger.Errorf("执行商品秒杀时发生 panic | panic=%v | stack=%s", r, string(debug.Stack()))
		}
	}()

	// 执行lua脚本
	pscKey := utils.GetProductSeckillKey(productID)
	usckey := utils.GetUserSeckillKey(productID)
	resp, err := o.appCfg.RedisClient.Eval(ctx, luaScript, []string{pscKey, usckey}, productID, userID, quantity).Result()
	if err != nil {
		o.appCfg.Logger.Errorf("执行 Redis 秒杀脚本失败 | product_id=%d | user_id=%d | quantity=%d | error=%v",
			productID, userID, quantity, err)
		return 0, err
	}
	switch resp.(int64) {
	case -1:
		o.appCfg.Logger.Warn("商品库存不足",
			"product_id", productID,
			"user_id", userID,
			"quantity", quantity,
		)
		return -1, fmt.Errorf("商品库存不足")
	case -2:
		o.appCfg.Logger.Warn("购买数量超过剩余库存",
			"product_id", productID,
			"user_id", userID,
			"quantity", quantity,
		)
		return -2, fmt.Errorf("购买数量超过剩余库存")
	case -3:
		o.appCfg.Logger.Warn("用户重复提交秒杀请求",
			"product_id", productID,
			"user_id", userID,
			"quantity", quantity,
		)
		return -3, fmt.Errorf("用户重复提交秒杀请求")
	}

	err = repository.NewProductRepository(o.appCfg).DoSecKillSave(ctx, productID, userID, quantity)
	if err != nil {
		return 0, err
	}

	return 1, nil
}
