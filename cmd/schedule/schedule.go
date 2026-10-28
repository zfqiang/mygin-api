package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"mygin-api/internal/mission"
	"mygin-api/pkg/cache"
	"mygin-api/pkg/config"
	"mygin-api/pkg/database"
	_ "net/http/pprof"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// run 是应用程序的主运行函数，负责初始化和启动整个服务
// 参数 logger 用于记录应用程序的运行日志
// 返回 error 表示运行过程中可能出现的错误
/**
 * run 函数是应用程序的主入口点，负责初始化和启动整个服务
 * @param logger 日志记录器，用于记录应用程序运行时的日志信息
 * @return error 如果服务启动或运行过程中发生错误，则返回相应的错误信息
 */
func run(logger *slog.Logger) error {
	// 加载配置文件，如果加载失败则返回错误
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// 创建一个10秒超时的启动上下文，用于控制初始化操作的超时时间
	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 初始化数据库连接，如果连接失败则返回错误
	db, err := database.Open(startupCtx, cfg.MySQL)
	if err != nil {
		return err
	}
	// 获取底层的 sql.DB 实例，用于后续的关闭操作
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	// 确保在函数退出时关闭数据库连接
	defer sqlDB.Close()

	// 初始化 Redis 客户端，如果连接失败则返回错误
	redisClient, err := cache.Open(startupCtx, cfg.Redis)
	if err != nil {
		return err
	}
	// 确保在函数退出时关闭 Redis 连接
	defer redisClient.Close()

	// 创建路由处理器，传入配置、数据库连接、Redis 客户端和日志记录器
	appCfg := &config.AppConfig{
		Cfg:         &cfg,
		DB:          db,
		RedisClient: redisClient,
		Logger:      logger,
	}

	// 当收到信号时，cancel() 会被调用，所有派生的 ctx 都会收到 Done 信号
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup // 创建一个 WaitGroup，用于等待所有后台协程完成

	// 1. 创建根 Context，用于控制整个应用的生命周期
	//订单超时处理

	// 1. 启动订单超时处理协程
	orderTimeout := mission.NewOrderTimeout(appCfg)
	wg.Add(1)
	go func() {
		defer wg.Done()
		orderTimeout.Run(ctx)
	}()
	defer orderTimeout.Shutdown() // 确保在函数退出时关闭订单超时处理协程

	// 2. 监听系统信号 (SIGINT: Ctrl+C, SIGTERM: Kill/Docker Stop)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// ------------------------

	// 3. 阻塞主线程，等待信号或上下文取消
	select {
	case sig := <-sigChan:
		log.Printf("收到终止信号: %v，开始优雅退出...", sig)
		cancel() // 触发所有监听 ctx.Done() 的协程退出
	case <-ctx.Done():
		// 防止其他逻辑意外取消 context
	}

	// 4. 等待所有后台协程完成清理工作
	// 设置一个超时保护，防止某个协程卡死导致程序无法退出
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("所有后台任务已安全退出。")
	case <-time.After(10 * time.Second):
		log.Println("警告：等待超时，强制退出程序。")
	}
	return nil
}
