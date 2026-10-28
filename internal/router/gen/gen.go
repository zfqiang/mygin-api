package gen

import (
	"mygin-api/internal/controller"
	"mygin-api/internal/middleware"
	"mygin-api/internal/service"
	"mygin-api/pkg/config"
)

type GenRouter struct {
	appCfg *config.AppConfig
}

func NewGenRouter(appCfg *config.AppConfig) *GenRouter {
	return &GenRouter{
		appCfg: appCfg,
	}
}

// RegisterRoutes 注册路由方法
// 该方法用于调用具体的路由注册函数
func (r *GenRouter) RegisterRoutes() {
	// 调用注册通用路由的方法
	r.RegisterGenRoutes()
}

// RegisterGenRoutes 注册生成测试数据的路由
// 该方法为ApiRouter结构体的方法，用于注册与生成测试数据相关的API路由
func (r *GenRouter) RegisterGenRoutes() {
	// 创建一个路由组，路径前缀为"/gen"，并使用LocalhostOnly中间件限制只有本地可以访问
	gen := r.appCfg.Engine.Group("/gen").Use(middleware.LocalhostOnlyMiddleware())
	// 注册生成测试订单数据的路由，GET请求"/gen/orders"将调用OrderService的GenTestOrderData方法
	gen.GET("/orders", service.NewOrderService(r.appCfg).GenTestOrderData)
	// 注册生成测试产品数据的路由，GET请求"/gen/products"将调用ProductService的GenTestProductData方法
	gen.GET("/products", controller.NewProductController(r.appCfg).GenTestProductData)
}
