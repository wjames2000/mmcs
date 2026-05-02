# 多模型协作系统 —— WBS 任务分解与优先级排序

## 1. 总体结构

| 阶段 | 迭代 | 周期 | 优先级 | 说明 |
|------|------|------|--------|------|
| Phase 0 | 基础设施搭建 | 1 周 | P0 | 项目骨架、数据库、CI/CD |
| Phase 1 | v0.1 MVP | 4 周 | P0 | 核心闭环：创建会话→角色发言→流式展示 |
| Phase 2 | v0.2 结构化讨论 | 3 周 | P1 | 多轮次、多范式、条件分支 |
| Phase 3 | v0.3 Agent 能力 | 3 周 | P1 | ADK 集成、Agent 参与者、Tool 系统 |
| Phase 4 | v0.4 任务引擎 | 3 周 | P2 | 结论转任务、Todo 面板、自动分配 |
| Phase 5 | v0.5 验证闭环 | 2 周 | P2 | 验证官 Agent、退回重试 |
| Phase 6 | v0.6 增强体验 | 3 周 | P2 | 人类介入、并行流、角色记忆 |
| Phase 7 | v1.0 生产就绪 | 3 周 | P3 | 性能、安全、监控、文档 |

**优先级定义：**

| 等级 | 定义 | 行动 |
|------|------|------|
| P0 | 阻塞性，不完成则后续无法进行 | 立即投入资源 |
| P1 | 核心功能，MVP 必须包含 | 本迭代必须交付 |
| P2 | 重要功能，增强产品价值 | 按资源情况安排 |
| P3 | 锦上添花，生产环境需要 | 可推迟或降级 |

---

## 2. WBS 任务分解

### 2.1 Phase 0：基础设施搭建（1 周，P0）

| WBS 编码 | 任务 | 产出 | 工时 | 前置 |
|----------|------|------|------|------|
| 0.1 | 初始化 Go Module 项目结构 | `go.mod`、目录骨架（cmd/internal/pkg） | 1d | — |
| 0.2 | 搭建 PostgreSQL + Redis 本地开发环境 | `docker-compose.yml`、Dockerfile | 1d | — |
| 0.3 | 配置 golangci-lint、pre-commit hooks | `.golangci.yml`、lint 通过 | 0.5d | 0.1 |
| 0.4 | 实现配置加载模块（viper） | `config/config.go` + `config.yaml` | 1d | 0.1 |
| 0.5 | 实现数据库连接池（pgxpool） | `pkg/postgres/pool.go` | 1d | 0.2 |
| 0.6 | 实现 Redis 客户端初始化 | `pkg/redis/client.go` | 0.5d | 0.2 |
| 0.7 | 实现日志模块（zerolog） | `pkg/logger/logger.go` | 0.5d | 0.1 |
| 0.8 | 编写基础库函数（ULID 生成、时间工具） | `pkg/util/id.go` | 0.5d | 0.1 |
| 0.9 | 配置 GitHub Actions / GitLab CI | `.github/workflows/ci.yml` | 1d | 0.1 |
| 0.10 | 运行 `001_init.up.sql` 初始化数据库 | PostgreSQL 10 张表创建 | 0.5d | 0.5 |

**里程碑 M0：基础框架搭建完成，数据库初始化成功，CI 通过**

---

### 2.2 Phase 1：v0.1 MVP（4 周，P0）

| WBS 编码 | 任务 | 产出 | 工时 | 前置 |
|----------|------|------|------|------|
| **1.1 用户认证** | | | | |
| 1.1.1 | 用户 Repository 层 | `internal/user/repository.go` | 1d | 0.5 |
| 1.1.2 | 用户 Service 层（注册/登录） | `internal/user/service.go` | 1d | 1.1.1 |
| 1.1.3 | JWT 签发与验证 | `internal/user/auth.go` | 1d | 1.1.2 |
| 1.1.4 | 认证中间件 | `internal/user/middleware.go` | 1d | 1.1.3 |
| **1.2 工作区模块** | | | | |
| 1.2.1 | 工作区 Repository | `internal/workspace/repository.go` | 1d | 0.5 |
| 1.2.2 | 工作区 Service（CRUD + 归档） | `internal/workspace/service.go` | 1d | 1.2.1 |
| **1.3 角色引擎模块** | | | | |
| 1.3.1 | 角色 Repository | `internal/role/repository.go` | 1d | 0.5 |
| 1.3.2 | 角色 Service（CRUD） | `internal/role/service.go` | 1d | 1.3.1 |
| 1.3.3 | Skill 注册表定义 | `internal/role/skills.go` | 1d | — |
| 1.3.4 | ChatTemplate 构建器（含 Skill 注入） | `internal/role/template.go` | 2d | 1.3.3 |
| **1.4 模型网关模块** | | | | |
| 1.4.1 | Provider 注册表与工厂 | `internal/model_gateway/gateway.go` | 2d | — |
| 1.4.2 | OpenAI Provider | `internal/model_gateway/provider/openai.go` | 1d | 1.4.1 |
| 1.4.3 | Ollama Provider | `internal/model_gateway/provider/ollama.go` | 1d | 1.4.1 |
| 1.4.4 | API Key 加密存储 | `pkg/crypto/aes.go` | 1d | — |
| **1.5 会话模块** | | | | |
| 1.5.1 | 会话 Repository | `internal/session/repository.go` | 1d | 0.5 |
| 1.5.2 | 会话状态机 | `internal/session/state.go` | 1d | — |
| 1.5.3 | Graph 实例池 | `internal/session/graph_pool.go` | 2d | 1.5.2 |
| 1.5.4 | 会话 Service（CRUD + 启停） | `internal/session/service.go` | 2d | 1.5.1, 1.3.2, 1.4.1 |
| **1.6 讨论编排** | | | | |
| 1.6.1 | Graph 工厂（BuildGraph 接口） | `internal/orchestrator/factory.go` | 2d | 1.3.4, 1.4.1 |
| 1.6.2 | 轮询发言范式 Graph | `internal/orchestrator/round_robin.go` | 3d | 1.6.1 |
| 1.6.3 | GraphNode：context_init | — | 1d | 1.6.1 |
| 1.6.4 | GraphNode：expert_speak | — | 2d | 1.3.4 |
| 1.6.5 | GraphNode：moderator_eval | — | 2d | 1.3.4 |
| 1.6.6 | GraphNode：summarize | — | 1d | — |
| **1.7 流式通信** | | | | |
| 1.7.1 | SSE Hub（Session 级别广播器） | `internal/stream/hub.go` | 2d | — |
| 1.7.2 | Graph Stream → SSE 桥接 | `internal/stream/bridge.go` | 1d | 1.7.1, 1.6.2 |
| **1.8 HTTP 路由** | | | | |
| 1.8.1 | 注册所有路由（gin/chi） | `internal/api/router.go` | 1d | 1.1.4, 1.2.2, 1.3.2, 1.5.4, 1.7.1 |
| 1.8.2 | 统一错误处理中间件 | `internal/api/middleware/error.go` | 1d | — |
| 1.8.3 | 限流中间件 | `internal/api/middleware/ratelimit.go` | 1d | — |
| **1.9 测试** | | | | |
| 1.9.1 | 用户模块单元测试 | — | 1d | 1.1.2 |
| 1.9.2 | 角色模块单元测试 | — | 1d | 1.3.2 |
| 1.9.3 | 会话模块单元测试（含 Mock） | — | 2d | 1.5.4 |
| 1.9.4 | 轮询范式 Graph 集成测试 | — | 2d | 1.6.2, 1.7.1 |

**里程碑 M1：用户可创建会话 → 配置角色 → 启动 → 在 SSE 中看到各角色轮询发言**

---

### 2.3 Phase 2：v0.2 结构化讨论（3 周，P1）

| WBS 编码 | 任务 | 产出 | 工时 | 前置 |
|----------|------|------|------|------|
| **2.1 多轮次循环** | | | | |
| 2.1.1 | ModeratorEval round 判定逻辑 | `internal/orchestrator/eval.go` | 2d | 1.6.5 |
| 2.1.2 | Graph 条件分支（continue/end） | `internal/orchestrator/branch.go` | 1d | 2.1.1 |
| **2.2 模拟法庭范式** | | | | |
| 2.2.1 | CourtSimulation Graph 构建 | `internal/orchestrator/court.go` | 3d | 2.1.2 |
| 2.2.2 | GraphNode：author_statement | — | 1d | 2.2.1 |
| 2.2.3 | GraphNode：author_response | — | 1d | 2.2.1 |
| 2.2.4 | GraphNode：review_phase（并行审查） | — | 2d | 2.2.1 |
| **2.3 加权评估范式** | | | | |
| 2.3.1 | WeightedEvaluation Workflow 构建 | `internal/orchestrator/evaluation.go` | 3d | 2.1.2 |
| 2.3.2 | GraphNode：expert_scoring | — | 1d | 2.3.1 |
| 2.3.3 | GraphNode：critic_challenge | — | 1d | 2.3.1 |
| 2.3.4 | GraphNode：matrix_generation | — | 2d | 2.3.1 |
| **2.4 结果输出** | | | | |
| 2.4.1 | Callbacks 数据收集 | `internal/minutes/builder.go` | 2d | — |
| 2.4.2 | MeetingMinutes 构建器 | `internal/minutes/build.go` | 2d | 2.4.1 |
| 2.4.3 | 推理链生成 | `internal/minutes/reasoning.go` | 2d | 2.4.1 |
| **2.5 测试** | | | | |
| 2.5.1 | 法庭范式 Graph 集成测试 | — | 2d | 2.2.4 |
| 2.5.2 | 加权评估 Workflow 集成测试 | — | 2d | 2.3.4 |
| 2.5.3 | 会议纪要生成测试 | — | 1d | 2.4.2 |

**里程碑 M2：支持 3 种讨论范式 + 多轮次循环 + 自动生成会议纪要**

---

### 2.4 Phase 3：v0.3 Agent 能力（3 周，P1）

| WBS 编码 | 任务 | 产出 | 工时 | 前置 |
|----------|------|------|------|------|
| **3.1 ADK 集成** | | | | |
| 3.1.1 | Agent 类型定义与工厂 | `internal/agent/types.go` | 1d | — |
| 3.1.2 | ChatModelAgent 封装 | `internal/agent/chat_model.go` | 2d | 3.1.1, 1.4.1 |
| 3.1.3 | SupervisorAgent 封装 | `internal/agent/supervisor.go` | 2d | 3.1.1 |
| **3.2 Agent 执行调度** | | | | |
| 3.2.1 | Asynq Client 初始化 | `internal/agent/asynq.go` | 1d | — |
| 3.2.2 | Agent Executor（同步 + 异步） | `internal/agent/executor.go` | 2d | 3.2.1, 3.1.2 |
| 3.2.3 | Asynq Server Worker | `cmd/agent-worker/main.go` | 2d | 3.2.2 |
| **3.3 Tool 系统** | | | | |
| 3.3.1 | Tool 接口与注册表 | `internal/agent/tools.go` | 1d | — |
| 3.3.2 | Tool：CreateTask | — | 1d | 3.3.1 |
| 3.3.3 | Tool：QueryDatabase | — | 1d | 3.3.1 |
| 3.3.4 | Tool：ExecuteCode | — | 2d | 3.3.1 |
| **3.4 自由群聊范式** | | | | |
| 3.4.1 | FreeChat Graph（DeepAgent 模式） | `internal/orchestrator/free_chat.go` | 3d | 3.1.3 |
| **3.5 测试** | | | | |
| 3.5.1 | Agent 单元测试 | — | 1d | 3.2.2 |
| 3.5.2 | Asynq 任务队列测试 | — | 1d | 3.2.3 |
| 3.5.3 | Tool 单元测试 | — | 1d | 3.3.4 |

**里程碑 M3：Agent 可作为讨论参与者 + Tool 调用 + 异步任务分发**

---

### 2.5 Phase 4：v0.4 任务引擎（3 周，P2）

| WBS 编码 | 任务 | 产出 | 工时 | 前置 |
|----------|------|------|------|------|
| **4.1 任务模块** | | | | |
| 4.1.1 | 任务 Repository | `internal/task/repository.go` | 1d | 0.5 |
| 4.1.2 | 任务 Service（CRUD + 筛选） | `internal/task/service.go` | 1d | 4.1.1 |
| 4.1.3 | 任务状态机 | `internal/task/state.go` | 0.5d | — |
| **4.2 结论转任务** | | | | |
| 4.2.1 | PlanExecuteAgent 封装 | `internal/task/planner.go` | 2d | 3.1.2 |
| 4.2.2 | MeetingMinutes → Task 提取逻辑 | `internal/task/extraction.go` | 3d | 4.2.1, 2.4.2 |
| 4.2.3 | 任务自动分配逻辑 | `internal/task/assign.go` | 2d | 4.1.2, 3.2.2 |
| **4.3 Todo List 面板 API** | | | | |
| 4.3.1 | 工作区任务聚合查询 | `internal/workspace/tasks.go` | 1d | 4.1.2 |
| 4.3.2 | 任务 SSE 事件推送（task.created/updated） | `internal/stream/task_events.go` | 1d | 1.7.1 |
| **4.4 测试** | | | | |
| 4.4.1 | 任务提取集成测试 | — | 2d | 4.2.2 |
| 4.4.2 | 任务分配逻辑测试 | — | 1d | 4.2.3 |

**里程碑 M4：讨论结束后自动提取任务 → 写入 Todo 面板 → 分配 Agent 执行**

---

### 2.6 Phase 5：v0.5 验证闭环（2 周，P2）

| WBS 编码 | 任务 | 产出 | 工时 | 前置 |
|----------|------|------|------|------|
| **5.1 验证模块** | | | | |
| 5.1.1 | 验证结果 Repository | `internal/validation/repository.go` | 0.5d | 0.5 |
| 5.1.2 | 验证官 Agent 封装 | `internal/validation/agent.go` | 2d | 3.1.2 |
| 5.1.3 | 验证 Service（触发 + 判定 + 回写） | `internal/validation/service.go` | 2d | 5.1.1, 5.1.2 |
| **5.2 退回重试机制** | | | | |
| 5.2.1 | 重新分配逻辑（rejected → pending） | `internal/task/retry.go` | 1d | 4.1.2 |
| 5.2.2 | 验证状态同步到 Todo 面板 | `internal/stream/validation_events.go` | 1d | 1.7.1 |
| **5.3 测试** | | | | |
| 5.3.1 | 验证官 Agent 单元测试 | — | 1d | 5.1.2 |
| 5.3.2 | 验证闭环 E2E 测试 | — | 2d | 5.1.3, 5.2.1 |

**里程碑 M5：任务执行完成 → 验证官判定 → passed/rejected → 同步状态到面板**

---

### 2.7 Phase 6：v0.6 增强体验（3 周，P2）

| WBS 编码 | 任务 | 产出 | 工时 | 前置 |
|----------|------|------|------|------|
| **6.1 人类介入** | | | | |
| 6.1.1 | Graph Interrupt 节点标记 | — | 1d | 1.6.1 |
| 6.1.2 | Session Pause/Resume 实现 | `internal/session/interrupt.go` | 2d | 6.1.1 |
| 6.1.3 | 用户插入消息 → Resume 逻辑 | — | 1d | 6.1.2 |
| **6.2 并行流合并** | | | | |
| 6.2.1 | 多角色并行 Stream 合并 | `internal/stream/merge.go` | 2d | 1.7.1 |
| 6.2.2 | SSE 事件时间戳排序 | `internal/stream/sorter.go` | 1d | 6.2.1 |
| **6.3 上下文管理** | | | | |
| 6.3.1 | ContextManager（Token 计数 + 压缩） | `internal/context/manager.go` | 2d | — |
| 6.3.2 | Summarize 压缩策略实现 | `internal/context/summarize.go` | 2d | 6.3.1 |
| 6.3.3 | Sliding Window 压缩策略实现 | `internal/context/sliding.go` | 1d | 6.3.1 |
| **6.4 角色记忆 RAG** | | | | |
| 6.4.1 | 消息向量化写入（pgvector） | `internal/context/embedding.go` | 2d | 6.3.1 |
| 6.4.2 | 记忆召回（Retriever 集成） | `internal/context/retriever.go` | 2d | 6.4.1 |
| **6.5 测试** | | | | |
| 6.5.1 | Interrupt/Resume 集成测试 | — | 2d | 6.1.2 |
| 6.5.2 | ContextManager 单元测试 | — | 1d | 6.3.1 |
| 6.5.3 | 角色记忆测试 | — | 1d | 6.4.2 |

**里程碑 M6：用户可打断讨论插入意见 + 多角色并行流展示 + 长对话上下文自动压缩**

---

### 2.8 Phase 7：v1.0 生产就绪（3 周，P3）

| WBS 编码 | 任务 | 产出 | 工时 | 前置 |
|----------|------|------|------|------|
| **7.1 性能优化** | | | | |
| 7.1.1 | Graph Pool 基准测试与调优 | — | 2d | 1.5.3 |
| 7.1.2 | SSE Hub 性能测试（1000 并发客户端） | — | 2d | 1.7.1 |
| 7.1.3 | 数据库查询优化（慢查询分析 + 索引优化） | — | 2d | — |
| 7.1.4 | Provider 调用缓存（重复请求去重） | `internal/model_gateway/cache.go` | 2d | 1.4.1 |
| **7.2 安全加固** | | | | |
| 7.2.1 | 安全审计：SQL 注入 / XSS / CSRF 检查 | — | 2d | — |
| 7.2.2 | 全链路审计日志完善 | `internal/audit/callback.go` | 2d | 2.4.1 |
| 7.2.3 | 数据加密：API Key + 敏感字段 | — | 1d | 1.4.4 |
| **7.3 运维监控** | | | | |
| 7.3.1 | Health Check 端点 | `/healthz` + 依赖检查 | 1d | — |
| 7.3.2 | Prometheus 指标暴露 | `pkg/metrics/metrics.go` | 2d | — |
| 7.3.3 | 日志级别动态调整 | — | 1d | 0.7 |
| 7.3.4 | 慢 Provider 调用告警 | — | 1d | 7.3.2 |
| **7.4 部署** | | | | |
| 7.4.1 | Docker 多阶段构建 | `Dockerfile` | 1d | — |
| 7.4.2 | docker-compose 生产配置 | `docker-compose.prod.yml` | 1d | 7.4.1 |
| 7.4.3 | Kubernetes 部署清单 | `k8s/` 目录 | 2d | 7.4.1 |
| 7.4.4 | 数据库迁移 CI 集成 | — | 1d | 0.9 |
| **7.5 文档** | | | | |
| 7.5.1 | API 接口文档补全 | `docs/06 API 接口说明书.md` | 2d | — |
| 7.5.2 | 部署运维手册 | `docs/07 部署运维手册.md` | 2d | 7.4.2 |
| 7.5.3 | README + 贡献指南 | `README.md` + `CONTRIBUTING.md` | 1d | — |

**里程碑 M7：系统达到生产环境发布标准，文档完备**

---

## 3. 优先级矩阵

将所有任务按「价值」和「风险/依赖」两个维度归入优先级矩阵：

```
                  高风险 / 高依赖    低风险 / 低依赖
                ┌─────────────────┬─────────────────┐
   高价值      │ P0 (立即做)      │ P1 (尽快做)      │
              │ Phase 0 基础设施 │ Phase 2 结构化   │
              │ Phase 1 MVP      │ Phase 3 Agent    │
              │ 认证 / 工作区    │ 法庭 / 评估范式  │
              │ 角色 / 模型网关  │ Asynq / Tool    │
              │ 会话 + Graph     │ 自由群聊         │
              │ SSE              │                  │
                ├─────────────────┼─────────────────┤
   低价值      │ P2 (按计划做)    │ P3 (有余力做)    │
              │ Phase 4 任务引擎 │ Phase 7 生产就绪 │
              │ Phase 5 验证闭环 │ 性能 / 安全      │
              │ Phase 6 增强体验 │ 监控 / 部署      │
              │ 上下文管理       │ 文档             │
              └─────────────────┴─────────────────┘
```

---

## 4. 关键依赖路径

```
Phase 0 ──→ Phase 1 ──→ Phase 2 ──→ Phase 3 ──→ Phase 4 ──→ Phase 5
  (基础)     (MVP)      (多范式)     (Agent)     (任务)      (验证)
                            │                       │
                            │                       │
                            └── Phase 6 ────────────┘
                              (增强体验)

Phase 3 ───── Phase 4 ───── Phase 5 ───── Phase 7
  (Agent)      (任务)        (验证)        (生产就绪)

Phase 7 可并行:
  ├── 性能优化 — 依赖 Phase 1-6 完成后做基准测试
  ├── 安全加固 — 可穿插在所有阶段中做
  ├── 运维监控 — Phase 1 完成后即可开始
  └── 文档     — 随各阶段同步编写
```

---

## 5. 资源分配建议

| 角色 | 建议人数 | 负责范畴 |
|------|---------|---------|
| 后端 A | 1 | Phase 0 + 认证/工作区/角色 + 数据库 |
| 后端 B | 1 | 会话/编排/Graph 节点 + 流式通信 |
| 后端 C | 1 | 模型网关/Agent + 任务/验证 |
| 前端 | 1 | React/Next.js UI（与后端 A/B/C 并行） |
| QA | 0.5 | 集成测试 + E2E 测试（贯穿全周期） |

**并行策略：**
- Phase 1：后端 A 做认证+工作区+角色，后端 B 做会话+编排+SSE，后端 C 做模型网关
- Phase 2：后端 B 做多范式，后端 C 做结果输出
- Phase 3：后端 C 做 Agent，后端 B 做自由群聊
- Phase 4-5：后端 C 主导，后端 B 配合 SSE 事件
- Phase 6：后端 B 带头，后端 C 配合上下文

---

## 6. 交付物清单

| WBS | 交付物 | 类型 |
|-----|--------|------|
| 0.1~0.10 | 项目骨架 / CI 配置 / 数据库初始化 | 技术 |
| 1.1~1.8 | 用户认证 / 工作区 / 角色 / 会话 / SSE API | 功能 |
| 1.9 | MVP 测试套件 | 测试 |
| 2.1~2.4 | 多范式 Graph / 会议纪要 | 功能 |
| 2.5 | 结构化讨论测试套件 | 测试 |
| 3.1~3.4 | Agent 系统 / Asynq 调度 / Tool 系统 | 功能 |
| 3.5 | Agent 测试套件 | 测试 |
| 4.1~4.3 | 任务系统 / Todo 面板 API / 任务提取 | 功能 |
| 4.4 | 任务引擎测试套件 | 测试 |
| 5.1~5.2 | 验证官 Agent / 退回重试 | 功能 |
| 5.3 | 验证闭环测试套件 | 测试 |
| 6.1~6.4 | 人类介入 / 并行流 / 上下文管理 / RAG | 功能 |
| 6.5 | 增强体验测试套件 | 测试 |
| 7.1~7.5 | 性能报告 / 安全审计 / 监控 / 文档 | 质量 |
