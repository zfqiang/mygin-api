package database

import (
	"context"
	"fmt"
	"net"
	"time"

	"mygin-api/pkg/config"

	mysqlDriverConfig "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Open(ctx context.Context, cfg config.MySQLConfig, dbLogger *zap.Logger) (*gorm.DB, error) {
	dsnConfig := mysqlDriverConfig.Config{
		User:                 cfg.User,
		Passwd:               cfg.Password,
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(cfg.Host, cfg.Port),
		DBName:               cfg.Database,
		Params:               map[string]string{"charset": "utf8mb4"},
		ParseTime:            true,
		Loc:                  time.Local,
		AllowNativePasswords: true,
	}
	dsn := dsnConfig.FormatDSN()
	db, err := gorm.Open(mysqlDriver.Open(dsn), &gorm.Config{
		Logger: logger.New(zap.NewStdLog(dbLogger), logger.Config{
			SlowThreshold:        200 * time.Millisecond,
			LogLevel:             logger.Warn,
			ParameterizedQueries: true,
			Colorful:             false,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("open MySQL connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get MySQL connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping MySQL: %w", err)
	}
	return db, nil
}
