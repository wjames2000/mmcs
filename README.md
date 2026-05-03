# MMCS — 多模型协作系统 (Multi-Model Collaboration System)

基于 Go 语言构建的 AI 多模型协作讨论平台，支持多个 AI 角色以不同范式进行结构化讨论，实现观点碰撞、方案评审与智能决策。

## 技术栈

| 组件 | 技术 | 用途 |
|------|------|------|
| 编程语言 | **Go 1.25+** | 高性能并发后端 |
| HTTP 框架 | **Go 1.22+ ServeMux** | RESTful API 路由 |
| AI 编排 | **Eino** | Graph 节点编排与 DAG 执行 |
| 数据库 | **PostgreSQL (pgvector)** | 业务数据持久化 + 向量存储 |
| 缓存 | **Redis 7** | 会话缓存、SSE Hub、任务队列 |
| 队列 | **Asynq** | 异步任务处理 |
| 监控 | **Prometheus** | 指标采集 |
| 日志 | **Zerolog** | 结构化 JSON 日志 |
| 部署 | **Docker / Kubernetes** | 容器化与编排 |

## 快速开始

### 使用 Docker Compose

```bash
# 克隆仓库
git clone https://github.com/wjames2000/mmcs.git
cd mmcs

# 复制环境变量配置
export JWT_SECRET="your-jwt-secret"
export OPENAI_API_KEY="your-api-key"

# 启动所有服务
docker compose up -d

# 查看日志
docker compose logs -f
```

### 本地开发

```bash
# 确保 PostgreSQL 和 Redis 已运行

# 加载配置并迁移数据库
make migrate-up

# 启动 API Server
go run ./cmd/api-server

# 启动 Agent Worker (另一个终端)
go run ./cmd/agent-worker
```

### 验证服务

```bash
# 健康检查
curl http://localhost:8080/healthz

# 就绪检查
curl http://localhost:8080/healthz/ready

# 依赖详情
curl http://localhost:8080/healthz/deps

# 指标端点
curl http://localhost:8080/metrics
```

## 项目结构

```
mmcs/
├── cmd/                    # 可执行入口
│   ├── api-server/         # HTTP API 服务
│   ├── agent-worker/       # Asynq 任务消费者
│   └── sse-server/         # SSE 推送服务
├── internal/               # 内部业务逻辑
│   ├── agent/              # Agent 执行器
│   ├── api/                # HTTP Handlers & Router
│   ├── audit/              # 审计日志
│   ├── context/            # 上下文管理
│   ├── minutes/            # 会议纪要
│   ├── model_gateway/      # AI 模型网关 (Provider 管理)
│   ├── orchestrator/       # 讨论编排器 (Graph 节点)
│   ├── role/               # 角色与技能系统
│   ├── session/            # 会话管理与 Graph 池
│   ├── stream/             # SSE 流式推送
│   ├── task/               # 任务管理
│   ├── user/               # 用户认证与授权
│   ├── validation/         # 任务验证
│   └── workspace/          # 工作区管理
├── pkg/                    # 可复用工具包
│   ├── api/                # HTTP 客户端
│   ├── crypto/             # 加密工具
│   ├── logger/             # 日志封装
│   ├── metrics/            # Prometheus 指标
│   ├── model/              # 领域模型
│   ├── postgres/           # PostgreSQL 连接池
│   ├── redis/              # Redis 客户端
│   └── util/               # 通用工具
├── config/                 # 配置文件
├── migrations/             # 数据库迁移
├── deploy/                 # 部署配置
│   ├── docker-compose.prod.yml
│   └── k8s/                # Kubernetes 清单
├── docs/                   # 文档
├── Dockerfile              # 多阶段构建
└── go.mod
```

## 讨论范式

MMCS 支持四种讨论范式，每种范式对应不同的 Graph 编排策略：

| 范式 | 说明 | 适用场景 |
|------|------|----------|
| **Round Robin** | 轮询发言，按顺序依次讨论 | 开放式头脑风暴 |
| **Court** | 模拟法庭：陈述→审查→回应→总结 | 代码审查 / 方案评审 |
| **Evaluation** | 加权评估：并行发言→综合打分 | 评分 / 决策支持 |
| **Free Chat** | 自由群聊：角色自主发言 | 模拟对话 / 角色扮演 |

## API 概览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 存活检查 |
| GET | `/healthz/ready` | 就绪检查 |
| GET | `/healthz/deps` | 依赖详情 |
| GET | `/metrics` | Prometheus 指标 |
| POST | `/api/v1/auth/register` | 用户注册 |
| POST | `/api/v1/auth/login` | 用户登录 |
| POST | `/api/v1/auth/refresh` | Token 刷新 |
| POST | `/api/v1/sessions` | 创建会话 |
| GET | `/api/v1/sessions/{id}` | 获取会话 |
| POST | `/api/v1/sessions/{id}/start` | 启动会话 |
| POST | `/api/v1/sessions/{id}/pause` | 暂停会话 |
| POST | `/api/v1/sessions/{id}/resume` | 恢复会话 |
| POST | `/api/v1/sessions/{id}/terminate` | 终止会话 |
| GET | `/api/v1/sessions/{id}/stream` | SSE 流式推送 |

## 开发路线图

### Phase 1 — 项目骨架 & 基础设施
- [x] Go module 初始化、目录结构、配置文件
- [x] PostgreSQL 连接池 & Redis 客户端封装
- [x] 结构化日志 (Zerolog) 与配置管理

### Phase 2 — 用户认证 & 工作区
- [x] JWT 认证 (注册/登录/Token 刷新)
- [x] 认证中间件
- [x] 工作区 CRUD

### Phase 3 — 角色与技能系统
- [x] 角色 CRUD & Prompt 模板
- [x] 技能注册 & 注入
- [x] 技能注入中间件

### Phase 4 — 模型网关 & Provider
- [x] ChatModel 接口抽象
- [x] OpenAI / Ollama Provider 实现
- [x] Gateway 工厂注册表

### Phase 5 — 会话 & 编排引擎
- [x] 会话管理 & 状态机
- [x] Graph 节点 (ContextInit / ExpertSpeak / ModeratorEval / Summarize)
- [x] 四种讨论范式编排器
- [x] 并行专家发言 & 人类介入 (Pause/Resume)

### Phase 6 — 任务系统 & SSE 流式推送
- [x] 任务 CRUD & Asynq 异步队列
- [x] SSE Hub & Bridge (Graph → SSE 事件推送)
- [x] Agent 执行器 & 任务验证
- [x] 流合并 (MergeStream)

### Phase 7 — 生产就绪 (v1.0) ✅
- [x] **性能优化**: Benchmark 测试、Provider 缓存
- [x] **安全加固**: 全链路审计日志
- [x] **运维监控**: 健康检查、Prometheus 指标
- [x] **部署**: Dockerfile 多阶段构建、Docker Compose 生产配置、K8s 清单
- [x] **文档**: README、架构说明
- [x] **测试覆盖**: Benchmark、Cache、Audit、Metrics 测试

## Prometheus 指标

所有指标以 `mmcs_` 为前缀：

| 指标 | 类型 | 标签 | 说明 |
|------|------|------|------|
| `mmcs_http_request_duration_seconds` | Histogram | method, path, status | HTTP 请求耗时 |
| `mmcs_http_request_total` | Counter | method, path, status | HTTP 请求总数 |
| `mmcs_active_sessions` | Gauge | — | 活跃会话数 |
| `mmcs_session_total` | Counter | — | 历史会话总数 |
| `mmcs_provider_call_total` | Counter | provider, model, status | Provider 调用次数 |
| `mmcs_provider_call_duration_seconds` | Histogram | provider, model | Provider 调用耗时 |
| `mmcs_provider_cache_hit_total` | Counter | provider | 缓存命中 |
| `mmcs_provider_cache_miss_total` | Counter | provider | 缓存未命中 |
| `mmcs_agent_execution_duration_seconds` | Histogram | agent_type, status | Agent 执行耗时 |
| `mmcs_agent_execution_total` | Counter | agent_type, status | Agent 执行次数 |
| `mmcs_goroutine_count` | GaugeFunc | — | Goroutine 数量 |
| `mmcs_audit_entry_total` | Counter | — | 审计日志条目数 |
| `mmcs_task_queue_depth` | Gauge | queue | 任务队列深度 |

## 审计日志

审计日志使用内存环形缓冲区存储，支持：

- **Record**: 记录每次节点执行的输入输出
- **Flush**: 批量写入 PostgreSQL `audit_logs` 表
- **GetRecent**: 获取最近 N 条审计日志
- **Prometheus 集成**: 记录审计日志条目总数

## 贡献指南

1. Fork 仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交变更 (`git commit -m 'feat: add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件
