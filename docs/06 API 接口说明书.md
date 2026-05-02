# 多模型协作系统 —— API 接口说明书

## 1. 引言

### 1.1 编写目的

本文档为「多模型圆桌会议系统」的 API 接口说明书，定义所有**对外 REST API 的请求/响应格式、认证方式、错误码、SSE 协议、以及典型调用流程**，作为前端开发和第三方集成的依据。

### 1.2 基础信息

| 项目 | 说明 |
|------|------|
| Base URL | `https://api.mmcs.example.com/api/v1` |
| 协议 | HTTPS |
| 数据格式 | JSON (Content-Type: application/json) |
| 字符集 | UTF-8 |
| 认证方式 | Bearer JWT |
| 分页 | 默认 page=1, page_size=20，最大值 100 |

---

## 2. 通用规范

### 2.1 通用响应格式

```json
// 成功
{
  "code": 0,
  "message": "success",
  "data": { ... },
  "meta": {
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}

// 错误
{
  "code": 40001,
  "message": "参数校验失败",
  "request_id": "req_abc123"
}
```

### 2.2 ID 格式

所有业务 ID 采用 `{前缀}_{ULID}` 格式：

| 前缀 | 资源 |
|------|------|
| `ws_` | Workspace (工作区) |
| `s_` | Session (会话) |
| `r_` | Role (角色) |
| `sr_` | SessionRole (会话角色绑定) |
| `msg_` | Message (消息) |
| `t_` | Task (任务) |
| `vr_` | ValidationResult (验证结果) |
| `mp_` | ModelProvider (模型配置) |

示例：`s_01JQZ5ZJY8X7K2W6V9N3M4P5QR`

### 2.3 认证

所有 API 请求需在 HTTP Header 中携带 JWT Token：

```
Authorization: Bearer eyJhbGciOiJSUzI1NiIs...
```

Token 通过 `POST /api/v1/auth/login` 获取，有效期 15 分钟。过期后使用 Refresh Token 续期：

```
POST /api/v1/auth/refresh
Authorization: Bearer <refresh_token>
```

### 2.4 公共错误码

| code | HTTP | message | 说明 |
|------|------|---------|------|
| 0 | 200 | success | 成功 |
| 40001 | 400 | 参数校验失败 | 请求参数不合法 |
| 40002 | 400 | 资源已存在 | 创建重复资源 |
| 40101 | 401 | 未认证 | 缺少或无效的 Token |
| 40102 | 401 | Token 已过期 | 需要刷新 Token |
| 40301 | 403 | 无权限 | 无权操作该资源 |
| 40401 | 404 | 资源不存在 | ID 无效 |
| 40901 | 409 | 状态冲突 | Session 状态不允许该操作 |
| 42901 | 429 | 请求太频繁 | 触发限流 |
| 50001 | 500 | 服务器内部错误 | 请联系管理员 |
| 50301 | 503 | 服务暂不可用 | 服务正在重启或过载 |

---

## 3. 认证 API

### POST /api/v1/auth/login

用户登录，返回 JWT Token。

```json
// Request
{
  "email": "user@example.com",
  "password": "your-password"
}

// Response 200
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJSUzI1NiIs...",
    "expires_in": 900,
    "token_type": "Bearer",
    "user": {
      "id": "u_xxx",
      "name": "张三",
      "email": "user@example.com",
      "avatar_url": "https://..."
    }
  }
}

// Response 401
{
  "code": 40101,
  "message": "邮箱或密码错误",
  "request_id": "req_xxx"
}
```

### POST /api/v1/auth/refresh

使用 Refresh Token 获取新的 Access Token。

```json
// Request (Header)
Authorization: Bearer <refresh_token>

// Response 200
{
  "code": 0,
  "data": {
    "access_token": "eyJhbGciOiJSUzI1NiIs...",
    "expires_in": 900
  }
}
```

### POST /api/v1/auth/register

注册新用户。

```json
// Request
{
  "name": "张三",
  "email": "user@example.com",
  "password": "your-password"
}

// Response 201
{
  "code": 0,
  "data": {
    "id": "u_xxx",
    "name": "张三",
    "email": "user@example.com",
    "created_at": "2026-05-02T10:00:00Z"
  }
}
```

---

## 4. 工作区 API

### POST /api/v1/workspaces

创建新的工作区。

```json
// Request
{
  "name": "Q2 产品需求评审",
  "description": "2026 Q2 所有产品需求的讨论和评审工作区",
  "mode": "shared",
  "members": ["u_001", "u_002", "u_003"]
}

// Response 201
{
  "code": 0,
  "data": {
    "id": "ws_xxx",
    "name": "Q2 产品需求评审",
    "mode": "shared",
    "status": "active",
    "created_at": "2026-05-02T10:00:00Z"
  }
}
```

### GET /api/v1/workspaces

获取当前用户所属的工作区列表。

```json
// Response 200
{
  "code": 0,
  "data": [
    {
      "id": "ws_xxx",
      "name": "Q2 产品需求评审",
      "description": "2026 Q2 所有产品需求的讨论和评审工作区",
      "mode": "shared",
      "status": "active",
      "session_count": 5,
      "task_summary": { "pending": 3, "completed": 7 },
      "creator_id": "u_xxx",
      "created_at": "2026-05-02T10:00:00Z"
    }
  ],
  "meta": { "total": 10, "page": 1, "page_size": 20 }
}
```

Query 参数：`?status=active&page=1&page_size=20`

### GET /api/v1/workspaces/:id

获取工作区详情及关联的会话列表。

```json
// Response 200
{
  "code": 0,
  "data": {
    "id": "ws_xxx",
    "name": "Q2 产品需求评审",
    "description": "...",
    "mode": "shared",
    "status": "active",
    "members": ["u_001", "u_002"],
    "sessions": [
      {
        "id": "s_xxx",
        "title": "用户登录模块代码评审",
        "paradigm": "court",
        "status": "ended",
        "created_at": "2026-05-01T08:00:00Z"
      },
      {
        "id": "s_yyy",
        "title": "技术选型：自研 vs 采购",
        "paradigm": "evaluation",
        "status": "running",
        "created_at": "2026-05-02T09:00:00Z"
      }
    ],
    "creator_id": "u_xxx",
    "created_at": "2026-05-02T10:00:00Z",
    "updated_at": "2026-05-02T10:00:00Z"
  }
}
```

### PATCH /api/v1/workspaces/:id

更新工作区信息（部分更新）。

```json
// Request
{
  "name": "Q2 产品与技术评审",
  "members": ["u_001", "u_002", "u_003", "u_004"]
}

// Response 200
{
  "code": 0,
  "data": { "id": "ws_xxx", "updated_at": "2026-05-02T11:00:00Z" }
}
```

### POST /api/v1/workspaces/:id/archive

归档工作区。归档后，工作区内所有运行中的会话将被强制终止，不可再创建新会话。

```json
// Response 200
{
  "code": 0,
  "data": { "id": "ws_xxx", "status": "archived" }
}
```

### GET /api/v1/workspaces/:id/tasks

获取工作区下所有任务的聚合视图。

```json
// Response 200
{
  "code": 0,
  "data": {
    "workspace_id": "ws_xxx",
    "total": 15,
    "by_status": {
      "pending": 3,
      "in_progress": 2,
      "reviewing": 1,
      "completed": 8,
      "rejected": 1
    },
    "tasks": [
      {
        "id": "t_001",
        "title": "修复用户登录模块 SQL 注入漏洞",
        "status": "in_progress",
        "priority": "critical",
        "assigned_agent": "agent_security_01",
        "session_id": "s_xxx",
        "created_at": "2026-05-01T08:30:00Z"
      }
    ]
  }
}
```

---

## 5. 会话 API

### POST /api/v1/sessions

创建新的讨论会话。会话创建后处于 `draft` 状态，需调用 `Start` 启动。

```json
// Request
{
  "workspace_id": "ws_xxx",
  "title": "用户登录模块代码评审",
  "paradigm": "court",
  "roles": [
    {
      "role_id": "r_global_security",
      "model": {
        "provider": "anthropic",
        "model_name": "claude-3.5-sonnet",
        "parameters": { "temperature": 0.3 }
      }
    },
    {
      "role_id": "r_global_perf"
    },
    {
      "role_id": "r_global_maintain",
      "model": {
        "provider": "openai",
        "model_name": "gpt-4o",
        "parameters": { "temperature": 0.5 }
      }
    }
  ],
  "config": {
    "max_rounds": 5,
    "round_timeout": 300,
    "context_enabled": true,
    "auto_terminate": true
  }
}

// Response 201
{
  "code": 0,
  "data": {
    "id": "s_xxx",
    "title": "用户登录模块代码评审",
    "paradigm": "court",
    "status": "draft",
    "created_at": "2026-05-02T10:00:00Z"
  }
}
```

### GET /api/v1/sessions/:id

获取会话详情，包括轮次摘要、角色信息、会议纪要（如已结束）。

```json
// Response 200
{
  "code": 0,
  "data": {
    "id": "s_xxx",
    "workspace_id": "ws_xxx",
    "title": "用户登录模块代码评审",
    "paradigm": "court",
    "status": "ended",
    "rounds": [
      {
        "round": 1,
        "phase": "statement",
        "messages": 3,
        "started_at": "2026-05-02T10:01:00Z"
      },
      {
        "round": 2,
        "phase": "review",
        "messages": 12,
        "started_at": "2026-05-02T10:05:00Z"
      }
    ],
    "roles": [
      {
        "role_id": "r_global_security",
        "role_name": "安全审查员",
        "model_name": "claude-3.5-sonnet",
        "provider": "anthropic"
      }
    ],
    "config": {
      "max_rounds": 5,
      "round_timeout": 300,
      "context_enabled": true
    },
    "creator_id": "u_xxx",
    "created_at": "2026-05-02T10:00:00Z",
    "started_at": "2026-05-02T10:00:30Z",
    "ended_at": "2026-05-02T10:15:00Z"
  }
}
```

### POST /api/v1/sessions/:id/start

启动会话。成功后状态变为 `running`，同时返回 SSE 订阅 URL。

```json
// Response 200
{
  "code": 0,
  "data": {
    "session_id": "s_xxx",
    "status": "running",
    "stream_url": "/api/v1/sessions/s_xxx/stream"
  }
}

// Response 409 (状态冲突)
{
  "code": 40901,
  "message": "会话已结束，无法启动",
  "request_id": "req_xxx"
}
```

### POST /api/v1/sessions/:id/pause

暂停正在运行的会话。通过 Eino Interrupt 机制实现。

```json
// Response 200
{
  "code": 0,
  "data": {
    "session_id": "s_xxx",
    "status": "paused",
    "interrupt_at": "moderator_eval"
  }
}
```

### POST /api/v1/sessions/:id/resume

恢复暂停的会话。用户可附带一条消息（如回复当前讨论）。

```json
// Request
{
  "message": "关于 SQL 注入的问题，我们已经使用了参数化查询，请确认是否仍有风险"
}

// Response 200
{
  "code": 0,
  "data": {
    "session_id": "s_xxx",
    "status": "running"
  }
}
```

### POST /api/v1/sessions/:id/terminate

强制终止会话。立即停止 Graph 执行，触发 Summarize 节点生成纪要。

```json
// Response 200
{
  "code": 0,
  "data": {
    "session_id": "s_xxx",
    "status": "ended",
    "minutes": { ... }   // MeetingMinutes 完整结构
  }
}
```

### POST /api/v1/sessions/:id/interrupt

人类在指定环节插入发言。需要先 Pause 再进行介入。也可以直接在 Resume 时附带消息。

```json
// Request
{
  "node_name": "review_phase",
  "message": "请额外关注 OAuth 集成部分的安全性"
}

// Response 200
{
  "code": 0,
  "data": {
    "session_id": "s_xxx",
    "status": "paused",
    "message": "等待后续 Resume"
  }
}
```

### GET /api/v1/sessions/:id/messages

获取会话的消息记录，支持按轮次筛选。

```json
// Query 参数: ?round_id=round_xxx&author_type=ai&page=1&page_size=50

// Response 200
{
  "code": 0,
  "data": [
    {
      "id": "msg_001",
      "round_id": "round_s_xxx_1",
      "role_id": "r_global_security",
      "author_type": "ai",
      "role_name": "安全审查员",
      "content": "从安全角度来看，该模块存在 SQL 注入风险...",
      "tokens": 156,
      "created_at": "2026-05-02T10:02:00Z"
    }
  ],
  "meta": { "total": 50, "page": 1, "page_size": 50 }
}
```

### GET /api/v1/sessions/:id/minutes

获取会话的完整会议纪要（含推理链、决策矩阵、分歧记录）。

```json
// Response 200
{
  "code": 0,
  "data": {
    "session_id": "s_xxx",
    "title": "用户登录模块代码评审",
    "paradigm": "court",
    "participants": ["安全审查员", "性能分析员", "可维护性评估员"],
    "started_at": "2026-05-02T10:00:30Z",
    "ended_at": "2026-05-02T10:15:00Z",
    "rounds": [
      {
        "round": 1,
        "phase": "statement",
        "messages": [ ... ]
      }
    ],
    "decisions": [
      {
        "topic": "SQL 注入防护",
        "conclusion": "现有参数化查询已覆盖，但需补充输入长度校验",
        "confidence": "high"
      }
    ],
    "disagreements": [
      {
        "topic": "缓存策略",
        "positions": {
          "性能分析员": "建议引入 Redis 缓存",
          "安全审查员": "缓存敏感信息需额外加密"
        },
        "resolution": "缓存非敏感数据，敏感数据绕过缓存"
      }
    ],
    "conclusion": "代码整体质量良好，存在 3 个中风险项需要修复...",
    "reasoning_chain": { ... }
  }
}
```

---

## 6. SSE 流式 API

### GET /api/v1/sessions/:id/stream

订阅会话的流式事件。使用 SSE (Server-Sent Events) 协议。

**请求头：**
```
Accept: text/event-stream
Cache-Control: no-cache
Authorization: Bearer <token>
```

**响应头：**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

**事件类型：**

| 事件名 | 说明 | Data 字段 |
|--------|------|-----------|
| `session.created` | 会话创建 | `{session_id, title, paradigm}` |
| `round.start` | 新轮次开始 | `{round, phase, started_at}` |
| `role.speak` | 角色发言流式 token | `{role_id, role_name, token, seq, round}` |
| `role.done` | 角色发言结束 | `{role_id, role_name, full_content, tokens, latency_ms}` |
| `round.eval` | 主持人评估结果 | `{round, should_continue, summary, consensus, disagreements}` |
| `session.paused` | 会话暂停 | `{session_id, interrupt_at}` |
| `session.resumed` | 会话恢复 | `{session_id, resumed_at}` |
| `session.ended` | 会话结束 | `{session_id, duration_s, minutes_url}` |
| `task.created` | 任务创建 | `{task_id, title, priority, session_id}` |
| `task.updated` | 任务状态变更 | `{task_id, status, session_id}` |
| `error` | 运行时错误 | `{code, message, node_name}` |

**完整 SSE 流示例：**

```
event: round.start
data: {"round":1,"phase":"brainstorm","started_at":"2026-05-02T10:01:00Z"}
id: evt_r1_start

event: role.speak
data: {"role_id":"r1","role_name":"安全审查员","token":"该","seq":1,"round":1}
id: evt_r1_t001

event: role.speak
data: {"role_id":"r1","role_name":"安全审查员","token":"模块","seq":2,"round":1}
id: evt_r1_t002

... (若干 token 事件)

event: role.done
data: {"role_id":"r1","role_name":"安全审查员","full_content":"该模块存在 SQL 注入风险...","tokens":156,"latency_ms":3200}
id: evt_r1_end

event: role.speak
data: {"role_id":"r2","role_name":"性能分析员","token":"从","seq":1,"round":1}
id: evt_r2_t001

... (其他角色发言)

event: round.eval
data: {"round":1,"should_continue":true,"summary":"本轮各角色提出了 5 个问题...","consensus":["SQL 注入需优先修复"],"disagreements":["缓存策略的选择"]}
id: evt_r1_eval

event: round.start
data: {"round":2,"phase":"response","started_at":"2026-05-02T10:06:00Z"}
id: evt_r2_start

...

event: session.ended
data: {"session_id":"s_xxx","duration_s":900,"minutes_url":"/api/v1/sessions/s_xxx/minutes"}
id: evt_end
```

**客户端重连：**

SSE 连接意外断开时，客户端应携带 `Last-Event-ID` 重连：

```
GET /api/v1/sessions/:id/stream
Last-Event-ID: evt_r1_t050
```

服务端会从该事件之后继续推送，保证不丢事件。

---

## 7. 角色 API

### GET /api/v1/roles

获取角色列表。支持筛选全局模板和用户自建角色。

```json
// Query: ?is_global=true&page=1&page_size=20

// Response 200
{
  "code": 0,
  "data": [
    {
      "id": "r_global_security",
      "name": "安全审查员",
      "title": "Security Auditor",
      "traits": { "aggressiveness": 7, "optimism": 3, "creativity": 4, "detail": 9 },
      "expertise": ["网络安全", "应用安全", "密码学"],
      "speaking_style": "严谨、直接、引用标准规范",
      "skills": ["security-audit", "chinese-code-review"],
      "is_global": true,
      "created_at": "2026-01-01T00:00:00Z"
    }
  ],
  "meta": { "total": 6, "page": 1, "page_size": 20 }
}
```

### POST /api/v1/roles

创建自定义角色。

```json
// Request
{
  "name": "UX 体验专家",
  "title": "User Experience Designer",
  "traits": { "aggressiveness": 3, "optimism": 8, "creativity": 9, "detail": 6 },
  "expertise": ["用户体验", "交互设计", "无障碍"],
  "speaking_style": "用户友好、同理心强",
  "system_prompt": "你是一名 UX 体验专家...",
  "skills": ["chinese-code-review", "security-audit"],
  "default_model": {
    "provider": "openai",
    "model_name": "gpt-4o",
    "parameters": { "temperature": 0.7 }
  }
}

// Response 201
{
  "code": 0,
  "data": {
    "id": "r_xxx",
    "name": "UX 体验专家",
    "title": "User Experience Designer",
    "traits": { "aggressiveness": 3, "optimism": 8, "creativity": 9, "detail": 6 },
    "expertise": ["用户体验", "交互设计", "无障碍"],
    "skills": ["chinese-code-review", "security-audit"],
    "is_global": false,
    "creator_id": "u_xxx",
    "created_at": "2026-05-02T12:00:00Z"
  }
}
```

### GET /api/v1/roles/:id

获取角色详情（含完整 system_prompt）。

```json
// Response 200
{
  "code": 0,
  "data": {
    "id": "r_xxx",
    "name": "UX 体验专家",
    "title": "User Experience Designer",
    "traits": { ... },
    "expertise": ["用户体验", "交互设计", "无障碍"],
    "speaking_style": "用户友好、同理心强",
    "system_prompt": "你是一名 UX 体验专家...",
    "skills": ["chinese-code-review", "security-audit"],
    "default_model": { "provider": "openai", "model_name": "gpt-4o" },
    "is_global": false,
    "creator_id": "u_xxx",
    "created_at": "2026-05-02T12:00:00Z",
    "updated_at": "2026-05-02T12:00:00Z"
  }
}
```

### PUT /api/v1/roles/:id

更新角色（全量更新）。

```json
// Request (同 POST /roles)
// Response 200
{
  "code": 0,
  "data": { "id": "r_xxx", "updated_at": "..." }
}
```

### DELETE /api/v1/roles/:id

删除角色。如角色已被会话引用，则不允许删除。

```json
// Response 200
{
  "code": 0,
  "message": "success"
}

// Response 409
{
  "code": 40902,
  "message": "该角色正在被 3 个会话使用，无法删除",
  "request_id": "req_xxx"
}
```

---

## 8. 任务 API

### GET /api/v1/tasks

获取任务列表。支持按工作区、状态、优先级筛选。

```json
// Query: ?workspace_id=ws_xxx&status=pending&priority=critical&page=1&page_size=20

// Response 200
{
  "code": 0,
  "data": [
    {
      "id": "t_001",
      "session_id": "s_xxx",
      "workspace_id": "ws_xxx",
      "title": "修复 SQL 注入漏洞",
      "description": "在用户登录模块添加参数校验",
      "acceptance_criteria": "1. 所有输入参数长度校验通过 2. 特殊字符过滤通过",
      "status": "in_progress",
      "priority": "critical",
      "assigned_agent": "agent_sec_01",
      "assigned_by": "",
      "created_at": "2026-05-02T10:15:00Z"
    }
  ],
  "meta": { "total": 5, "page": 1, "page_size": 20 }
}
```

### POST /api/v1/tasks

手动创建任务（也可以由讨论结论自动提取）。

```json
// Request
{
  "session_id": "s_xxx",
  "title": "优化数据库查询性能",
  "description": "用户列表页查询超过 5 秒，需优化索引",
  "acceptance_criteria": "查询耗时低于 500ms",
  "priority": "high"
}

// Response 201
{
  "code": 0,
  "data": {
    "id": "t_xxx",
    "session_id": "s_xxx",
    "status": "pending",
    "created_at": "2026-05-02T10:20:00Z"
  }
}
```

### GET /api/v1/tasks/:id

获取任务详情（含验证结果）。

```json
// Response 200
{
  "code": 0,
  "data": {
    "id": "t_xxx",
    "session_id": "s_xxx",
    "title": "优化数据库查询性能",
    "description": "...",
    "acceptance_criteria": "查询耗时低于 500ms",
    "status": "completed",
    "priority": "high",
    "assigned_agent": "agent_perf_01",
    "assigned_by": "",
    "validation_result": {
      "id": "vr_xxx",
      "validator": "validator_agent",
      "verdict": "passed",
      "reason": "经测试，查询耗时从 5.2s 降至 320ms，满足验收标准",
      "detail": {
        "checked_items": [
          { "item": "查询耗时低于 500ms", "result": "pass", "detail": "320ms" }
        ]
      },
      "created_at": "2026-05-02T11:00:00Z"
    },
    "created_at": "2026-05-02T10:20:00Z",
    "completed_at": "2026-05-02T11:00:00Z"
  }
}
```

### POST /api/v1/tasks/:id/assign

分配任务给 Agent 执行。

```json
// Request
{
  "agent_id": "agent_perf_01"
}

// Response 200
{
  "code": 0,
  "data": {
    "task_id": "t_xxx",
    "assigned_agent": "agent_perf_01",
    "status": "in_progress"
  }
}
```

### PATCH /api/v1/tasks/:id

更新任务状态或内容。

```json
// Request
{
  "status": "in_progress",
  "assigned_agent": "agent_perf_01"
}

// Response 200
{
  "code": 0,
  "data": {
    "id": "t_xxx",
    "status": "in_progress",
    "updated_at": "2026-05-02T10:30:00Z"
  }
}
```

### POST /api/v1/tasks/:id/validate

触发验证官 Agent 对任务进行验证。验证结果通过 SSE 推送或轮询获取。

```json
// Response 200 (异步验证，立即返回)
{
  "code": 0,
  "data": {
    "task_id": "t_xxx",
    "status": "reviewing"
  }
}
```

### GET /api/v1/tasks/:id/validation

获取任务的最新验证结果。

```json
// Response 200
{
  "code": 0,
  "data": {
    "id": "vr_xxx",
    "task_id": "t_xxx",
    "validator": "validator_agent",
    "verdict": "passed",
    "reason": "...",
    "detail": { "checked_items": [...] },
    "created_at": "2026-05-02T11:00:00Z"
  }
}

// Response 200 (尚未验证)
{
  "code": 0,
  "data": null
}
```

---

## 9. 模型配置 API

### CRUD /api/v1/models

管理用户自己的模型 API 配置。

```json
// POST /api/v1/models
{
  "provider": "openai",
  "model_name": "gpt-4o",
  "api_key": "sk-xxx",
  "base_url": "https://api.openai.com/v1",
  "config": {
    "temperature": 0.5,
    "max_tokens": 4096
  }
}

// Response 201
{
  "code": 0,
  "data": {
    "id": "mp_xxx",
    "provider": "openai",
    "model_name": "gpt-4o",
    "is_active": true,
    "created_at": "2026-05-02T12:00:00Z"
  }
}

// GET /api/v1/models
{
  "code": 0,
  "data": [
    {
      "id": "mp_xxx",
      "provider": "openai",
      "model_name": "gpt-4o",
      "is_active": true,
      "created_at": "2026-05-02T12:00:00Z"
    }
  ]
}
```

---

## 10. 讨论范式 API

### GET /api/v1/paradigms

获取系统支持的讨论范式列表。

```json
// Response 200
{
  "code": 0,
  "data": [
    {
      "key": "round_robin",
      "name": "轮询发言",
      "description": "按固定顺序依次发言，每轮每人发言一次",
      "suitable_for": ["创意脑暴", "信息收集"],
      "config_schema": {
        "max_rounds": { "type": "integer", "default": 10, "min": 1, "max": 100 },
        "parallel": { "type": "boolean", "default": true }
      }
    },
    {
      "key": "court",
      "name": "模拟法庭",
      "description": "陈述 → 交叉质询 → 回应 → 总结",
      "suitable_for": ["代码评审", "方案审查"],
      "config_schema": {
        "max_rounds": { "type": "integer", "default": 3 },
        "critic_enabled": { "type": "boolean", "default": true }
      }
    },
    {
      "key": "evaluation",
      "name": "加权评估",
      "description": "打分 → 质疑 → 修正 → 决策矩阵",
      "suitable_for": ["产品决策", "技术选型"],
      "config_schema": {
        "criteria": { "type": "array", "items": { "type": "string" } },
        "weights": { "type": "object" }
      }
    },
    {
      "key": "free_chat",
      "name": "自由群聊",
      "description": "角色自由发言，主持人动态调度",
      "suitable_for": ["开放讨论", "策略推演"],
      "config_schema": {}
    }
  ]
}
```

---

## 11. 错误码完整列表

| code | HTTP | 说明 | 处理建议 |
|------|------|------|---------|
| 0 | 200 | 成功 | — |
| 40001 | 400 | 参数校验失败 | 检查请求体中的必填字段和格式 |
| 40002 | 400 | 资源已存在 | 检查是否重复创建 |
| 40003 | 400 | 请求体不是合法 JSON | 检查 JSON 格式 |
| 40101 | 401 | 未认证 | 检查 Authorization header |
| 40102 | 401 | Token 已过期 | 使用 refresh_token 续期 |
| 40103 | 401 | Refresh Token 已过期 | 重新登录 |
| 40301 | 403 | 无权限 | 检查资源归属 |
| 40302 | 403 | 操作被禁用 | 资源处于不可操作状态 |
| 40401 | 404 | 资源不存在 | 检查资源 ID |
| 40901 | 409 | 状态冲突 | Session 当前状态不允许该操作 |
| 40902 | 409 | 资源被引用 | 存在关联资源阻止操作 |
| 42901 | 429 | 请求太频繁 | 降低请求频率，参考 Retry-After 头 |
| 50001 | 500 | 内部错误 | 联系管理员 |
| 50201 | 502 | 上游 Provider 错误 | 检查模型 API Key 和配额 |
| 50301 | 503 | 服务暂不可用 | 稍后重试 |
| 50401 | 504 | Provider 超时 | 稍后重试或降低模型复杂度 |

---

## 12. 限流策略

| 接口 | 限制 | 惩罚 |
|------|------|------|
| `/auth/*` | 10 req/min per IP | 429, Retry-After: 60 |
| `/workspaces` | 100 req/min per user | 429, Retry-After: 30 |
| `/sessions` | 60 req/min per user | 429, Retry-After: 30 |
| `/sessions/:id/stream` | 每个 Session 最多 10 个并发 SSE 连接 | 429 |
| `/sessions/:id/start` | 10 req/min per user (防止频繁启停) | 429 |
| `/tasks` | 200 req/min per user | 429 |
| `/roles` | 100 req/min per user | 429 |

响应头包含限流信息：

```http
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 42
X-RateLimit-Reset: 1683032400
```

---

## 13. API 变更版本策略

| 版本 | URL 前缀 | 状态 | 说明 |
|------|---------|------|------|
| v1 | `/api/v1/` | 当前稳定版 | 不向后兼容变更 |
| v2 | `/api/v2/` | 规划中 | — |

变更策略：
- **向前兼容变更**（加字段、新增端点）：小版本更新，不通知
- **不兼容变更**（改字段、删端点）：新版本 API，旧版本至少维护 6 个月
- 弃用端点会在响应头中加入 `Deprecation: true` 和 `Sunset: date`

---

## 14. 附录

### 附录 A：API 路径汇总

```
认证
  POST   /api/v1/auth/login
  POST   /api/v1/auth/register
  POST   /api/v1/auth/refresh

工作区
  GET    /api/v1/workspaces
  POST   /api/v1/workspaces
  GET    /api/v1/workspaces/:id
  PATCH  /api/v1/workspaces/:id
  POST   /api/v1/workspaces/:id/archive
  GET    /api/v1/workspaces/:id/tasks

会话
  POST   /api/v1/sessions
  GET    /api/v1/sessions/:id
  POST   /api/v1/sessions/:id/start
  POST   /api/v1/sessions/:id/pause
  POST   /api/v1/sessions/:id/resume
  POST   /api/v1/sessions/:id/terminate
  POST   /api/v1/sessions/:id/interrupt
  GET    /api/v1/sessions/:id/stream          (SSE)
  GET    /api/v1/sessions/:id/messages
  GET    /api/v1/sessions/:id/minutes

角色
  GET    /api/v1/roles
  POST   /api/v1/roles
  GET    /api/v1/roles/:id
  PUT    /api/v1/roles/:id
  DELETE /api/v1/roles/:id

任务
  GET    /api/v1/tasks
  POST   /api/v1/tasks
  GET    /api/v1/tasks/:id
  PATCH  /api/v1/tasks/:id
  POST   /api/v1/tasks/:id/assign
  POST   /api/v1/tasks/:id/validate
  GET    /api/v1/tasks/:id/validation

模型配置
  GET    /api/v1/models
  POST   /api/v1/models
  DELETE /api/v1/models/:id

讨论范式
  GET    /api/v1/paradigms
```

### 附录 B：gRPC 服务定义

```protobuf
syntax = "proto3";
package mmcs.v1;
option go_package = "github.com/mmcs/api/v1";

service SessionService {
    rpc CreateSession(CreateSessionRequest) returns (CreateSessionResponse);
    rpc GetSession(GetSessionRequest) returns (GetSessionResponse);
    rpc StartSession(StartSessionRequest) returns (StartSessionResponse);
    rpc PauseSession(PauseSessionRequest) returns (PauseSessionResponse);
    rpc ResumeSession(ResumeSessionRequest) returns (ResumeSessionResponse);
    rpc TerminateSession(TerminateSessionRequest) returns (TerminateSessionResponse);
    rpc StreamSession(StreamSessionRequest) returns (stream SSHEvent);
}

service OrchestrationService {
    rpc ExecuteGraph(ExecuteGraphRequest) returns (ExecuteGraphResponse);
    rpc StreamGraph(StreamGraphRequest) returns (stream GraphEvent);
    rpc InterruptGraph(InterruptGraphRequest) returns (InterruptGraphResponse);
}

service AgentService {
    rpc RunAgent(RunAgentRequest) returns (RunAgentResponse);
    rpc StreamAgent(StreamAgentRequest) returns (stream AgentEvent);
    rpc AssignTask(AssignTaskRequest) returns (AssignTaskResponse);
}

service ValidationService {
    rpc ValidateTask(ValidateTaskRequest) returns (ValidateTaskResponse);
}
```

### 附录 C：curl 示例

```bash
# 1. 登录
curl -X POST https://api.mmcs.example.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret"}'

# 2. 创建工作区
curl -X POST https://api.mmcs.example.com/api/v1/workspaces \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"代码评审","mode":"shared","members":["u_001"]}'

# 3. 创建会话
curl -X POST https://api.mmcs.example.com/api/v1/sessions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "workspace_id": "ws_xxx",
    "title": "登录模块评审",
    "paradigm": "court",
    "roles": [{"role_id": "r_global_security"}]
  }'

# 4. 启动会话
curl -X POST https://api.mmcs.example.com/api/v1/sessions/s_xxx/start \
  -H "Authorization: Bearer <token>"

# 5. 订阅 SSE 流
curl -N https://api.mmcs.example.com/api/v1/sessions/s_xxx/stream \
  -H "Authorization: Bearer <token>" \
  -H "Accept: text/event-stream"

# 6. 获取会议纪要
curl https://api.mmcs.example.com/api/v1/sessions/s_xxx/minutes \
  -H "Authorization: Bearer <token>"
```

### 附录 D：相关文档

| 文件 | 说明 |
|------|------|
| `docs/02 需求规格说明书.md` | 功能需求与技术映射 |
| `docs/04 详细设计说明书.md` | API 服务端详细实现 |
| `docs/05 数据库设计说明书.md` | 数据库设计 |
