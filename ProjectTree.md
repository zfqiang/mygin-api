mygin-api/
├── cmd/                    # 项目的主入口
│   └── server/             # 服务启动入口 (可以有多个，如 api, worker)
│       └── main.go         # main 函数，初始化配置、路由、数据库等，并启动 Gin
│
├── internal/               # 私有代码，不可被外部项目导入 (Go 1.4+ 编译器强制约束)
│   ├── controller/         # 控制器层：处理 HTTP 请求，绑定参数，调用 Service，返回响应
│   ├── service/            # 业务逻辑层：核心业务代码，不关心 HTTP 和数据库底层实现
│   ├── repository/         # 数据访问层 (DAO)：与数据库/缓存交互，CRUD 操作
│   ├── model/              # 数据模型：数据库表结构映射 (GORM model) 和 DTO (请求/响应体)
│   ├── middleware/         # Gin 中间件：JWT 鉴权、CORS 跨域、Logger、Recovery、RateLimit 等
│   └── router/             # 路由注册：初始化 Gin Engine，注册路由规则与 Controller 的映射
│
├── pkg/                    # 公共代码：可被内部或外部项目复用的工具包
│   ├── config/             # 配置加载：读取 .yaml / .env / .json 等
│   ├── logger/             # 日志封装：如 Zap、Logrus 的初始化
│   ├── database/           # 数据库初始化：MySQL、PostgreSQL、Redis 连接池等
│   ├── response/           # 统一响应格式：如 { code: 200, msg: "success", data: {} }
│   └── utils/              # 通用工具函数：加密、字符串处理、时间格式化等
│
├── config/                 # 配置文件存放目录
│   ├── config.yaml         # 主配置文件
│   └── config.dev.yaml     # 开发环境配置
│
├── api/                    # API 协议定义 (可选)：Swagger 生成文件，或 Protobuf 文件
├── scripts/                # 脚本文件：数据库迁移脚本、构建部署脚本 (shell, makefile 等)
├── test/                   # 集成测试/黑盒测试数据及用例
│
├── .gitignore              # Git 忽略文件
├── go.mod                  # Go 模块依赖管理
├── go.sum                  # 依赖校验文件
├── Makefile                # 编译与构建指令 (推荐替代直接写 scripts)
└── README.md               # 项目说明文档
