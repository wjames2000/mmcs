-- ============================
-- 文件名: 002_add_session_topic.down.sql
-- 说明: 回滚 topic 字段
-- ============================

ALTER TABLE sessions DROP COLUMN IF EXISTS topic;
