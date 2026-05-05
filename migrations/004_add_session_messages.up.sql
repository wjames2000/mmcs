-- ============================
-- 文件名: 004_add_session_messages.up.sql
-- 说明: 添加会话消息持久化表
-- ============================

CREATE TABLE IF NOT EXISTS session_messages (
    id            VARCHAR(64) PRIMARY KEY,
    session_id    VARCHAR(64) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    round         INTEGER NOT NULL DEFAULT 0,
    role_name     VARCHAR(256) NOT NULL DEFAULT '',
    content       TEXT NOT NULL DEFAULT '',
    tokens        INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_session_messages_session_id ON session_messages(session_id);
CREATE INDEX idx_session_messages_created_at ON session_messages(created_at);

COMMENT ON TABLE session_messages IS '会话讨论消息持久化存储';
COMMENT ON COLUMN session_messages.round IS '消息所属讨论轮次';
COMMENT ON COLUMN session_messages.role_name IS '发言人角色名称';
