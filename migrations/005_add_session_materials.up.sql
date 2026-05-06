-- ============================
-- 文件名: 005_add_session_materials.up.sql
-- 说明: 添加会议附件持久化表
-- ============================

CREATE TABLE IF NOT EXISTS session_materials (
    id            VARCHAR(64) PRIMARY KEY,
    session_id    VARCHAR(64) NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    file_name     VARCHAR(512) NOT NULL DEFAULT '',
    file_size     BIGINT NOT NULL DEFAULT 0,
    mime_type     VARCHAR(256) NOT NULL DEFAULT '',
    content       TEXT NOT NULL DEFAULT '',
    data          BYTEA,
    uploaded_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_session_materials_session_id ON session_materials(session_id);

COMMENT ON TABLE session_materials IS '会议附件材料持久化存储';
COMMENT ON COLUMN session_materials.content IS '文本内容（TXT/MD/代码等可直接展示的）';
COMMENT ON COLUMN session_materials.data IS '原始文件数据（二进制，大文件建议单独存储）';
