# Gin + MySQL + Redis API 框架

一个可运行的 Go API 服务骨架，包含 JWT 认证、CORS、Redis 固定窗口限流、结构化请求日志、panic Recovery、MySQL/Redis 连接管理和健康检查。

## 目录结构

```text
cmd/server/          服务入口与优雅关闭
internal/config/     环境变量配置及校验
internal/database/   GORM + MySQL 初始化
internal/cache/      Redis 初始化
internal/middleware/ JWT、CORS、限流、日志、Recovery
internal/router/     路由及健康检查
```

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
| GET | `/api/v1/me` | 示例 JWT 保护接口，要求 `Authorization: Bearer <token>` |

`/api/v1/me` 的 token 通过 `middleware.IssueToken(userID, secret, issuer, ttl)` 生成。登录、用户模型和授权策略需由业务层实现；框架不会提供固定的账号体系。数据库 schema/migration 同样由业务模块管理。

## 中间件行为

- **JWT**：仅接受 HS256；校验签名、issuer、过期时间和用户 ID。
- **CORS**：`CORS_ORIGINS` 为逗号分隔的精确来源列表；设置 `*` 可允许任意来源，但不启用 credentials。
- **限流**：Redis Lua 脚本原子执行固定窗口计数，默认每 IP 每分钟 100 次；Redis 不可用时返回 `503`，超限返回 `429` 和 `Retry-After`。
- **日志 / Recovery**：`slog` JSON 结构化记录请求；panic 记录堆栈并返回通用 `500`，不会向客户端泄漏异常细节。
- **代理 IP**：默认不信任反向代理头，避免伪造客户端 IP。部署在代理后时，应在 `internal/router/router.go` 中按部署网络配置可信代理。

## 测试与构建

```powershell
go test ./...
go build ./cmd/server
```
