package controller

import (
	"mygin-api/internal/service"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"

	"github.com/gin-gonic/gin"
)

type OrderController struct {
	appCfg *config.AppConfig
}

func NewOrderController(appCfg *config.AppConfig) *OrderController {
	return &OrderController{appCfg: appCfg}
}

func (o *OrderController) GenTestOrderData(c *gin.Context) {
	count, err := utils.StrToInt(c.DefaultQuery("count", "10"))
	if err != nil {
		c.JSON(200, gin.H{"status": "error", "message": err.Error()})
		o.appCfg.Logger.WarnF("生成测试订单数据时数量参数无效 | error=%v", err)
		return
	}
	service.NewOrderService(o.appCfg).GenTestOrderData(count)
	c.JSON(200, gin.H{"status": "success"})
}
