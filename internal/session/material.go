// Package session 提供会话管理
package session

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/wjames2000/mmcs/pkg/util"
)

// Material 会议材料（单个上传文件）
type Material struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	FileName   string    `json:"file_name"`
	FileSize   int64     `json:"file_size"`
	MimeType   string    `json:"mime_type"`
	Content    string    `json:"content,omitempty"` // 文本内容（TXT/MD/代码等可直接展示的）
	Data       []byte    `json:"-"`                 // 原始文件数据（不序列化到 JSON）
	UploadedAt time.Time `json:"uploaded_at"`
}

// MaterialStore 内存材料存储，支持并发安全
// 线程安全：所有导出方法均使用 RWMutex 保护
type MaterialStore struct {
	mu    sync.RWMutex
	items map[string][]*Material // sessionID → materials
	byID  map[string]*Material   // materialID → material（快速查找）
}

// NewMaterialStore 创建材料存储
func NewMaterialStore() *MaterialStore {
	return &MaterialStore{
		items: make(map[string][]*Material),
		byID:  make(map[string]*Material),
	}
}

// Add 添加材料到会话
// 返回创建的 Material 指针。并发安全。
func (s *MaterialStore) Add(sessionID, fileName, mimeType string, data []byte) *Material {
	s.mu.Lock()
	defer s.mu.Unlock()

	m := &Material{
		ID:         util.NewID("mat"),
		SessionID:  sessionID,
		FileName:   fileName,
		FileSize:   int64(len(data)),
		MimeType:   mimeType,
		Content:    getContent(mimeType, data),
		Data:       data,
		UploadedAt: time.Now(),
	}

	s.items[sessionID] = append(s.items[sessionID], m)
	s.byID[m.ID] = m
	return m
}

// ListBySession 获取会话的所有材料
// 返回的材料切片为拷贝，外部修改不影响内部状态。并发安全。
func (s *MaterialStore) ListBySession(sessionID string) []*Material {
	s.mu.RLock()
	defer s.mu.RUnlock()

	materials := s.items[sessionID]
	result := make([]*Material, len(materials))
	for i, m := range materials {
		// 浅拷贝，但 Content 和 Data 是值类型/不可变，安全
		cp := *m
		result[i] = &cp
	}
	return result
}

// Get 获取单个材料
// 返回材料拷贝。并发安全。
func (s *MaterialStore) Get(id string) (*Material, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	m, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("材料不存在: %s", id)
	}
	cp := *m
	return &cp, nil
}

// Delete 删除材料
// 并发安全。如果材料不存在，返回 error。
func (s *MaterialStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.byID[id]
	if !ok {
		return fmt.Errorf("材料不存在: %s", id)
	}

	// 从 session 列表中移除
	sessionID := m.SessionID
	materials := s.items[sessionID]
	for i, mat := range materials {
		if mat.ID == id {
			s.items[sessionID] = append(materials[:i], materials[i+1:]...)
			break
		}
	}

	// 如果该会话没有材料了，清理 map 条目
	if len(s.items[sessionID]) == 0 {
		delete(s.items, sessionID)
	}

	// 从 ID 索引中移除
	delete(s.byID, id)

	return nil
}

// CopyToSession 将源会话的所有材料复制到目标会话
// 材料被浅拷贝复制（ID 重新生成，SessionID 更新为目标会话）
// 并发安全。
func (s *MaterialStore) CopyToSession(sourceSessionID, targetSessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	materials := s.items[sourceSessionID]
	for _, m := range materials {
		cp := &Material{
			ID:         util.NewID("mat"),
			SessionID:  targetSessionID,
			FileName:   m.FileName,
			FileSize:   m.FileSize,
			MimeType:   m.MimeType,
			Content:    m.Content,
			Data:       m.Data,
			UploadedAt: time.Now(),
		}
		s.items[targetSessionID] = append(s.items[targetSessionID], cp)
		s.byID[cp.ID] = cp
	}
}

// getContent 根据 MIME 类型提取文本内容
// 文本类（text/*, application/json 等）返回原文本；
// 二进制类返回 base64 编码字符串。
func getContent(mimeType string, data []byte) string {
	if isTextMIME(mimeType) {
		return string(data)
	}
	return base64.StdEncoding.EncodeToString(data)
}

// isTextMIME 判断 MIME 类型是否为文本类
func isTextMIME(mimeType string) bool {
	// 严格匹配已知文本类型
	textTypes := map[string]bool{
		"text/plain":             true,
		"text/markdown":          true,
		"text/html":              true,
		"text/css":               true,
		"text/csv":               true,
		"text/xml":               true,
		"application/json":       true,
		"application/xml":        true,
		"application/yaml":       true,
		"application/javascript": true,
		"application/typescript": true,
		"application/x-sh":       true,
		"application/x-yaml":     true,
	}

	if textTypes[mimeType] {
		return true
	}

	// 宽松匹配：text/* 开头
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	// application/*+json 或 application/*+xml
	if strings.HasSuffix(mimeType, "+json") || strings.HasSuffix(mimeType, "+xml") {
		return true
	}

	return false
}
