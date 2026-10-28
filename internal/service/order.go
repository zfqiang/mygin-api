package service

import (
	"fmt"
	"mygin-api/internal/mission"
	"mygin-api/internal/model"
	"mygin-api/internal/model/genm"
	"mygin-api/pkg/config"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-faker/faker/v4"
	"github.com/redis/go-redis/v9"
)

type OrderService struct {
	appCfg *config.AppConfig
}

func NewOrderService(appCfg *config.AppConfig) *OrderService {
	return &OrderService{appCfg: appCfg}
}

func (o *OrderService) GenTestOrderData(c *gin.Context) {
	count := 10
	orders := make([]genm.Order, count)
	for i := 0; i < count; i++ {
		var order genm.Order
		faker.FakeData(&order)
		fmt.Printf("order: %+v\n", order)
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
