-- ============================
-- 文件名: 003_add_parent_session_id.up.sql
-- 说明: 为 sessions 表增加 parent_session_id 字段，
--       支持重启会议（restart）功能
-- ============================

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS parent_session_id VARCHAR(64) REFERENCES sessions(id);
COMMENT ON COLUMN sessions.parent_session_id IS '重启会议时关联的原会话 ID，NULL 表示原始会话';
