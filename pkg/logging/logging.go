package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Loggers struct {
	System   *Logger
	Database *Logger
	Redis    *Logger
	writers  []io.Closer
}

type Logger struct {
	base *zap.Logger
}

func (l *Logger) LogF(format string, args ...any) {
	l.base.Info(fmt.Sprintf(format, args...))
}

func (l *Logger) WarnF(format string, args ...any) {
	l.base.Warn(fmt.Sprintf(format, args...))
}

func (l *Logger) Errorf(format string, args ...any) {
	l.base.Error(fmt.Sprintf(format, args...))
}

func (l *Logger) Fatal(args ...any) {
	l.base.Fatal(formatMessage(args))
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
		System:   wrapLogger(system),
		Database: wrapLogger(database),
		Redis:    wrapLogger(redis),
		writers:  []io.Closer{systemWriter, databaseWriter, redisWriter},
	}, nil
}

func (l *Loggers) Sync() error {
	if l == nil {
		return nil
	}
	return errors.Join(
		wrapSyncError("system", l.System.base.Sync()),
		wrapSyncError("database", l.Database.base.Sync()),
		wrapSyncError("Redis", l.Redis.base.Sync()),
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

func wrapLogger(base *zap.Logger) *Logger {
	return &Logger{
		base: base.WithOptions(zap.AddCallerSkip(1)),
	}
}

func formatMessage(args []any) string {
	if len(args) == 0 {
		return ""
	}

	var message strings.Builder
	message.WriteString(fmt.Sprint(args[0]))
	for i := 1; i < len(args); i += 2 {
		key := fmt.Sprint(args[i])
		message.WriteString(" | ")
		message.WriteString(key)
		message.WriteByte('=')
		if i+1 == len(args) {
			message.WriteString("<missing>")
			continue
		}
		message.WriteString(formatValue(args[i+1]))
	}
	return message.String()
}

func formatValue(value any) string {
	text := fmt.Sprint(value)
	if strings.ContainsAny(text, " \t\r\n|\"") {
		return strconv.Quote(text)
	}
	return text
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
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.ConsoleSeparator = " | "
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderConfig), zapcore.AddSync(writer), zapcore.DebugLevel)
	return zap.New(core, zap.AddCaller()).Named(name), writer, nil
}
