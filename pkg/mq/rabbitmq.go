package mq

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

const (
	// ExchangeDirect 按 routing key 精确匹配，适合路由模式。
	ExchangeDirect = "direct"
	// ExchangeFanout 将消息广播给所有绑定队列，适合发布/订阅模式。
	ExchangeFanout = "fanout"
	// ExchangeTopic 按 * 和 # 通配 routing key，适合主题模式。
	ExchangeTopic = "topic"
	// ExchangeHeaders 按消息 headers 匹配，适合不便使用 routing key 的场景。
	ExchangeHeaders = "headers"
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

// DeclareExchange 声明持久化交换机。exchangeType 可使用 ExchangeDirect、
// ExchangeFanout、ExchangeTopic 或 ExchangeHeaders。
func (r *RabbitMQ) DeclareExchange(ctx context.Context, exchange, exchangeType string) error {
	if err := validateExchange(exchange, exchangeType); err != nil {
		return err
	}
	channel, err := r.openChannel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	if err := channel.ExchangeDeclare(exchange, exchangeType, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ 交换机 %q: %w", exchange, err)
	}
	return nil
}

// BindQueue 将队列绑定到交换机。routingKey 对 direct/topic 有意义；
// headers 交换机使用 arguments 进行匹配（可设置 x-match=all 或 x-match=any）。
func (r *RabbitMQ) BindQueue(ctx context.Context, queue, exchange, routingKey string, arguments amqp.Table) error {
	if queue == "" {
		return fmt.Errorf("RabbitMQ 队列名称不能为空")
	}
	if exchange == "" {
		return fmt.Errorf("RabbitMQ 交换机名称不能为空")
	}
	channel, err := r.openChannel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	if err := channel.QueueBind(queue, routingKey, exchange, false, arguments); err != nil {
		return fmt.Errorf("绑定 RabbitMQ 队列 %q 到交换机 %q: %w", queue, exchange, err)
	}
	return nil
}

// PublishToExchange 将持久化消息发布到指定交换机，发送端等待 broker 发布确认。
// direct/topic 用 routingKey 路由；fanout 忽略 routingKey；headers 的匹配值放在 headers 中。
func (r *RabbitMQ) PublishToExchange(ctx context.Context, exchange, exchangeType, routingKey string, body []byte, headers amqp.Table) error {
	if err := validateExchange(exchange, exchangeType); err != nil {
		return err
	}
	channel, err := r.openChannel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	if err := channel.ExchangeDeclare(exchange, exchangeType, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ 交换机 %q: %w", exchange, err)
	}
	return r.publishOnChannel(ctx, channel, exchange, routingKey, body, "application/octet-stream", headers)
}

// PublishJSONToExchange 将 JSON 消息发布到指定交换机。
func (r *RabbitMQ) PublishJSONToExchange(ctx context.Context, exchange, exchangeType, routingKey string, value any, headers amqp.Table) error {
	body, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("编码 RabbitMQ JSON 消息: %w", err)
	}
	return r.PublishToExchange(ctx, exchange, exchangeType, routingKey, body, headers)
}

// ConsumeFromExchange 声明并绑定队列后持续消费交换机消息。
// queue 传空字符串时创建仅当前消费者使用的临时队列；headers 交换机通过 bindingArgs 匹配。
func (r *RabbitMQ) ConsumeFromExchange(ctx context.Context, exchange, exchangeType, queue, bindingKey string, bindingArgs amqp.Table, prefetch int, handler func(context.Context, amqp.Delivery) error) error {
	if err := validateExchange(exchange, exchangeType); err != nil {
		return err
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

	if err := channel.ExchangeDeclare(exchange, exchangeType, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ 交换机 %q: %w", exchange, err)
	}
	queueName := queue
	if queueName == "" {
		declared, err := channel.QueueDeclare("", false, true, true, false, nil)
		if err != nil {
			return fmt.Errorf("声明 RabbitMQ 临时队列: %w", err)
		}
		queueName = declared.Name
	} else if _, err := channel.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ 队列 %q: %w", queueName, err)
	}
	if err := channel.QueueBind(queueName, bindingKey, exchange, false, bindingArgs); err != nil {
		return fmt.Errorf("绑定 RabbitMQ 队列 %q 到交换机 %q: %w", queueName, exchange, err)
	}
	return r.consumeOnChannel(ctx, channel, queueName, prefetch, handler)
}

// CallRPC 向 RPC 请求队列发送请求，并等待匹配 CorrelationId 的响应。
// 调用方应传入带超时的 context，避免服务端无响应时无限等待。
func (r *RabbitMQ) CallRPC(ctx context.Context, queue string, request []byte) ([]byte, error) {
	if queue == "" {
		return nil, fmt.Errorf("RabbitMQ RPC 队列名称不能为空")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("RabbitMQ RPC 请求已取消: %w", err)
	}
	correlationID, err := newCorrelationID()
	if err != nil {
		return nil, fmt.Errorf("生成 RabbitMQ RPC 请求标识: %w", err)
	}
	channel, err := r.openChannel(ctx)
	if err != nil {
		return nil, err
	}
	defer channel.Close()

	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return nil, fmt.Errorf("声明 RabbitMQ RPC 队列 %q: %w", queue, err)
	}
	replyQueue, err := channel.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		return nil, fmt.Errorf("声明 RabbitMQ RPC 响应队列: %w", err)
	}
	replies, err := channel.Consume(replyQueue.Name, "", true, true, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("监听 RabbitMQ RPC 响应: %w", err)
	}
	response := make(chan []byte, 1)
	replyErr := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-replies:
				if !ok {
					replyErr <- fmt.Errorf("RabbitMQ RPC 响应通道已关闭")
					return
				}
				if delivery.CorrelationId == correlationID {
					response <- delivery.Body
					return
				}
			}
		}
	}()

	if err := channel.Confirm(false); err != nil {
		return nil, fmt.Errorf("启用 RabbitMQ RPC 发布确认: %w", err)
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	if err := channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:   "application/octet-stream",
		DeliveryMode:  amqp.Persistent,
		Timestamp:     time.Now(),
		CorrelationId: correlationID,
		ReplyTo:       replyQueue.Name,
		Body:          request,
	}); err != nil {
		return nil, fmt.Errorf("发送 RabbitMQ RPC 请求到队列 %q: %w", queue, err)
	}
	select {
	case confirmation, ok := <-confirms:
		if !ok {
			return nil, fmt.Errorf("RabbitMQ RPC 发布确认通道已关闭")
		}
		if !confirmation.Ack {
			return nil, fmt.Errorf("RabbitMQ 未确认 RPC 请求")
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("等待 RabbitMQ RPC 请求确认: %w", ctx.Err())
	}
	select {
	case body := <-response:
		return body, nil
	case err := <-replyErr:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("等待 RabbitMQ RPC 响应: %w", ctx.Err())
	}
}

// ServeRPC 消费 RPC 请求，并将 handler 返回的数据发送到请求的 ReplyTo 队列。
// handler 返回错误时请求会被拒绝且不重新入队；可为请求队列配置死信交换机。
func (r *RabbitMQ) ServeRPC(ctx context.Context, queue string, prefetch int, handler func(context.Context, amqp.Delivery) ([]byte, error)) error {
	if queue == "" {
		return fmt.Errorf("RabbitMQ RPC 队列名称不能为空")
	}
	if prefetch < 1 {
		return fmt.Errorf("RabbitMQ 消费预取数量必须大于零")
	}
	if handler == nil {
		return fmt.Errorf("RabbitMQ RPC 处理函数不能为空")
	}
	channel, err := r.openChannel(ctx)
	if err != nil {
		return err
	}
	defer channel.Close()

	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("声明 RabbitMQ RPC 队列 %q: %w", queue, err)
	}
	if err := channel.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("设置 RabbitMQ RPC 消费预取数量: %w", err)
	}
	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("启用 RabbitMQ RPC 响应发布确认: %w", err)
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, prefetch))
	consumerTag := fmt.Sprintf("mygin-api-rpc-%d", atomic.AddUint64(&r.consumerID, 1))
	deliveries, err := channel.Consume(queue, consumerTag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("开始消费 RabbitMQ RPC 队列 %q: %w", queue, err)
	}
	r.logger.LogF("RabbitMQ RPC 开始消费 | queue=%s | prefetch=%d", queue, prefetch)

	for {
		select {
		case <-ctx.Done():
			if err := channel.Cancel(consumerTag, false); err != nil && !errors.Is(err, amqp.ErrClosed) {
				return fmt.Errorf("停止 RabbitMQ RPC 消费: %w", err)
			}
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("RabbitMQ RPC 队列 %q 的消费通道已关闭", queue)
			}
			if delivery.ReplyTo == "" || delivery.CorrelationId == "" {
				if err := delivery.Nack(false, false); err != nil {
					return fmt.Errorf("拒绝无效的 RabbitMQ RPC 请求: %w", err)
				}
				continue
			}
			body, handlerErr := handler(ctx, delivery)
			if handlerErr != nil {
				r.logger.Errorf("RabbitMQ RPC 请求处理失败 | queue=%s | error=%v", queue, handlerErr)
				if err := delivery.Nack(false, false); err != nil {
					return fmt.Errorf("拒绝 RabbitMQ RPC 请求: %w", err)
				}
				continue
			}
			if err := channel.PublishWithContext(ctx, "", delivery.ReplyTo, false, false, amqp.Publishing{
				ContentType:   "application/octet-stream",
				CorrelationId: delivery.CorrelationId,
				Body:          body,
			}); err != nil {
				return fmt.Errorf("发送 RabbitMQ RPC 响应: %w", err)
			}
			select {
			case confirmation, ok := <-confirms:
				if !ok {
					return fmt.Errorf("RabbitMQ RPC 响应确认通道已关闭")
				}
				if !confirmation.Ack {
					return fmt.Errorf("RabbitMQ 未确认 RPC 响应")
				}
			case <-ctx.Done():
				return fmt.Errorf("等待 RabbitMQ RPC 响应确认: %w", ctx.Err())
			}
			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("确认 RabbitMQ RPC 请求: %w", err)
			}
		}
	}
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
	return r.publishOnChannel(ctx, channel, "", queue, body, contentType, nil)
}

func (r *RabbitMQ) publishOnChannel(ctx context.Context, channel *amqp.Channel, exchange, routingKey string, body []byte, contentType string, headers amqp.Table) error {
	destinationType := "交换机"
	destination := exchange
	if exchange == "" {
		destinationType = "队列"
		destination = routingKey
	}
	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("启用 RabbitMQ 发布确认: %w", err)
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, 1))
	if err := channel.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  contentType,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Headers:      headers,
		Body:         body,
	}); err != nil {
		r.logger.Errorf("RabbitMQ 消息发送失败 | destination=%s | routing_key=%s | error=%v", destination, routingKey, err)
		return fmt.Errorf("发送 RabbitMQ 消息到%s %q: %w", destinationType, destination, err)
	}
	select {
	case confirmation, ok := <-confirms:
		if !ok {
			return fmt.Errorf("RabbitMQ %s %q 的发布确认通道已关闭", destinationType, destination)
		}
		if !confirmation.Ack {
			return fmt.Errorf("RabbitMQ 未确认%s %q 中的消息", destinationType, destination)
		}
	case <-ctx.Done():
		return fmt.Errorf("等待 RabbitMQ %s %q 的发布确认: %w", destinationType, destination, ctx.Err())
	}
	r.logger.LogF("RabbitMQ 消息发送成功 | destination=%s | routing_key=%s | bytes=%d", destination, routingKey, len(body))
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
	return r.consumeOnChannel(ctx, channel, queue, prefetch, handler)
}

func (r *RabbitMQ) consumeOnChannel(ctx context.Context, channel *amqp.Channel, queue string, prefetch int, handler func(context.Context, amqp.Delivery) error) error {
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

func validateExchange(exchange, exchangeType string) error {
	if exchange == "" {
		return fmt.Errorf("RabbitMQ 交换机名称不能为空")
	}
	switch exchangeType {
	case ExchangeDirect, ExchangeFanout, ExchangeTopic, ExchangeHeaders:
		return nil
	default:
		return fmt.Errorf("不支持的 RabbitMQ 交换机类型 %q", exchangeType)
	}
}

func newCorrelationID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
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
