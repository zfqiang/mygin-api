package controller

import (
	"mygin-api/internal/service"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"

	"github.com/gin-gonic/gin"
	"net/http"
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
		o.appCfg.Logger.WarnF("生成测试订单数据时数量参数无效 | error=%v", err)
		utils.Failure(c, http.StatusBadRequest, "invalid count")
		return
	}
	service.NewOrderService(o.appCfg).GenTestOrderData(count)
	utils.Success(c, nil)
}
