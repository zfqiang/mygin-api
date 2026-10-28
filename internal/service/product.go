package service

import (
	"fmt"
	"math/rand"
	"mygin-api/pkg/config"
	"mygin-api/pkg/utils"
	"mygin-api/test/fakermodel"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-faker/faker/v4"
	"github.com/shopspring/decimal"
)

type ProductService struct {
	appCfg *config.AppConfig
}

func NewProductService(appCfg *config.AppConfig) *ProductService {
	return &ProductService{appCfg: appCfg}
}

func (o *ProductService) GenTestProductData(c *gin.Context) {
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
