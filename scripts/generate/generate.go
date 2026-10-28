package main

import (
	"context"
	"fmt"
	"time"

	"mygin-api/pkg/config"
	"mygin-api/pkg/database"

	"gorm.io/gen"
	"gorm.io/gorm"
)

// 详细配置参考: https://gorm.io/gen/config.html
func main() {
	// 1. 连接数据库
	// 注意：parseTime=True 对于时间类型字段正确映射至关重要
	//  outPath ModelPkgPath 是相对当前项目目录的
	g := gen.NewGenerator(gen.Config{
		OutPath: "../../internal/repository/gend", // 生成代码的输出目录
		// OutFile:      "",                                            // 如果指定文件名，则所有代码生成到一个文件；否则按表名分文件
		ModelPkgPath: "../../internal/model/genm",                   // 模型结构体生成的包路径
		Mode:         gen.WithDefaultQuery | gen.WithQueryInterface, // 生成模式
	})
	// 【关键】显式声明需要导入的第三方包，否则生成的代码不会自动导入
	g.WithImportPkgPath("github.com/shopspring/decimal")

	// 加载配置文件，如果加载失败则返回错误
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Failed to load config:", err)
		return
	}

	// 创建一个10秒超时的启动上下文，用于控制初始化操作的超时时间
	startupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 初始化数据库连接，如果连接失败则返回错误
	db, err := database.Open(startupCtx, cfg.MySQL)
	if err != nil {
		fmt.Println("Failed to open database:", err)
		return
	}

	// 确保程序退出时关闭数据库连接，防止资源泄漏
	sqlDB, err := db.DB()
	if err != nil {
		panic(fmt.Errorf("cannot get underlying sql.DB: %w", err))
	}
	defer sqlDB.Close()

	// 2. 设置数据库连接
	g.UseDB(db)

	// 3. 自定义模型配置 (可选)
	// 例如：将所有 int 类型映射为 int64，或者自定义字段标签
	// g.WithJSONTagNameStrategy(func(c string) string { return c })

	g.WithDataTypeMap(map[string]func(gorm.ColumnType) (dataType string){
		"decimal": func(columnType gorm.ColumnType) (dataType string) {
			return "decimal.Decimal"
		},
		// 如果数据库中有 numeric 类型，也建议映射
		"numeric": func(columnType gorm.ColumnType) (dataType string) {
			return "decimal.Decimal"
		},
	})

	// 4. 指定要生成模型的表
	// ApplyBasic 会根据表结构生成基本的 CRUD 结构体和方法
	// 你可以传入具体的表名，或者使用 g.GenerateAllTable() 生成所有表
	g.ApplyBasic(
		// 方式一：生成指定表
		// g.GenerateModel("users"),
		// g.GenerateModel("orders"),
		// g.GenerateModel("products"),
		// g.GenerateModel("order_items"),

	// 方式二：生成所有表 (推荐用于快速上手)
	// g.GenerateAllTable()...,
	)

	// 5. 执行生成
	g.Execute()
}
