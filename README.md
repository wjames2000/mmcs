# MMCS — 多模型协作系统

[English](#english) | [中文](#chinese)

---

<a name="chinese"></a>

# MMCS 多模型协作系统

基于 Go 语言构建的 AI 多模型协作讨论平台，支持多个 AI 角色以不同范式进行结构化讨论，
实现观点碰撞、方案评审与智能决策。提供桌面端（Wails）和 Web 端双模式。

## 特性

- 🤖 **多角色讨论** — 自定义 AI 专家角色，支持角色技能系统
- 🔄 **四种讨论范式** — 轮询发言、法庭模式、加权评估、自由群聊
- ⏸ **人类介入** — 讨论过程中可暂停、输入意见、恢复讨论
- 📎 **会议材料** — 上传附件作为讨论上下文，首轮自动注入各角色模型
- 📝 **会议纪要** — 自动生成带推理链和决策记录的完整纪要
- 📋 **任务清单** — 大模型自动从讨论中提取行动项，关联工作区
- 🎨 **Markdown 渲染** — 聊天和纪要中的 Markdown 内容完整渲染（表格、代码块等）
- 🔧 **多模型支持** — OpenAI、Claude、DeepSeek、Ollama、OpenRouter 等
- 💻 **桌面 + Web** — Wails 桌面应用 / 浏览器双模式运行

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | **Go 1.25+**, Go 1.22+ ServeMux |
| 桌面端 | **Wails v2** |
| AI 编排 | **Eino** (Graph 节点编排) |
| 数据库 | **PostgreSQL** (含 pgvector) |
| 缓存 | **Redis** |
| 前端 | **React 18**, TypeScript, Tailwind CSS, Vite |
| 日志 | **Zerolog** (结构化 JSON) |
| 部署 | Docker / Docker Compose |

## 快速开始

### 前置条件

- Go 1.25+
- Node.js 18+
- PostgreSQL 16+
- Redis 7+（可选）
- Docker & Docker Compose（可选）

### 使用 Docker Compose

```bash
git clone https://github.com/wjames2000/mmcs.git
cd mmcs

# 配置环境变量
export JWT_SECRET="your-jwt-secret"
export OPENAI_API_KEY="your-api-key"

# 启动所有服务
docker compose up -d

# 查看日志
docker compose logs -f
```

### 本地开发

```bash
# 1. 启动 PostgreSQL 和 Redis
docker compose up -d postgres redis

# 2. 运行数据库迁移
docker exec -i hpds-pgsql psql -U postgres -d mmcs < migrations/001_init.up.sql
docker exec -i hpds-pgsql psql -U postgres -d mmcs < migrations/004_add_session_messages.up.sql
docker exec -i hpds-pgsql psql -U postgres -d mmcs < migrations/005_add_session_materials.up.sql

# 3. 启动 API Server
go run ./cmd/api-server

# 4. 前端开发模式
cd frontend && npm run dev
```

### 桌面端构建

```bash
# 构建前端
cd frontend && npm run build

# 复制前端资源
cp -r dist/ ../cmd/desktop/frontend/dist/

# 构建 Wails 桌面应用
cd ../cmd/desktop && wails build -clean -skipbindings
```

### 验证服务

```bash
# 健康检查
curl http://localhost:8080/healthz

# 用户注册
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"name":"admin","email":"admin@test.com","password":"admin123"}'

# 用户登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@test.com","password":"admin123"}'
```

## 讨论范式

| 范式 | 说明 | 适用场景 |
|------|------|----------|
| **Round Robin** | 轮询发言，按顺序依次讨论 | 开放讨论、头脑风暴 |
| **Court** | 模拟法庭：陈述→审查→回应→总结 | 代码审查、方案评审 |
| **Evaluation** | 加权评估：并行发言→综合打分 | 评分、决策支持 |
| **Free Chat** | 自由群聊：角色自主发言 | 模拟对话、角色扮演 |

## 项目结构

```
mmcs/
├── cmd/
│   ├── api-server/         # HTTP API 服务入口
│   └── desktop/            # Wails 桌面应用
├── internal/
│   ├── api/                # HTTP handlers & 路由
│   │   └── middleware/     # 中间件（CORS、限流、错误）
│   ├── agent/              # Agent 执行器
│   ├── audit/              # 审计日志
│   ├── context/            # 上下文管理
│   ├── minutes/            # 会议纪要生成
│   ├── model_gateway/      # AI 模型网关
│   │   └── provider/       # Provider 实现（OpenAI、Claude、Ollama）
│   ├── orchestrator/       # 讨论编排器（Graph 节点）
│   ├── role/               # 角色与技能系统
│   ├── session/            # 会话管理、消息/材料存储
│   ├── stream/             # SSE 流式推送
│   ├── task/               # 任务管理
│   ├── user/               # 用户认证（JWT）
│   ├── validation/         # 任务验证
│   └── workspace/          # 工作区管理
├── pkg/                    # 可复用工具包
│   ├── postgres/           # PostgreSQL 连接池
│   ├── redis/              # Redis 客户端
│   ├── metrics/            # Prometheus 指标
│   └── logger/             # 日志封装
├── frontend/               # React 前端
│   └── src/
│       ├── components/     # UI 组件
│       ├── pages/          # 页面
│       ├── hooks/          # 自定义 Hooks（useSSE）
│       └── lib/            # API 客户端 & 认证
├── config/                 # 配置文件
├── migrations/             # 数据库迁移
├── deploy/                 # 部署配置
└── docs/                   # 文档
```

## API 概览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/healthz` | 存活检查 |
| GET | `/healthz/ready` | 就绪检查 |
| POST | `/api/v1/auth/register` | 用户注册 |
| POST | `/api/v1/auth/login` | 用户登录 |
| POST | `/api/v1/auth/refresh` | Token 刷新 |
| GET | `/api/v1/workspaces` | 工作区列表 |
| GET | `/api/v1/roles` | 角色列表 |
| POST | `/api/v1/sessions` | 创建会话 |
| GET | `/api/v1/sessions/{id}` | 获取会话 |
| POST | `/api/v1/sessions/{id}/start` | 启动会话 |
| POST | `/api/v1/sessions/{id}/pause` | 暂停会话 |
| POST | `/api/v1/sessions/{id}/resume` | 恢复会话 |
| POST | `/api/v1/sessions/{id}/terminate` | 终止会话 |
| GET | `/api/v1/sessions/{id}/stream` | SSE 流式推送 |
| GET | `/api/v1/sessions/{id}/minutes` | 会议纪要 |
| GET | `/api/v1/sessions/{id}/messages` | 历史消息 |
| GET | `/api/v1/sessions/{id}/tasks` | 提取任务清单 |
| GET | `/api/v1/workspaces/{id}/tasks` | 工作区任务看板 |
| GET | `/api/v1/models/providers` | 模型提供商配置 |

## 配置

配置文件位于 `config/config.yaml`，支持环境变量覆盖：

```yaml
model_gateway:
  providers:
    openai:
      enabled: true
      base_url: "https://api.deepseek.com"
      api_key: "${OPENAI_API_KEY}"
      default_model: "deepseek-v4-flash"
    openrouter:
      enabled: false
      base_url: "https://openrouter.ai/api/v1"
      api_key: "${OPENROUTER_API_KEY}"
    claude:
      enabled: false
      base_url: "https://api.anthropic.com"
      api_key: "${ANTHROPIC_API_KEY}"
      default_model: "claude-sonnet-4-20250514"
```

## 许可证

MIT License

---

<a name="english"></a>

# MMCS — Multi-Model Collaboration System

An AI-powered multi-model discussion platform built with Go. Supports structured discussions
among multiple AI roles using different paradigms, enabling idea exchange, proposal review,
and intelligent decision-making. Dual-mode: desktop (Wails) and web.

## Features

- 🤖 **Multi-Role Discussion** — Custom AI expert roles with skill system
- 🔄 **Four Discussion Paradigms** — Round Robin, Court, Evaluation, Free Chat
- ⏸ **Human Intervention** — Pause, input opinions, and resume discussions
- 📎 **Meeting Materials** — Upload attachments auto-injected into first-round prompts
- 📝 **Meeting Minutes** — Auto-generated with reasoning chain and decision records
- 📋 **Task Extraction** — AI extracts action items linked to workspaces
- 🎨 **Markdown Rendering** — Full Markdown support including tables and code blocks
- 🔧 **Multi-Model Support** — OpenAI, Claude, DeepSeek, Ollama, OpenRouter
- 💻 **Desktop + Web** — Wails desktop app & browser modes

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | **Go 1.25+**, ServeMux |
| Desktop | **Wails v2** |
| AI Orchestration | **Eino** (Graph/DAG) |
| Database | **PostgreSQL** (with pgvector) |
| Cache | **Redis** |
| Frontend | **React 18**, TypeScript, Tailwind CSS, Vite |
| Logging | **Zerolog** (structured JSON) |
| Deployment | Docker / Docker Compose |

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js 18+
- PostgreSQL 16+
- Docker & Docker Compose (optional)

### Using Docker

```bash
git clone https://github.com/wjames2000/mmcs.git
cd mmcs

export JWT_SECRET="your-jwt-secret"
export OPENAI_API_KEY="your-api-key"

docker compose up -d
```

### Local Development

```bash
# 1. Start databases
docker compose up -d postgres redis

# 2. Run migrations
docker exec -i hpds-pgsql psql -U postgres -d mmcs < migrations/001_init.up.sql
docker exec -i hpds-pgsql psql -U postgres -d mmcs < migrations/004_add_session_messages.up.sql
docker exec -i hpds-pgsql psql -U postgres -d mmcs < migrations/005_add_session_materials.up.sql

# 3. Start API server
go run ./cmd/api-server

# 4. Start frontend dev server
cd frontend && npm run dev
```

### Build Desktop App

```bash
cd frontend && npm run build && cp -r dist/ ../cmd/desktop/frontend/dist/
cd ../cmd/desktop && wails build -clean -skipbindings
```

## Discussion Paradigms

| Paradigm | Description | Use Cases |
|----------|-------------|-----------|
| **Round Robin** | Sequential discussion in order | Brainstorming, open discussion |
| **Court** | Simulation: statement → review → response → summary | Code review, proposal evaluation |
| **Evaluation** | Weighted scoring with parallel analysis | Decision support, scoring |
| **Free Chat** | Free-form multi-role chat | Role-playing, simulation |

## License

MIT License
