-- ============================
-- 文件名: 001_init.up.sql
-- 说明: 初始化数据库结构
-- ============================

-- 创建必要扩展
CREATE EXTENSION IF NOT EXISTS "pgcrypto";    -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS "vector";       -- pgvector

-- ===== 1. 用户表 =====
CREATE TABLE users (
    id            VARCHAR(64) PRIMARY KEY,       -- 格式: u_xxx
    name          VARCHAR(128) NOT NULL,         -- 用户名
    email         VARCHAR(256) UNIQUE NOT NULL,  -- 邮箱 (登录凭证)
    password_hash VARCHAR(256) NOT NULL,         -- bcrypt 哈希
    avatar_url    VARCHAR(512),                  -- 头像 URL
    status        VARCHAR(32) NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active', 'disabled')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE users IS '系统用户表';
COMMENT ON COLUMN users.password_hash IS 'bcrypt(password + salt), 60 chars';

CREATE UNIQUE INDEX idx_users_email ON users(email)
    WHERE status = 'active';

-- ===== 2. 工作区表 =====
CREATE TABLE workspaces (
    id            VARCHAR(64) PRIMARY KEY,       -- 格式: ws_xxx
    name          VARCHAR(256) NOT NULL,         -- 工作区名称
    description   TEXT,                          -- 描述
    mode          VARCHAR(32) NOT NULL DEFAULT 'standalone'
                  CHECK (mode IN ('standalone', 'shared')),
    status        VARCHAR(32) NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active', 'archived')),
    members       TEXT[] NOT NULL DEFAULT '{}',  -- 参与者 user_id 数组
    creator_id    VARCHAR(64) NOT NULL REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE workspaces IS '工作区：多轮对话/多议题分区管理容器';
COMMENT ON COLUMN workspaces.mode IS 'standalone=独立模式(1:1), shared=共享模式(1:N)';
COMMENT ON COLUMN workspaces.members IS '参与者 ID 列表，用于权限校验和 UI 导航';

CREATE INDEX idx_workspaces_creator ON workspaces(creator_id);
CREATE INDEX idx_workspaces_members ON workspaces USING GIN(members);
CREATE INDEX idx_workspaces_status ON workspaces(status)
    WHERE status = 'active';

-- ===== 3. 角色表 =====
CREATE TABLE roles (
    id              VARCHAR(64) PRIMARY KEY,     -- 格式: r_xxx
    name            VARCHAR(128) NOT NULL,       -- 角色名 (如: 安全审查员)
    title           VARCHAR(128) NOT NULL,       -- 职位头衔
    traits          JSONB NOT NULL DEFAULT '{}'
                    CHECK (jsonb_typeof(traits) = 'object'),
    expertise       TEXT[] NOT NULL DEFAULT '{}',
    speaking_style  TEXT,                         -- 说话风格描述
    system_prompt   TEXT,                         -- 系统提示词模板
    skills          TEXT[] NOT NULL DEFAULT '{}',  -- 引用的 Skill 名称列表
    default_model   JSONB,                       -- {provider, model_name, parameters...}
    is_global       BOOLEAN NOT NULL DEFAULT false,
    creator_id      VARCHAR(64) REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE roles IS 'AI 专家角色定义';
COMMENT ON COLUMN roles.traits IS '{"aggressiveness": 1-10, "optimism": 1-10, "creativity": 1-10, "detail": 1-10}';
COMMENT ON COLUMN roles.skills IS '引用的 Skill 名称数组，内容自动注入 system_prompt';
COMMENT ON COLUMN roles.is_global IS 'true=系统预置全局模板, false=用户自建';

CREATE INDEX idx_roles_creator ON roles(creator_id);
CREATE INDEX idx_roles_global ON roles(is_global) WHERE is_global = true;
CREATE INDEX idx_roles_expertise ON roles USING GIN(expertise);
CREATE INDEX idx_roles_skills ON roles USING GIN(skills);

-- ===== 4. 会话表 =====
CREATE TABLE sessions (
    id              VARCHAR(64) PRIMARY KEY,     -- 格式: s_xxx
    workspace_id    VARCHAR(64) NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title           VARCHAR(256) NOT NULL,
    paradigm        VARCHAR(64) NOT NULL
                    CHECK (paradigm IN ('round_robin', 'court', 'evaluation', 'free_chat')),
    status          VARCHAR(32) NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'running', 'paused', 'ended', 'failed')),
    max_rounds      INT NOT NULL DEFAULT 10
                    CHECK (max_rounds >= 1 AND max_rounds <= 100),
    round_timeout   INT NOT NULL DEFAULT 300
                    CHECK (round_timeout >= 30),
    config          JSONB NOT NULL DEFAULT '{}',
    creator_id      VARCHAR(64) NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at      TIMESTAMPTZ,
    ended_at        TIMESTAMPTZ
);
COMMENT ON TABLE sessions IS '讨论会议会话';
COMMENT ON COLUMN sessions.paradigm IS '讨论范式: round_robin/court/evaluation/free_chat';
COMMENT ON COLUMN sessions.config IS '扩展配置 {context_enabled, auto_terminate, ...}';
COMMENT ON COLUMN sessions.started_at IS '首次 Start() 时间';
COMMENT ON COLUMN sessions.ended_at IS 'Terminate() 或自然结束时间';

CREATE INDEX idx_sessions_workspace ON sessions(workspace_id);
CREATE INDEX idx_sessions_creator ON sessions(creator_id);
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_workspace_status ON sessions(workspace_id, status);
CREATE INDEX idx_sessions_created_at ON sessions(created_at DESC);

-- ===== 5. 会话角色绑定表 =====
CREATE TABLE session_roles (
    id              VARCHAR(64) PRIMARY KEY,     -- 格式: sr_xxx
    session_id      VARCHAR(64) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    role_id         VARCHAR(64) NOT NULL REFERENCES roles(id),
    model_override  JSONB,                       -- 覆盖角色的默认模型 {provider, model_name, parameters}
    sort_order      INT NOT NULL DEFAULT 0,      -- 发言顺序
    UNIQUE(session_id, role_id)
);
COMMENT ON TABLE session_roles IS '会话与会话中使用的角色的关联表';
COMMENT ON COLUMN session_roles.model_override IS '可为该会话临时更换角色绑定的模型';

CREATE INDEX idx_session_roles_session ON session_roles(session_id);
CREATE INDEX idx_session_roles_role ON session_roles(role_id);

-- ===== 6. 消息表 =====
CREATE TABLE messages (
    id              VARCHAR(64) PRIMARY KEY,     -- 格式: msg_xxx
    session_id      VARCHAR(64) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    round_id        VARCHAR(64),                  -- 所属轮次 (可选)
    role_id         VARCHAR(64),                  -- 发言角色 ID
    author_type     VARCHAR(32) NOT NULL
                    CHECK (author_type IN ('ai', 'user', 'system')),
    role_name       VARCHAR(128),                -- 角色显示名 (冗余，便于查询)
    content         TEXT NOT NULL,
    tokens          INT NOT NULL DEFAULT 0
                    CHECK (tokens >= 0),
    metadata        JSONB NOT NULL DEFAULT '{}', -- {model, latency_ms, seq, phase}
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE messages IS '讨论消息记录（含流式 token 最终合并结果）';
COMMENT ON COLUMN messages.round_id IS '格式: round_{session_id}_{number}';
COMMENT ON COLUMN messages.role_name IS '冗余字段，避免频繁 JOIN roles 表';
COMMENT ON COLUMN messages.tokens IS '该消息消耗的 token 数，用于计费和上下文管理';

CREATE INDEX idx_messages_session ON messages(session_id);
CREATE INDEX idx_messages_session_round ON messages(session_id, round_id);
CREATE INDEX idx_messages_session_created ON messages(session_id, created_at);
CREATE INDEX idx_messages_author ON messages(session_id, author_type);

-- ===== 7. 任务表 =====
CREATE TABLE tasks (
    id                  VARCHAR(64) PRIMARY KEY, -- 格式: t_xxx
    session_id          VARCHAR(64) NOT NULL REFERENCES sessions(id),
    workspace_id        VARCHAR(64) NOT NULL REFERENCES workspaces(id),
    title               VARCHAR(256) NOT NULL,
    description         TEXT,
    acceptance_criteria TEXT,                    -- 验收标准
    status              VARCHAR(32) NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'in_progress', 'reviewing', 'completed', 'rejected')),
    priority            VARCHAR(16) NOT NULL DEFAULT 'medium'
                        CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    assigned_agent      VARCHAR(64),             -- Agent ID
    assigned_by         VARCHAR(64) REFERENCES users(id),  -- "" 表示自动分配
    source_round        INT,                     -- 来源讨论轮次
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at        TIMESTAMPTZ
);
COMMENT ON TABLE tasks IS '从讨论结论中提取的可执行任务';
COMMENT ON COLUMN tasks.acceptance_criteria IS '验证官判定依据';
COMMENT ON COLUMN tasks.assigned_agent IS '执行 Agent 的 ID，null 表示待分配';

CREATE INDEX idx_tasks_session ON tasks(session_id);
CREATE INDEX idx_tasks_workspace ON tasks(workspace_id);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_assignee ON tasks(assigned_agent);
CREATE INDEX idx_tasks_workspace_status ON tasks(workspace_id, status);
CREATE INDEX idx_tasks_priority ON tasks(priority) WHERE status = 'pending';

-- ===== 8. 验证结果表 =====
CREATE TABLE validation_results (
    id              VARCHAR(64) PRIMARY KEY,     -- 格式: vr_xxx
    task_id         VARCHAR(64) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    validator       VARCHAR(64) NOT NULL,        -- 验证官 Agent ID/名称
    verdict         VARCHAR(32) NOT NULL
                    CHECK (verdict IN ('passed', 'needs_revision', 'rejected')),
    reason          TEXT NOT NULL,                -- 判定理由
    detail          JSONB NOT NULL DEFAULT '{}', -- {checked_items: [{item, result, detail}], scores}
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(task_id)
);
COMMENT ON TABLE validation_results IS '验证官判定结果（每任务一条）';
COMMENT ON COLUMN validation_results.detail IS '逐项检查明细';

-- ===== 9. 模型配置表 =====
CREATE TABLE model_providers (
    id                  VARCHAR(64) PRIMARY KEY, -- 格式: mp_xxx
    user_id             VARCHAR(64) NOT NULL REFERENCES users(id),
    provider            VARCHAR(32) NOT NULL,    -- openai / anthropic / ollama / ark
    model_name          VARCHAR(128) NOT NULL,
    api_key_encrypted   TEXT,                    -- AES-256-GCM 加密
    base_url            VARCHAR(512),            -- 自定义 API 地址
    config              JSONB NOT NULL DEFAULT '{}'
                        CHECK (jsonb_typeof(config) = 'object'),
    is_active           BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, provider, model_name)
);
COMMENT ON TABLE model_providers IS '用户自行配置的模型 API 凭证';
COMMENT ON COLUMN model_providers.api_key_encrypted IS 'AES-256-GCM 加密后的 API Key';

CREATE INDEX idx_model_providers_user ON model_providers(user_id);
CREATE INDEX idx_model_providers_active ON model_providers(is_active)
    WHERE is_active = true;

-- ===== 10. 审计日志表 =====
CREATE TABLE audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    session_id      VARCHAR(64),
    user_id         VARCHAR(64),
    event_type      VARCHAR(64) NOT NULL,        -- node_start / node_end / agent_call / error
    event_name      VARCHAR(256) NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',  -- {input, output, latency, node_name, ...}
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
COMMENT ON TABLE audit_logs IS '全链路审计日志，通过 Eino Callbacks 写入';
COMMENT ON COLUMN audit_logs.payload IS '事件详情，含输入输出快照';

CREATE INDEX idx_audit_session ON audit_logs(session_id);
CREATE INDEX idx_audit_user ON audit_logs(user_id);
CREATE INDEX idx_audit_type ON audit_logs(event_type);
CREATE INDEX idx_audit_created ON audit_logs(created_at DESC);
