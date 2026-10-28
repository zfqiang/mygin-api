package database

import (
	"context"
	"fmt"
	"time"

	"mygin-api/pkg/logging"

	gormlogger "gorm.io/gorm/logger"
)

type gormLogger struct {
	logger        *logging.Logger
	level         gormlogger.LogLevel
	slowThreshold time.Duration
}

func newGormLogger(logger *logging.Logger) gormlogger.Interface {
	return &gormLogger{
		logger:        logger,
		level:         gormlogger.Warn,
		slowThreshold: 200 * time.Millisecond,
	}
}

func (l *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *gormLogger) Info(_ context.Context, message string, args ...interface{}) {
	if l.level >= gormlogger.Info {
		l.logger.LogF("数据库日志 | message=%s", fmt.Sprintf(message, args...))
	}
}

func (l *gormLogger) Warn(_ context.Context, message string, args ...interface{}) {
	if l.level >= gormlogger.Warn {
		l.logger.WarnF("数据库警告 | message=%s", fmt.Sprintf(message, args...))
	}
}

func (l *gormLogger) Error(_ context.Context, message string, args ...interface{}) {
	if l.level >= gormlogger.Error {
		l.logger.Errorf("数据库错误 | message=%s", fmt.Sprintf(message, args...))
	}
}

func (l *gormLogger) Trace(_ context.Context, begin time.Time, query func() (string, int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	duration := time.Since(begin)
	switch {
	case err != nil && l.level >= gormlogger.Error:
		sql, rows := query()
		l.logger.Errorf("SQL 执行失败 | duration=%s | rows_affected=%d | sql=%s | error=%v",
			duration, rows, sql, err)
	case l.slowThreshold > 0 && duration > l.slowThreshold && l.level >= gormlogger.Warn:
		sql, rows := query()
		l.logger.WarnF("SQL 执行缓慢 | duration=%s | slow_threshold=%s | rows_affected=%d | sql=%s",
			duration, l.slowThreshold, rows, sql)
	case l.level >= gormlogger.Info:
		sql, rows := query()
		l.logger.LogF("SQL 执行完成 | duration=%s | rows_affected=%d | sql=%s",
			duration, rows, sql)
	}
}

func (l *gormLogger) ParamsFilter(_ context.Context, sql string, _ ...interface{}) (string, []interface{}) {
	return sql, nil
}
