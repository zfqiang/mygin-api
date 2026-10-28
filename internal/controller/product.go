package controller

import (
	"context"
	"fmt"
	"math/rand"
	"mygin-api/internal/model"
	"mygin-api/internal/service"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"
	"mygin-api/test/fakermodel"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-faker/faker/v4"
	"github.com/shopspring/decimal"
)

type ProductParam struct {
	ProductID int64 `json:"product_id"`
	UserID    int64 `json:"user_id"`
	Count     int32 `json:"count"`
}

type ProductController struct {
	appCfg *config.AppConfig
}

func NewProductController(appCfg *config.AppConfig) *ProductController {
	return &ProductController{appCfg: appCfg}
}

func (p *ProductController) HandleSecKill(c *gin.Context) {
	var param ProductParam
	if err := c.ShouldBindJSON(&param); err != nil {
		p.appCfg.Logger.Error("Invalid parameters", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	productID := param.ProductID
	userID := param.UserID
	count := param.Count

	// 校验参数
	if productID <= 0 || userID <= 0 || count <= 0 {
		p.appCfg.Logger.Error("Invalid parameters")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid parameters"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 模拟快速校验（如用户是否已购买过，可在 Redis 中检查）
	uscKey := utils.GetUserSeckillKey(productID)
	result, err := p.appCfg.RedisClient.SIsMember(ctx, uscKey, userID).Result()
	if err != nil {
		p.appCfg.Logger.Error("Redis error: SIsMember", "productID", productID, "userID", userID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error: SIsMember"})
		return
	}

	if result {
		p.appCfg.Logger.Warn("User has already bought the product", "productID", productID, "userID", userID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户已购抢购过该商品"})
		return
	}

	// 方案 A：同步执行（推荐用于需要立即知道结果的抢购，防止用户重复点击）
	resultChan := make(chan int, 1)

	go func() {
		resp, err := service.NewProductService(p.appCfg).DoSecKill(ctx, productID, userID, count)
		if err != nil {
			p.appCfg.Logger.Error("DoSecKill error", "productID", productID, "userID", userID, "count", count, "error", err)

			//抢购失败，从redis中删除记录
			p.appCfg.RedisClient.SRem(ctx, uscKey, userID)
		}
		resultChan <- resp
	}()

	// 等待协程结果，或者设置一个较短的超时让前端轮询/等待
	select {
	case <-ctx.Done():
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		return
	case <-time.After(5 * time.Second):
		c.JSON(http.StatusInternalServerError, gin.H{"error": "处理超时，请稍后查询订单状态"})
		return
	case resp := <-resultChan:
		c.JSON(http.StatusOK, resp)
	}

}

func (o *ProductController) GenTestProductData(c *gin.Context) {
	count, err := utils.StrToInt(c.DefaultQuery("count", "10"))
	if err != nil {
		c.JSON(200, gin.H{"status": "error", "message": err.Error()})
		o.appCfg.Logger.Error("strconv.Atoi failed:", "error", err)
		return
	}
	Products := make([]fakermodel.Product, count)
	for i := 0; i < count; i++ {
		var Product fakermodel.Product
		faker.FakeData(&Product)
		fmt.Printf("Product: %+v\n", Product)

		r := rand.New(rand.NewSource(time.Now().UnixNano()))

		Product.Status = 1
		Product.Price = decimal.NewFromFloat(float64(r.Intn(1000))).Round(2)
		Products[i] = Product
	}
	o.appCfg.DB.CreateInBatches(&Products, count)
	c.JSON(200, gin.H{"status": "success"})
}

func (o *ProductController) SecKillInit(c *gin.Context) {
	productID, _ := utils.StrToInt64(c.DefaultQuery("product_id", "1"))

	if productID <= 0 {
		c.JSON(200, gin.H{"status": "error", "message": "Invalid product_id"})
		return
	}
	// 查询商品信息
	var product model.Product
	if err := o.appCfg.DB.Select("id, stock").Where("id = ?", productID).Where("stock > 0").First(&product).Error; err != nil {
		c.JSON(200, gin.H{"status": "error", "message": err.Error()})
		return
	}

	//初始化到抢购的redis里面
	pscKey := utils.GetProductSeckillKey(productID)
	o.appCfg.RedisClient.Set(context.Background(), pscKey, product.Stock, 0)
	c.JSON(200, gin.H{"status": "success"})
}
