# Gin + MySQL + Redis API 框架

基于 Go 和 Gin 的 API 服务示例，集成 MySQL、Redis、RabbitMQ、JWT 认证、CORS、请求限流、结构化日志、健康检查及订单超时后台任务。

## 项目结构

```text
cmd/                    # 可执行程序入口
  server/               # HTTP API 服务
  schedule/             # 后台定时任务
internal/               # 控制器、中间件、路由、模型、数据访问及业务逻辑
pkg/                    # 配置、数据库、缓存、消息队列、日志和工具组件
scripts/sql/            # 数据库 SQL 脚本
scripts/generate/       # GORM Gen 代码生成入口
test/                   # 并发测试和测试模型
logs/                   # 运行日志
```

## 环境要求

- Go 版本需符合 `go.mod` 中的声明（当前为 Go 1.26.0）。
- Docker Compose
- MySQL 8、Redis 7、RabbitMQ 3（可通过项目的 Compose 配置启动）

## 本地启动

在项目根目录执行以下 PowerShell 命令：

1. 创建本地配置文件并编辑配置：

   ```powershell
   Copy-Item .env.example .env
   ```

   至少替换 `JWT_SECRET` 为不少于 32 字节的随机值，并确认 `REDIS_ADDR` 使用 `主机:端口` 格式（例如 `127.0.0.1:6379`）。如需修改 RabbitMQ 账号，也要同步更新 `RABBITMQ_URL`。

2. 启动 MySQL、Redis 和 RabbitMQ：

   ```powershell
   docker compose up -d
   ```

   默认端口为 MySQL `3306`、Redis `6379`、RabbitMQ `5672`；RabbitMQ 管理页面端口为 `15672`。Compose 默认 RabbitMQ 用户为 `mygin`，默认开发密码为 `local_rabbitmq_password`。

3. 启动 HTTP 服务：

   ```powershell
   go run ./cmd/server
   ```

   服务默认监听 `:8080`。应用从项目根目录的 `.env` 加载配置；系统环境变量优先于 `.env` 中的值。

4. （可选）单独启动订单超时后台任务：

   ```powershell
   go run ./cmd/schedule
   ```

   此进程也需要可用的 MySQL、Redis 和 `.env` 配置。

首次启动会检查各依赖的连接，并自动迁移认证用的 `users` 表及积分账户 `user_points` 表。`scripts/sql/mygin.sql` 包含商品、订单等业务表结构；如需使用相关业务功能，需自行导入数据库。该 SQL 脚本包含 `DROP TABLE IF EXISTS`，执行前请确认目标数据库并备份已有数据。

## 配置说明

配置示例见 `.env.example`。主要配置如下：

| 配置项 | 说明 |
|---|---|
| `APP_MODE`、`APP_PORT` | Gin 运行模式（`debug`、`release` 或 `test`）及 HTTP 端口 |
| `JWT_SECRET`、`JWT_ISSUER`、`JWT_TTL` | JWT 密钥、签发者和有效期；密钥至少 32 字节 |
| `CORS_ORIGINS` | 逗号分隔的允许来源 |
| `RATE_LIMIT`、`RATE_BURST` | HTTP 服务全局令牌桶限流速率（请求/秒）及突发容量 |
| `RATE_LIMIT_IP`、`RATE_BURST_IP` | 健康检查路由的单 IP 限流速率及突发容量 |
| `MYSQL_HOST`、`MYSQL_PORT`、`MYSQL_USER`、`MYSQL_PASSWORD`、`MYSQL_DATABASE` | MySQL 连接配置 |
| `REDIS_ADDR`、`REDIS_PASSWORD`、`REDIS_DB` | Redis 连接配置；地址格式为 `主机:端口` |
| `RABBITMQ_URL` | RabbitMQ AMQP/AMQPS 连接地址 |
| `EMAIL_HOST`、`EMAIL_PORT`、`EMAIL_USER`、`EMAIL_PASSWORD` | 欢迎邮件 SMTP 配置；全部填写后才会发送邮件 |
| `WORK_POOL_SIZE`、`WORK_POOL_IDLE_TIMEOUT` | 工作池配置 |

`.env.example` 中的密码仅用于本地开发，部署前必须替换。配置错误、密钥不符合要求或依赖服务无法连接时，服务会启动失败并输出错误。

## HTTP 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz/` | 存活检查；仅允许本机访问 |
| GET | `/healthz/readyz` | 就绪检查 MySQL、Redis 和 RabbitMQ；仅允许本机访问 |
| GET | `/api/ping` | 示例公开接口 |
| POST | `/api/auth/register` | 注册用户并投递异步欢迎任务 |
| POST | `/api/auth/login` | 用户名和密码校验成功后签发 JWT |
| GET | `/api/me` | JWT 保护的示例接口 |
| GET | `/gen/orders` | 生成测试订单数据；仅允许本机访问 |
| GET | `/gen/products` | 生成测试商品数据；仅允许本机访问 |
| GET | `/activity/secKill/init` | 初始化秒杀商品库存到 Redis |
| POST | `/activity/secKill/start` | 秒杀示例接口 |

## 统一响应格式

业务接口和中间件统一返回以下 JSON 结构，`code` 与 HTTP 状态码一致；成功结果放在 `data` 中，失败时 `data` 为 `null`：

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "message": "pong"
  }
}
```

使用 `pkg/utils` 中的 `Success` 和 `Failure` 快速返回成功或错误响应；需要自定义业务码时可使用 `Respond`。如需中止 Gin 请求链，可使用 `AbortFailure`。

登录请求示例：

```json
{
  "username": "alice",
  "password": "your-password"
}
```

注册请求需要用户名、邮箱、手机号和至少 8 位密码，邮箱和手机号必须唯一：

```json
{
  "username": "alice",
  "email": "alice@example.com",
  "phone": "+8613800000000",
  "password": "your-password"
}
```

注册成功后，API 将用户信息保存到数据库并立即返回“注册成功”，随后由独立 goroutine 异步发布注册事件到 RabbitMQ 的 `user.registered` 持久化队列；HTTP 服务中的后台消费者异步发送欢迎邮件、初始化 MySQL 用户积分账户（初始积分为 0）并处理短信通知。SMTP 配置未填写时会记录日志并跳过邮件；当前未接入短信服务商，短信任务会记录为跳过。RabbitMQ 发布失败会记录错误日志，但不会影响已成功的注册响应；当前发布操作没有持久化 outbox 或自动重试保障。用户注册信息和密码哈希保存在数据库中，密码使用 bcrypt 加密。

登录成功响应包含 `access_token`、`token_type`、`expires_in` 和基本用户信息。访问 `/api/me` 时，在请求头中传入 `Authorization: Bearer <access_token>`。用户名不存在或密码错误时均返回相同的 `401` 响应；密码使用 bcrypt 校验。

`/gen/*` 是本机限定的测试数据接口；秒杀相关接口为示例业务逻辑，不应未经评估直接用于生产环境。

## 中间件与运行行为

- **JWT**：仅接受 HS256，并校验签名、issuer、过期时间和用户 ID。
- **CORS**：`CORS_ORIGINS` 使用逗号分隔的精确来源列表；允许凭据时不要将来源设为 `*`。
- **限流**：全局限流使用进程内令牌桶；健康检查路由额外按客户端 IP 限流。超限返回 `429`。限流状态不会在多个服务实例间共享。
- **代理 IP**：Gin 默认不信任代理头，客户端 IP 来源于直接连接。部署在反向代理后时，需根据部署网络审慎配置可信代理。
- **健康检查访问**：健康检查路由仅允许回环地址访问；容器或外部负载均衡器探测时需确认网络访问方式。
- **日志**：系统、数据库、Redis 和 RabbitMQ 日志分别写入 `logs/system.log`、`logs/database.log`、`logs/redis.log` 和 `logs/rabbitmq.log`，日志文件按大小轮转。
- **优雅关闭**：HTTP 服务和后台任务接收终止信号后停止；HTTP 服务会等待在途请求完成。
- **RabbitMQ**：客户端在 HTTP 服务启动时连接，在进程退出时关闭。就绪检查会验证 RabbitMQ 连接状态。

RabbitMQ 客户端封装位于 `pkg/mq`，支持队列发布/消费、JSON 消息、Exchange（direct、fanout、topic、headers）以及 RPC。所有消费端均关闭 auto-ack：处理成功后显式 Ack，处理失败后 Nack 且不重新入队；服务取消时尚未处理的消息会重新入队。需要保留失败消息时，可为队列配置死信交换机。AMQP 管理控制台默认地址为 `http://localhost:15672`。

## 数据库代码生成

GORM Gen 的生成入口位于 `scripts/generate/generate.go`，模型和查询代码分别位于 `internal/model/genm` 与 `internal/repository/gend`。生成前请检查该脚本中的目标表配置、数据库连接和输出路径，再从项目根目录运行：

```powershell
go run ./scripts/generate
```

## 测试与构建

```powershell
go test ./...
go build ./cmd/server
go build ./cmd/schedule
```
