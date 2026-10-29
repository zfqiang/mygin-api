package mq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"mygin-api/pkg/logging"
)

type RabbitMQ struct {
	conn       *amqp.Connection
	logger     *logging.Logger
	consumerID uint64
}

// Open 建立 RabbitMQ 连接。连接失败时返回错误，避免服务在消息能力不可用时误报启动成功。
func Open(ctx context.Context, address string, logger *logging.Logger) (*RabbitMQ, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("连接 RabbitMQ: %w", err)
	}
	parsedURL, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("解析 RabbitMQ 地址: %w", err)
	}
	if (parsedURL.Scheme != "amqp" && parsedURL.Scheme != "amqps") || parsedURL.Host == "" {
		return nil, fmt.Errorf("RabbitMQ 地址必须是有效的 amqp 或 amqps URL")
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := amqp.DialConfig(address, amqp.Config{
		Dial: func(network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, address)
		},
	})
	if err != nil {
		logger.Errorf("RabbitMQ 连接失败 | address=%s | error=%v", parsedURL.Host, err)
		return nil, fmt.Errorf("连接 RabbitMQ: %w", err)
	}

	logger.LogF("RabbitMQ 连接成功 | address=%s", parsedURL.Host)
	return &RabbitMQ{conn: conn, logger: logger}, nil
}

// IsConnected 返回连接当前是否仍处于打开状态。
func (r *RabbitMQ) IsConnected() bool {
	return r != nil && r.conn != nil && !r.conn.IsClosed()
}

// Close 关闭 RabbitMQ 连接。
func (r *RabbitMQ) Close() error {
	if r == nil || r.conn == nil || r.conn.IsClosed() {
		return nil
	}
	if err := r.conn.Close(); err != nil {
		return fmt.Errorf("关闭 RabbitMQ 连接: %w", err)
	}
	r.logger.LogF("RabbitMQ 连接已关闭")
	return nil
}

// DeclareQueue 声明一个持久化队列。重复声明相同配置的队列是安全的。
func (r *RabbitMQ) DeclareQueue(ctx context.Context, queue string) error {
	if queue == "" {
		return fmt.Errorf("RabbitMQ 队列名称不能为空")
	}
	channel, err := r.openChannel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ 队列 %q: %w", queue, err)
	}
	return nil
}

// Publish 向指定队列发送持久化消息，使用默认交换机直接路由到该队列。
func (r *RabbitMQ) Publish(ctx context.Context, queue string, body []byte) error {
	return r.publish(ctx, queue, body, "application/octet-stream")
}

// PublishJSON 将任意可序列化数据编码为 JSON 后发送到指定队列。
func (r *RabbitMQ) PublishJSON(ctx context.Context, queue string, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("编码 RabbitMQ JSON 消息: %w", err)
	}
	return r.publish(ctx, queue, body, "application/json")
}

func (r *RabbitMQ) publish(ctx context.Context, queue string, body []byte, contentType string) error {
	if queue == "" {
		return fmt.Errorf("RabbitMQ 队列名称不能为空")
	}
	channel, err := r.openChannel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ 队列 %q: %w", queue, err)
	}
	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("启用 RabbitMQ 发布确认: %w", err)
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	if err := channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:  contentType,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         body,
	}); err != nil {
		r.logger.Errorf("RabbitMQ 消息发送失败 | queue=%s | error=%v", queue, err)
		return fmt.Errorf("发送 RabbitMQ 消息到队列 %q: %w", queue, err)
	}
	select {
	case confirmation, ok := <-confirms:
		if !ok {
			return fmt.Errorf("RabbitMQ 队列 %q 的发布确认通道已关闭", queue)
		}
		if !confirmation.Ack {
			return fmt.Errorf("RabbitMQ 未确认队列 %q 中的消息", queue)
		}
	case <-ctx.Done():
		return fmt.Errorf("等待 RabbitMQ 队列 %q 的发布确认: %w", queue, ctx.Err())
	}
	r.logger.LogF("RabbitMQ 消息发送成功 | queue=%s | bytes=%d", queue, len(body))
	return nil
}

// Consume 持续消费指定队列。处理成功后自动 Ack；处理失败后 Nack 且不重新入队，
// 避免错误消息无限循环，重要场景可为队列配置死信交换机。
func (r *RabbitMQ) Consume(ctx context.Context, queue string, prefetch int, handler func(context.Context, amqp.Delivery) error) error {
	if queue == "" {
		return fmt.Errorf("RabbitMQ 队列名称不能为空")
	}
	if prefetch < 1 {
		return fmt.Errorf("RabbitMQ 消费预取数量必须大于零")
	}
	if handler == nil {
		return fmt.Errorf("RabbitMQ 消息处理函数不能为空")
	}

	channel, err := r.openChannel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ 队列 %q: %w", queue, err)
	}
	if err := channel.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("设置 RabbitMQ 消费预取数量: %w", err)
	}
	consumerTag := fmt.Sprintf("mygin-api-%d", atomic.AddUint64(&r.consumerID, 1))
	deliveries, err := channel.Consume(queue, consumerTag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("开始消费 RabbitMQ 队列 %q: %w", queue, err)
	}
	r.logger.LogF("RabbitMQ 开始消费 | queue=%s | prefetch=%d", queue, prefetch)

	for {
		select {
		case <-ctx.Done():
			if err := channel.Cancel(consumerTag, false); err != nil && !errors.Is(err, amqp.ErrClosed) {
				return fmt.Errorf("停止 RabbitMQ 消费: %w", err)
			}
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				if err := ctx.Err(); err != nil {
					return nil
				}
				return fmt.Errorf("RabbitMQ 队列 %q 的消费通道已关闭", queue)
			}
			if ctx.Err() != nil {
				if err := delivery.Nack(false, true); err != nil {
					return fmt.Errorf("重新入队 RabbitMQ 队列 %q 中未处理的消息: %w", queue, err)
				}
				return nil
			}
			if err := handler(ctx, delivery); err != nil {
				r.logger.Errorf("RabbitMQ 消息处理失败 | queue=%s | error=%v", queue, err)
				if nackErr := delivery.Nack(false, false); nackErr != nil {
					return fmt.Errorf("拒绝 RabbitMQ 队列 %q 中的消息: %w", queue, nackErr)
				}
				continue
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("确认 RabbitMQ 队列 %q 中的消息: %w", queue, err)
			}
		}
	}
}

func (r *RabbitMQ) openChannel(ctx context.Context) (*amqp.Channel, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("RabbitMQ 操作已取消: %w", err)
	}
	if !r.IsConnected() {
		return nil, fmt.Errorf("RabbitMQ 连接不可用")
	}
	channel, err := r.conn.Channel()
	if err != nil {
		r.logger.Errorf("RabbitMQ 通道创建失败 | error=%v", err)
		return nil, fmt.Errorf("创建 RabbitMQ 通道: %w", err)
	}
	return channel, nil
}
