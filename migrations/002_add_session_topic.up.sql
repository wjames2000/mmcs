-- ============================
-- 文件名: 002_add_session_topic.up.sql
-- 说明: 为 sessions 表增加讨论主题描述字段 topic
-- ============================

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS topic TEXT DEFAULT '';
COMMENT ON COLUMN sessions.topic IS '讨论主题/背景描述，用于指导编排执行';
