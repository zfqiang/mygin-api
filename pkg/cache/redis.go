package cache

import (
	"context"
	"fmt"
	"net"
	"time"

	"mygin-api/pkg/config"
	"mygin-api/pkg/logging"

	"github.com/redis/go-redis/v9"
)

func Open(ctx context.Context, cfg config.RedisConfig, redisLogger *logging.Logger) (*redis.Client, error) {
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
	logger *logging.Logger
}

func (h redisLoggingHook) DialHook(next redis.DialHook) redis.DialHook {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		start := time.Now()
		conn, err := next(ctx, network, addr)
		if err != nil {
			h.logger.Errorf("Redis 连接失败 | network=%s | address=%s | duration=%s | error=%v",
				network, addr, time.Since(start), err)
			return nil, err
		}
		h.logger.LogF("Redis 连接成功 | network=%s | address=%s | duration=%s",
			network,
			addr,
			time.Since(start),
		)
		return conn, nil
	}
}

func (h redisLoggingHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		if err != nil && err != redis.Nil {
			h.logger.Errorf("Redis 命令执行失败 | command=%s | duration=%s | error=%v",
				cmd.Name(), time.Since(start), err)
		} else {
			h.logger.LogF("Redis 命令执行完成 | command=%s | duration=%s",
				cmd.Name(),
				time.Since(start),
			)
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
		if err != nil && err != redis.Nil {
			h.logger.Errorf("Redis Pipeline 执行失败 | commands=%v | command_count=%d | duration=%s | error=%v",
				commands, len(cmds), time.Since(start), err)
		} else {
			h.logger.LogF("Redis Pipeline 执行完成 | commands=%v | command_count=%d | duration=%s",
				commands,
				len(cmds),
				time.Since(start),
			)
		}
		return err
	}
}
