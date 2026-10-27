package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mygin-api/internal/auth"
	"mygin-api/internal/cache"
	"mygin-api/internal/config"
	"mygin-api/internal/database"
	"mygin-api/internal/router"
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
	if err := auth.Migrate(db.WithContext(startupCtx)); err != nil {
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
	router := router.NewRouter()
	handler := router.New(appCfg)
	// 创建 HTTP 服务器实例，配置监听地址、处理器和读取头超时时间
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// 创建一个用于接收系统信号的上下文，当接收到中断或终止信号时会触发
	serverCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 创建一个错误通道，用于接收服务器运行时的错误
	serverErr := make(chan error, 1)
	// 启动一个 goroutine 来运行 HTTP 服务器，并将错误发送到错误通道
	go func() {
		logger.Info("server starting", "address", server.Addr, "mode", cfg.Mode)
		serverErr <- server.ListenAndServe()
	}()

	// 使用 select 语句监听服务器错误或系统信号
	select {
	case err := <-serverErr:
		// 如果服务器错误不是由于服务器正常关闭引起的，则返回错误
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-serverCtx.Done():
		// 如果接收到系统信号，则优雅地关闭服务器
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		// 执行服务器关闭操作，如果超时则返回错误
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		// 等待服务器完全关闭，如果有错误则返回
		if err := <-serverErr; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	// 如果一切正常，则返回 nil 表示成功
	return nil
}
