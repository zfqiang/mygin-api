# Gin + MySQL + Redis API 框架

一个可运行的 Go API 服务骨架，包含 JWT 认证、CORS、Redis 固定窗口限流、结构化请求日志、panic Recovery、MySQL/Redis 连接管理和健康检查。

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
   go run ./cmd/server
   ```

配置项在进程启动时读取。`JWT_SECRET` 必填且至少 32 字节；其他配置见 `.env.example`。配置格式错误、MySQL 或 Redis 不可用时，服务会报错退出。

## 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 进程存活检查 |
| GET | `/readyz` | 检查 MySQL 和 Redis 可用性 |
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
- **日志 / Recovery**：Zap 可读控制台格式日志分别写入 `logs/system.log`、`logs/database.log` 和 `logs/redis.log`，采用统一的时间、级别、分类、调用位置及 `消息 | 字段=值` 格式；应用日志采用清晰的中文事件描述。统一日志器提供 `Info`、`Warn`、`Errorf`、`Fatal`、`Logf` 方法。日志按 100 MB 切分，保留最多 7 个备份和 30 天。数据库日志清晰记录慢查询及错误，SQL 参数默认脱敏；Redis 日志记录命令名、耗时和错误，不记录命令参数。panic 记录堆栈并返回通用 `500`，不会向客户端泄漏异常细节。
- **代理 IP**：默认不信任反向代理头，避免伪造客户端 IP。部署在代理后时，应在 `internal/router/router.go` 中按部署网络配置可信代理。

## 测试与构建

```powershell
go test ./...
go build ./cmd/server
```
