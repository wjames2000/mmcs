package session

import (
	"time"

	"github.com/wjames2000/mmcs/internal/minutes"
)

// MergedMinutes 合并原会话和新会话的会议纪要
// 用于重启会议场景：新会话的纪要包含原会话的讨论历史 + 新讨论
type MergedMinutes struct {
	OriginalSessionID string                  `json:"original_session_id"`
	NewSessionID      string                  `json:"new_session_id"`
	OriginalTitle     string                  `json:"original_title"`
	NewTitle          string                  `json:"new_title"`
	OriginalMinutes   *minutes.MeetingMinutes `json:"original_minutes,omitempty"`
	NewMinutes        *minutes.MeetingMinutes `json:"new_minutes,omitempty"`

	// 合并字段
	MergedDecisions  []minutes.Decision     `json:"merged_decisions"`
	MergedConclusion string                 `json:"merged_conclusion"`
	MergedMaterials  []minutes.MaterialInfo `json:"merged_materials,omitempty"`
}

// MaterialInfo 会议材料摘要（用于合并纪要展示）
type MaterialInfo struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	FileName   string    `json:"file_name"`
	FileSize   int64     `json:"file_size"`
	MimeType   string    `json:"mime_type"`
	UploadedAt time.Time `json:"uploaded_at"`
}
