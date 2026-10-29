# Gin + MySQL + Redis API 框架

一个可运行的 Go API 服务骨架，包含 JWT 认证、CORS、Redis 固定窗口限流、RabbitMQ 消息队列封装、结构化请求日志、panic Recovery、MySQL/Redis/RabbitMQ 连接管理和健康检查。

## 目录结构

```text

```

重新生成数据库访问代码时，在项目根目录执行 `go run ./cmd/gen`。

## 本地启动

需要 Go 1.22+、Docker Compose。

1. 复制 `.env.example` 为 `.env`，并替换 `JWT_SECRET`。`.env` 供 Docker Compose 读取；应用进程本身直接读取系统环境变量。
2. 启动依赖：

   ```powershell
   docker compose up -d
   ```

3. 在当前 PowerShell 会话设置应用环境变量并启动：

   ```powershell
   $env:APP_MODE = "debug"
   $env:APP_PORT = "8080"
   $env:JWT_SECRET = "dev-only-change-this-secret-before-use-123456789"
   $env:MYSQL_HOST = "127.0.0.1"
   $env:MYSQL_PORT = "3306"
   $env:MYSQL_USER = "root"
   $env:MYSQL_PASSWORD = "local_mysql_password"
   $env:MYSQL_DATABASE = "mygin_api"
   $env:REDIS_ADDR = "127.0.0.1:6379"
   $env:RABBITMQ_URL = "amqp://mygin:local_rabbitmq_password@127.0.0.1:5672/"
   go run ./cmd/server
   ```

配置项在进程启动时读取。`JWT_SECRET` 必填且至少 32 字节；其他配置见 `.env.example`。配置格式错误、MySQL、Redis 或 RabbitMQ 不可用时，服务会报错退出。Compose 默认创建 RabbitMQ 开发账号 `mygin`；需要自定义账号时，设置 `RABBITMQ_USER`、`RABBITMQ_PASSWORD`，并将 `RABBITMQ_URL` 配置为相同账号。示例密码仅用于本地开发，部署时必须替换。

## RabbitMQ 消息队列

RabbitMQ 客户端位于 `pkg/mq`，服务启动时建立连接并在退出时关闭；`/readyz` 会同时检查 MySQL、Redis 和 RabbitMQ。连接地址通过 `RABBITMQ_URL` 配置，支持 `amqp://` 和 `amqps://`。

框架提供常见消息模式。以下示例中的 `appCfg.RabbitMQ` 是应用中已初始化的客户端，`ctx` 应由请求或应用生命周期传入：

| 模式 | 发送 | 消费/路由说明 |
|---|---|---|
| 单发单收（Hello World） | `Publish` / `PublishJSON` | `Consume` 消费指定队列 |
| 工作队列（Work Queue） | `Publish` / `PublishJSON` | 多个消费者共同调用 `Consume` 同一队列，RabbitMQ 分发任务；可用 `prefetch` 控制未确认消息数 |
| 发布/订阅（Publish/Subscribe） | `PublishToExchange(..., mq.ExchangeFanout, ...)` | 多个消费者以空队列名调用 `ConsumeFromExchange`，每个消费者创建自己的临时队列并收到广播 |
| 路由（Routing） | `PublishToExchange(..., mq.ExchangeDirect, routingKey, ...)` | `ConsumeFromExchange` 用相同的精确 `bindingKey` 接收消息 |
| 主题（Topics） | `PublishToExchange(..., mq.ExchangeTopic, routingKey, ...)` | `bindingKey` 支持 `*`（一个词）和 `#`（零个或多个词） |
| RPC | `CallRPC` | 服务端调用 `ServeRPC`；使用 `CorrelationId` 与临时响应队列匹配请求和响应 |

#### 单发单收与工作队列

```go
err := appCfg.RabbitMQ.PublishJSON(ctx, "orders", map[string]any{
    "order_id": 123,
})
```

多实例对同一队列运行相同消费者即可形成工作队列；每个处理成功的消息由框架自动 Ack，回调返回错误则 Nack 且不重新入队：

```go
err := appCfg.RabbitMQ.Consume(ctx, "orders", 10, func(ctx context.Context, delivery amqp.Delivery) error {
    return orderService.Process(ctx, delivery.Body)
})
```

#### 发布/订阅、路由与主题

消费端声明并绑定交换机和队列，发送端将消息发布到交换机。发布/订阅用空队列名创建每个消费者独立的临时队列；路由/主题可传持久化队列名以便服务重启后继续使用。消费函数会一直运行，应在消费者 goroutine 或独立 worker 中启动；发送端代码单独调用：

```go
// 消费端：发布/订阅；每个消费者分别运行，routingKey 和 bindingKey 会被 fanout 忽略。
err := appCfg.RabbitMQ.ConsumeFromExchange(
    ctx, "events", mq.ExchangeFanout, "", "", nil, 10, handleEvent,
)
```

```go
// 发送端：发布/订阅消息广播给所有已绑定的消费者队列。
err := appCfg.RabbitMQ.PublishJSONToExchange(ctx, "events", mq.ExchangeFanout, "", event, nil)
```

```go
// 路由：只有绑定键与 routingKey 完全相同的队列会收到消息。
err := appCfg.RabbitMQ.ConsumeFromExchange(
    ctx, "logs", mq.ExchangeDirect, "error-logs", "error", nil, 10, handleLog,
)
```

```go
// 发送端：routingKey 必须与消费者绑定键完全一致。
err := appCfg.RabbitMQ.PublishToExchange(ctx, "logs", mq.ExchangeDirect, "error", []byte("failed"), nil)
```

```go
// 主题：例如绑定 "order.*" 可匹配 "order.created"，不匹配 "order.item.created"。
err := appCfg.RabbitMQ.ConsumeFromExchange(
    ctx, "domain-events", mq.ExchangeTopic, "order-events", "order.*", nil, 10, handleEvent,
)
```

```go
// 发送端：routingKey 由点分隔，"order.*" 可匹配 "order.created"。
err := appCfg.RabbitMQ.PublishToExchange(ctx, "domain-events", mq.ExchangeTopic, "order.created", body, nil)
```

常量 `mq.ExchangeDirect`、`mq.ExchangeFanout`、`mq.ExchangeTopic`、`mq.ExchangeHeaders` 分别表示 direct、fanout、topic、headers 交换机。headers 模式可用 `PublishToExchange` 传 `amqp.Table` 消息头，并用 `ConsumeFromExchange` 的 `bindingArgs`（例如 `amqp.Table{"x-match": "all", "format": "pdf"}`）配置匹配条件。更细粒度管理时可单独调用 `DeclareExchange`、`DeclareQueue` 和 `BindQueue`。

#### RPC

服务端消费 RPC 请求并返回字节响应；调用端使用带超时的 context 等待对应响应：

```go
// RPC 服务端
err := appCfg.RabbitMQ.ServeRPC(ctx, "rpc_queue", 10, func(ctx context.Context, request amqp.Delivery) ([]byte, error) {
    return processRequest(ctx, request.Body)
})
```

```go
// RPC 调用端：使用单独的 context 超时保护请求。
rpcCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
response, err := appCfg.RabbitMQ.CallRPC(rpcCtx, "rpc_queue", requestBody)
```

普通消费回调返回 `nil` 后框架自动 Ack；返回错误则 Nack 且不重新入队，避免错误消息无限循环。需要保留失败消息时，为队列配置死信交换机。RPC handler 返回错误时请求同样会被拒绝而不生成响应，调用端应使用超时 context；如需向调用端传递业务错误，可将错误编码进 handler 返回的响应 body。消费方法会持续运行，取消传入的 context 可以停止消费。发送方法等待 RabbitMQ 发布确认后才返回成功；该确认表示 broker 已接收消息，不代表消息一定匹配到队列。RabbitMQ 日志写入 `logs/rabbitmq.log`；管理控制台默认位于 `http://localhost:15672`。

以上示例使用 `mq "mygin-api/pkg/mq"`；消费回调的 `amqp.Delivery` 和 headers 参数来自 `github.com/rabbitmq/amqp091-go`。RPC 调用示例还需导入 `time`。请按业务定义 handler 和数据结构。

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 进程存活检查 |
| GET | `/readyz` | 检查 MySQL、Redis 和 RabbitMQ 可用性 |
| GET | `/api/v1/ping` | 示例公开接口 |
| POST | `/api/v1/auth/login` | 使用数据库中的用户名和密码登录，返回 JWT |
| GET | `/api/v1/me` | 示例 JWT 保护接口，要求 `Authorization: Bearer <token>` |

登录请求示例：

```json
{
  "username": "alice",
  "password": "your-password"
}
```

成功响应包含 `access_token`、`token_type`、`expires_in` 和基本用户信息；用 `Authorization: Bearer <access_token>` 访问 `/api/v1/me`。用户名查找和密码校验失败均返回相同的 `401` 响应。密码以 bcrypt 哈希保存，不会返回密码或哈希值。服务启动时会自动创建/更新 `users` 表；该接口不提供注册，新增用户时应使用 `auth.HashPassword` 生成 `PasswordHash` 后再写入数据库。

## 中间件行为

- **JWT**：仅接受 HS256；校验签名、issuer、过期时间和用户 ID。
- **CORS**：`CORS_ORIGINS` 为逗号分隔的精确来源列表；设置 `*` 可允许任意来源，但不启用 credentials。
- **限流**：Redis Lua 脚本原子执行固定窗口计数，默认每 IP 每分钟 100 次；Redis 不可用时返回 `503`，超限返回 `429` 和 `Retry-After`。
- **日志 / Recovery**：Zap 可读控制台格式日志分别写入 `logs/system.log`、`logs/database.log`、`logs/redis.log` 和 `logs/rabbitmq.log`，采用统一的时间、级别、分类、调用位置及 `消息 | 字段=值` 格式；应用日志采用清晰的中文事件描述。统一日志器提供格式化方法 `LogF`、`WarnF`、`Errorf`、`Fatal`。日志按 100 MB 切分，保留最多 7 个备份和 30 天。数据库日志清晰记录慢查询及错误，SQL 参数默认脱敏；Redis 日志记录命令名、耗时和错误，不记录命令参数。RabbitMQ 日志记录连接、消息发送和消费处理失败信息。panic 记录堆栈并返回通用 `500`，不会向客户端泄漏异常细节。
- **代理 IP**：默认不信任反向代理头，避免伪造客户端 IP。部署在代理后时，应在 `internal/router/router.go` 中按部署网络配置可信代理。

## 测试与构建

```powershell
go test ./...
go build ./cmd/server
```
