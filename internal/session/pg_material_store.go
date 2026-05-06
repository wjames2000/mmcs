package session

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PGMaterialStore 数据库持久化的材料存储
type PGMaterialStore struct {
	cache *MaterialStore
	pool  *pgxpool.Pool
	mu    sync.RWMutex
}

// NewPGMaterialStore 创建数据库持久化材料存储
func NewPGMaterialStore(pool *pgxpool.Pool) *PGMaterialStore {
	return &PGMaterialStore{
		cache: NewMaterialStore(),
		pool:  pool,
	}
}

// Add 添加材料（内存 + 数据库持久化）
func (s *PGMaterialStore) Add(sessionID, fileName, mimeType string, data []byte) *Material {
	m := s.cache.Add(sessionID, fileName, mimeType, data)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := s.pool.Exec(ctx,
			`INSERT INTO session_materials (id, session_id, file_name, file_size, mime_type, content, data, uploaded_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT (id) DO NOTHING`,
			m.ID, m.SessionID, m.FileName, m.FileSize, m.MimeType, m.Content, m.Data, m.UploadedAt,
		)
		if err != nil {
			return
		}
	}()

	return m
}

// ListBySession 获取会话的所有材料（优先缓存，回退到数据库）
func (s *PGMaterialStore) ListBySession(sessionID string) []*Material {
	msgs := s.cache.ListBySession(sessionID)
	if len(msgs) > 0 {
		return msgs
	}
	return s.loadFromDB(sessionID)
}

// Delete 删除材料（内存 + 数据库）
func (s *PGMaterialStore) Delete(id string) error {
	if err := s.cache.Delete(id); err != nil {
		return err
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = s.pool.Exec(ctx, `DELETE FROM session_materials WHERE id = $1`, id)
	}()

	return nil
}

// CopyToSession 复制材料到新会话（内存 + 数据库）
func (s *PGMaterialStore) CopyToSession(fromSessionID, toSessionID string) {
	s.cache.CopyToSession(fromSessionID, toSessionID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := s.pool.Query(ctx,
		`SELECT id, session_id, file_name, file_size, mime_type, content, data, uploaded_at
		 FROM session_materials WHERE session_id = $1`, fromSessionID,
	)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var mat Material
		if err := rows.Scan(&mat.ID, &mat.SessionID, &mat.FileName, &mat.FileSize, &mat.MimeType, &mat.Content, &mat.Data, &mat.UploadedAt); err != nil {
			continue
		}
		mat.ID = ""
		mat.SessionID = toSessionID
		s.cache.Add(toSessionID, mat.FileName, mat.MimeType, mat.Data)
	}
}

// loadFromDB 从数据库加载材料并回填缓存
func (s *PGMaterialStore) loadFromDB(sessionID string) []*Material {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := s.pool.Query(ctx,
		`SELECT id, session_id, file_name, file_size, mime_type, content, data, uploaded_at
		 FROM session_materials WHERE session_id = $1 ORDER BY uploaded_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var result []*Material
	for rows.Next() {
		m := &Material{}
		if err := rows.Scan(&m.ID, &m.SessionID, &m.FileName, &m.FileSize, &m.MimeType, &m.Content, &m.Data, &m.UploadedAt); err != nil {
			continue
		}
		result = append(result, m)
	}
	if result == nil {
		result = []*Material{}
	}

	for _, m := range result {
		s.cache.Add(m.SessionID, m.FileName, m.MimeType, m.Data)
	}

	return result
}
