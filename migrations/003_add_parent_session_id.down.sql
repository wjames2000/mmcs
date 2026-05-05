-- ============================
-- 文件名: 003_add_parent_session_id.down.sql
-- 说明: 回滚 — 删除 parent_session_id 字段
-- ============================

ALTER TABLE sessions DROP COLUMN IF EXISTS parent_session_id;
