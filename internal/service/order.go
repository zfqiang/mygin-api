package service

import (
	"fmt"
	"math/rand"
	"mygin-api/internal/mission"
	"mygin-api/internal/model"
	"mygin-api/pkg/config"
	"mygin-api/test/fakermodel"
	"time"

	"github.com/gin-gonic/gin"
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

func (o *OrderService) GenTestOrderData(c *gin.Context) {
	count := 10
	orders := make([]fakermodel.Order, count)
	for i := 0; i < count; i++ {
		var order fakermodel.Order
		faker.FakeData(&order)
		fmt.Printf("order: %+v\n", order)

		r := rand.New(rand.NewSource(time.Now().UnixNano()))

		// 2. 生成 [0, 6) 的随机整数，即 0, 1, 2, 3, 4, 5
		// Intn(n) 返回的是 [0, n)，所以要得到 5，参数必须是 6
		// order.Status = int32(r.Intn(6))
		order.Status = 0
		order.TotalAmount = decimal.NewFromFloat(float64(r.Intn(1000))).Round(2)
		order.PayAmount = decimal.NewFromFloat(float64(r.Intn(1000))).Round(2)
		orders[i] = order
		//判断订单状态，如果是未支付，加入超时队列
		if order.Status == model.StatusCreated {
			//加入超时队列--延迟20秒超时
			timeout := float64(time.Now().Add(20 * time.Second).UnixNano())
			o.appCfg.RedisClient.ZAdd(c.Request.Context(), mission.OrderTimeoutKey, redis.Z{
				Score:  timeout,
				Member: order.OrderNo,
			})
		}
	}
	o.appCfg.DB.CreateInBatches(&orders, count)
	c.JSON(200, gin.H{"status": "success"})
}
