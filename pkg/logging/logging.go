package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Loggers struct {
	System   *zap.SugaredLogger
	Database *zap.Logger
	Redis    *zap.Logger
	writers  []io.Closer
}

func New(directory string) (*Loggers, error) {
	if directory == "" {
		directory = "logs"
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	system, systemWriter, err := newLogger(filepath.Join(directory, "system.log"), "system")
	if err != nil {
		return nil, err
	}
	database, databaseWriter, err := newLogger(filepath.Join(directory, "database.log"), "database")
	if err != nil {
		_ = systemWriter.Close()
		_ = system.Sync()
		return nil, err
	}
	redis, redisWriter, err := newLogger(filepath.Join(directory, "redis.log"), "redis")
	if err != nil {
		_ = systemWriter.Close()
		_ = databaseWriter.Close()
		_ = system.Sync()
		_ = database.Sync()
		return nil, err
	}

	return &Loggers{
		System:   system.Sugar(),
		Database: database,
		Redis:    redis,
		writers:  []io.Closer{systemWriter, databaseWriter, redisWriter},
	}, nil
}

func (l *Loggers) Sync() error {
	if l == nil {
		return nil
	}
	return errors.Join(
		wrapSyncError("system", l.System.Sync()),
		wrapSyncError("database", l.Database.Sync()),
		wrapSyncError("Redis", l.Redis.Sync()),
	)
}

func (l *Loggers) Close() error {
	if l == nil {
		return nil
	}
	errs := []error{l.Sync()}
	for _, writer := range l.writers {
		errs = append(errs, writer.Close())
	}
	return errors.Join(errs...)
}

func wrapSyncError(name string, err error) error {
	if err != nil {
		return fmt.Errorf("sync %s log: %w", name, err)
	}
	return nil
}

func newLogger(path, name string) (*zap.Logger, *lumberjack.Logger, error) {
	writer := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     30,
		Compress:   true,
	}
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.LevelKey = "level"
	encoderConfig.NameKey = "logger"
	encoderConfig.CallerKey = "caller"
	encoderConfig.MessageKey = "message"
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	core := zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.AddSync(writer), zapcore.DebugLevel)
	return zap.New(core, zap.AddCaller()).Named(name), writer, nil
}
