mygin-api/
├── apidoc/                         # API 文档目录
├── cmd/                            # 可执行程序入口
│   ├── schedule/
│   │   └── schedule.go            # 定时任务相关代码
│   └── server/
│       └── main.go                # Gin 服务启动入口
├── debug/                          # 调试辅助目录
├── internal/                       # 项目内部业务代码
│   ├── controller/                 # HTTP 控制器
│   │   ├── order.go               # 订单请求处理
│   │   ├── product.go             # 商品请求处理
│   │   └── user.go                # 用户请求处理
│   ├── middleware/                 # Gin 中间件及测试
│   │   ├── testmw/
│   │   │   ├── cors_test.go       # CORS 中间件测试
│   │   │   └── jwt_test.go        # JWT 中间件测试
│   │   ├── cors.go                # 跨域请求处理中间件
│   │   ├── iprate_limit.go        # 基于 IP 的请求限流中间件
│   │   ├── jwt.go                 # JWT 身份认证中间件
│   │   ├── localhost_only.go      # 仅允许本机访问的中间件
│   │   ├── logging.go             # 请求日志中间件
│   │   ├── ratelimit.go           # 请求限流中间件
│   │   └── recovery.go            # panic 恢复中间件
│   ├── mission/                    # 后台任务
│   │   └── ordertimeout.go        # 订单超时处理任务
│   ├── model/                      # 数据模型
│   │   ├── genm/                   # 生成的数据模型
│   │   │   ├── order_items.gen.go  # 订单项生成模型
│   │   │   ├── orders.gen.go      # 订单生成模型
│   │   │   └── products.gen.go    # 商品生成模型
│   │   ├── orders.go              # 订单数据模型
│   │   └── product.go             # 商品数据模型
│   ├── repository/                 # 数据访问层
│   │   ├── gend/                   # 生成的数据访问代码
│   │   │   ├── gen.go             # 生成代码的查询对象初始化
│   │   │   ├── order_items.gen.go # 订单项数据访问代码
│   │   │   ├── orders.gen.go      # 订单数据访问代码
│   │   │   └── products.gen.go    # 商品数据访问代码
│   │   └── product.go             # 商品数据访问逻辑
│   ├── router/                     # 路由注册
│   │   ├── api/
│   │   │   └── api.go             # API 业务路由
│   │   ├── gen/
│   │   │   └── gen.go             # 生成代码相关路由
│   │   ├── health/
│   │   │   └── health.go          # 健康检查路由
│   │   └── router.go              # 路由初始化与注册
│   └── service/                    # 业务逻辑层
│       ├── auth/
│       │   └── login.go           # 用户登录与认证逻辑
│       ├── order.go               # 订单业务逻辑
│       ├── order_item.go          # 订单项业务逻辑
│       └── product.go             # 商品业务逻辑
├── logs/                           # 运行日志
│   ├── redis.log                  # Redis 相关日志
│   └── system.log                 # 系统运行日志
├── pkg/                            # 可复用的公共组件
│   ├── cache/
│   │   └── redis.go               # Redis 缓存客户端
│   ├── config/                     # 配置加载
│   │   ├── config.go              # 应用配置定义与加载
│   │   └── util.go                # 配置处理辅助函数
│   ├── database/                   # 数据库连接及日志
│   │   ├── logging.go             # 数据库日志配置
│   │   ├── logging_test.go        # 数据库日志测试
│   │   └── mysql.go               # MySQL 连接初始化
│   ├── logging/                    # 日志封装及测试
│   │   ├── logging.go             # 通用日志组件
│   │   └── logging_test.go        # 通用日志组件测试
│   ├── mq/                         # RabbitMQ 封装及测试
│   │   ├── rabbitmq.go            # RabbitMQ 客户端及消息操作
│   │   └── rabbitmq_test.go       # RabbitMQ 组件测试
│   └── utils/                      # 通用工具
│       ├── constant.go            # 通用常量
│       ├── redis_keys.go           # Redis 键名定义
│       ├── response.go             # 统一响应格式
│       ├── response_test.go        # 统一响应格式测试
│       ├── tool.go                 # 通用辅助函数
│       └── work_pool.go            # 并发工作池
├── scripts/                        # 代码生成和 SQL 脚本
│   ├── generate/
│   │   └── generate.go            # 数据访问代码生成脚本
│   └── sql/
│       └── mygin.sql               # 数据库建表及初始化脚本
├── test/                           # 测试代码及测试模型
│   ├── bingfa/
│   │   └── httpbf_test.go         # HTTP 并发测试
│   └── fakermodel/
│       ├── order.go               # 测试用订单模型
│       ├── order_item.go          # 测试用订单项模型
│       └── product.go             # 测试用商品模型
├── .env.example                    # 环境变量配置示例
├── .gitignore                      # Git 忽略规则
├── docker-compose.yml              # 本地依赖服务配置
├── go.mod                          # Go 模块及依赖定义
├── go.sum                          # Go 依赖校验信息
└── README.md                       # 项目使用说明
```

`.env` 是本地环境配置文件，未列入目录树。

项目命名规范
- 源文件命名：全小写 + 下划线
    Go 语言约定源文件名应全部使用‌小写字母‌，多个单词之间使用‌下划线 _‌ 分隔。
- 测试文件命名：固定后缀
    测试文件必须以 _test.go 结尾，且前缀通常与待测试的源文件名保持一致。
- 自动生成文件命名：明确标识
    对于通过 go generate或其他工具自动生成的代码文件，建议使用 _gen.go 或 _generated.go 作为后缀，以便与手写代码区分，并方便在 .gitignore 中管理。
- 特殊前缀文件：构建忽略
    Go 构建工具链会‌忽略‌以 _（下划线）或 .（点）开头的源文件。这些文件不会参与编译。
