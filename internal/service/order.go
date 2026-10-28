package service

import (
	"context"
	"math/rand"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"
	"mygin-api/test/fakermodel"
	"time"

	"github.com/go-faker/faker/v4"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

type OrderService struct {
	appCfg *config.AppConfig
}

func NewOrderService(appCfg *config.AppConfig) *OrderService {
	return &OrderService{appCfg: appCfg}
}

func (s *OrderService) GenTestOrderData(count int) {
	orders := make([]fakermodel.Order, count)
	for i := 0; i < count; i++ {
		var order fakermodel.Order
		faker.FakeData(&order)
		s.appCfg.Logger.LogF("生成测试订单数据 | order_no=%s | user_id=%d",
			order.OrderNo,
			order.UserID,
		)

		r := rand.New(rand.NewSource(time.Now().UnixNano()))

		// 2. 生成 [0, 6) 的随机整数，即 0, 1, 2, 3, 4, 5
		// Intn(n) 返回的是 [0, n)，所以要得到 5，参数必须是 6
		// order.Status = int32(r.Intn(6))
		order.Status = 0
		order.TotalAmount = decimal.NewFromFloat(float64(r.Intn(1000))).Round(2)
		order.PayAmount = decimal.NewFromFloat(float64(r.Intn(1000))).Round(2)
		orders[i] = order
		//判断订单状态，如果是未支付，加入超时队列
		if order.Status == utils.OrderStatusCreated {
			//加入超时队列--延迟20秒超时
			timeout := float64(time.Now().Add(20 * time.Second).UnixNano())
			s.appCfg.RedisClient.ZAdd(context.Background(), utils.OrderTimeoutKey, redis.Z{
				Score:  timeout,
				Member: order.OrderNo,
			})
		}
	}
	s.appCfg.DB.CreateInBatches(&orders, count)
}
