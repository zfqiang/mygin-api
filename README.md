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

封装提供以下能力：

- `DeclareQueue(ctx, queue)`：声明持久化队列，可重复调用。
- `Publish(ctx, queue, body)`：发送持久化二进制消息；`PublishJSON(ctx, queue, value)` 会先将数据编码为 JSON。发送端等待 RabbitMQ 发布确认后才返回成功。
- `Consume(ctx, queue, prefetch, handler)`：持续消费并手动确认。回调返回 `nil` 后自动 Ack；返回错误则 Nack 且不重新入队，避免坏消息无限重试。需要保留失败消息时，请为队列配置死信交换机；取消传入的上下文可以停止消费。

业务中可通过应用配置的 `RabbitMQ` 客户端调用这些方法。例如发送消息：

```go
err := appCfg.RabbitMQ.PublishJSON(ctx, "orders", map[string]any{
    "order_id": 123,
})
```

消费示例中，回调成功返回 `nil` 后框架自动确认消息；返回错误时消息会被拒绝：

```go
type OrderEvent struct {
    OrderID int `json:"order_id"`
}

err := appCfg.RabbitMQ.Consume(ctx, "orders", 10, func(ctx context.Context, delivery amqp.Delivery) error {
    var event OrderEvent
    if err := json.Unmarshal(delivery.Body, &event); err != nil {
        return err
    }
    return orderService.Process(ctx, event)
})
```

消费代码需要导入 `context`、`encoding/json` 和 `github.com/rabbitmq/amqp091-go`，并按业务定义 `OrderEvent` 和 `orderService`。

队列由发送或消费方法自动声明，也可在业务启动时显式调用 `DeclareQueue`。RabbitMQ 日志写入 `logs/rabbitmq.log`；管理控制台默认位于 `http://localhost:15672`。

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
