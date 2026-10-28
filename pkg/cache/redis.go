package cache

import (
	"context"
	"fmt"
	"net"
	"time"

	"mygin-api/pkg/config"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func Open(ctx context.Context, cfg config.RedisConfig, redisLogger *zap.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	client.AddHook(redisLoggingHook{logger: redisLogger})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return client, nil
}

type redisLoggingHook struct {
	logger *zap.Logger
}

func (h redisLoggingHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		start := time.Now()
		conn, err := next(ctx, network, addr)
		if err != nil {
			h.logger.Error("Redis connection failed",
				zap.String("network", network),
				zap.String("address", addr),
				zap.Duration("duration", time.Since(start)),
				zap.Error(err),
			)
			return nil, err
		}
		h.logger.Info("Redis connected",
			zap.String("network", network),
			zap.String("address", addr),
			zap.Duration("duration", time.Since(start)),
		)
		return conn, nil
	}
}

func (h redisLoggingHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		fields := []zap.Field{
			zap.String("command", cmd.Name()),
			zap.Duration("duration", time.Since(start)),
		}
		if err != nil && err != redis.Nil {
			fields = append(fields, zap.Error(err))
			h.logger.Error("Redis command failed", fields...)
		} else {
			h.logger.Info("Redis command", fields...)
		}
		return err
	}
}

func (h redisLoggingHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		commands := make([]string, len(cmds))
		for i, cmd := range cmds {
			commands[i] = cmd.Name()
		}
		fields := []zap.Field{
			zap.Strings("commands", commands),
			zap.Int("command_count", len(cmds)),
			zap.Duration("duration", time.Since(start)),
		}
		if err != nil && err != redis.Nil {
			fields = append(fields, zap.Error(err))
			h.logger.Error("Redis pipeline failed", fields...)
		} else {
			h.logger.Info("Redis pipeline", fields...)
		}
		return err
	}
}
