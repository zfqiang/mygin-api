package utils

import "fmt"

const (

	// 订单超时key
	OrderTimeoutKey = "order:timeout"

	// 秒杀抢购的key
	ProductSeckillKey = "seckill:stock"

	// 秒杀抢购成功的key
	UserSeckillKey = "seckill:success"
)

// GetSeckillKey 生成秒杀活动的键名
// 参数:
//   - productID: 商品ID，用于标识具体的秒杀商品
// 返回值:
//   - string: 返回拼接后的秒杀键名，格式为"SeckillKey:商品ID"
func GetProductSeckillKey(productID int64) string {
	return fmt.Sprintf("%s:%d", ProductSeckillKey, productID) // 使用fmt.Sprintf将SeckillKey和id拼接成字符串返回
}

// GetSeckillSuccessKey 根据商品ID生成秒杀成功的键名
// @param productID 商品ID
// @return 返回拼接后的秒杀成功键名
func GetUserSeckillKey(productID int64) string {
	// 使用fmt.Sprintf将SeckillSuccessKey和id拼接成字符串返回
	return fmt.Sprintf("%s:%d", UserSeckillKey, productID) // 使用fmt.Sprintf将SeckillSuccessKey和id拼接成字符串返回
}
