package mq

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestConsumeValidatesArguments(t *testing.T) {
	client := &RabbitMQ{}

	if err := client.Consume(context.Background(), "orders", 0, func(context.Context, amqp.Delivery) error {
		return nil
	}); err == nil {
		t.Fatal("预取数量为零时应返回错误")
	}

	if err := client.Consume(context.Background(), "orders", 1, nil); err == nil {
		t.Fatal("处理函数为空时应返回错误")
	}
}

func TestPublishJSONRejectsUnsupportedValue(t *testing.T) {
	client := &RabbitMQ{}
	if err := client.PublishJSON(context.Background(), "orders", make(chan int)); err == nil {
		t.Fatal("JSON 不支持的值应返回编码错误")
	}
}

func TestPublishHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &RabbitMQ{}
	if err := client.Publish(ctx, "orders", []byte("message")); err == nil {
		t.Fatal("已取消的上下文应返回错误")
	}
}

func TestOpenRejectsInvalidURL(t *testing.T) {
	if _, err := Open(context.Background(), "http://localhost:5672", nil); err == nil {
		t.Fatal("非 AMQP 地址应返回错误")
	}
}

func TestQueueMethodsRejectEmptyName(t *testing.T) {
	client := &RabbitMQ{}

	if err := client.DeclareQueue(context.Background(), ""); err == nil {
		t.Fatal("空队列名称应返回错误")
	}
	if err := client.Publish(context.Background(), "", nil); err == nil {
		t.Fatal("空队列名称应返回错误")
	}
}
