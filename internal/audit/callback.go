// Package audit 提供全链路审计日志功能
// 使用内存环形缓冲区存储审计条目，支持批量写入数据库和 Prometheus 指标
package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/wjames2000/mmcs/pkg/metrics"
)

// AuditEntry 审计日志条目
type AuditEntry struct {
	Time      time.Time `json:"time"`
	SessionID string    `json:"session_id"`
	Event     string    `json:"event"` // "node_start" / "node_end" / "agent_call"
	NodeName  string    `json:"node_name,omitempty"`
	Input     any       `json:"input,omitempty"`
	Output    any       `json:"output,omitempty"`
	UserID    string    `json:"user_id"`
	Duration  string    `json:"duration,omitempty"` // 耗时（node_start→node_end）
}

// AuditCallback 全链路审计日志回调
// 使用内存环形缓冲区存储，通过 Flush 批量写入数据库
// 线程安全
type AuditCallback struct {
	mu      sync.Mutex
	entries []AuditEntry
	maxSize int
	pos     int // 环形缓冲区当前位置
	count   int // 当前有效条目数
}

// NewAuditCallback 创建审计日志回调
// maxSize: 环形缓冲区最大容量
func NewAuditCallback(maxSize int) *AuditCallback {
	if maxSize <= 0 {
		maxSize = 10000 // 默认 1 万条
	}
	return &AuditCallback{
		entries: make([]AuditEntry, maxSize),
		maxSize: maxSize,
		pos:     0,
		count:   0,
	}
}

// Record 记录审计日志条目
// ctx: 上下文（用于提取 traceId 等信息）
// sessionID: 会话 ID
// event: 事件类型（node_start / node_end / agent_call）
// nodeName: 节点名称
// input: 输入数据
// output: 输出数据
func (c *AuditCallback) Record(ctx context.Context, sessionID, event, nodeName string, input, output any) {
	entry := AuditEntry{
		Time:      time.Now(),
		SessionID: sessionID,
		Event:     event,
		NodeName:  nodeName,
		Input:     input,
		Output:    output,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[c.pos] = entry
	c.pos = (c.pos + 1) % c.maxSize
	if c.count < c.maxSize {
		c.count++
	}

	// 更新 Prometheus 指标
	metrics.AuditEntryTotal.Inc()

	log.Debug().
		Str("session_id", sessionID).
		Str("event", event).
		Str("node", nodeName).
		Msg("审计日志记录")
}

// RecordWithUser 记录带用户 ID 的审计日志条目
func (c *AuditCallback) RecordWithUser(ctx context.Context, sessionID, event, nodeName string, input, output any, userID string) {
	entry := AuditEntry{
		Time:      time.Now(),
		SessionID: sessionID,
		Event:     event,
		NodeName:  nodeName,
		Input:     input,
		Output:    output,
		UserID:    userID,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[c.pos] = entry
	c.pos = (c.pos + 1) % c.maxSize
	if c.count < c.maxSize {
		c.count++
	}

	metrics.AuditEntryTotal.Inc()

	log.Debug().
		Str("session_id", sessionID).
		Str("event", event).
		Str("node", nodeName).
		Str("user_id", userID).
		Msg("审计日志记录")
}

// Flush 批量写入审计日志
// 将所有条目序列化并写入数据库（通过 db 接口）
// 写入成功后清空缓冲区
// db: 数据库写入函数（接收 []AuditEntry 并写入）
func (c *AuditCallback) Flush(ctx context.Context, flushFn func(context.Context, []AuditEntry) error) error {
	c.mu.Lock()
	entries := c.snapshot()

	if len(entries) == 0 {
		c.mu.Unlock()
		return nil
	}

	// 在释放锁之前先清空缓冲区
	c.count = 0
	c.pos = 0
	c.mu.Unlock()

	start := time.Now()
	if err := flushFn(ctx, entries); err != nil {
		log.Error().Err(err).Int("count", len(entries)).Msg("审计日志批量写入失败")

		// Flush 失败时需要恢复条目
		// 由于可能有并发写入，我们只恢复这些条目（追加到当前缓冲区尾部）
		c.mu.Lock()
		for _, entry := range entries {
			c.entries[c.pos] = entry
			c.pos = (c.pos + 1) % c.maxSize
			if c.count < c.maxSize {
				c.count++
			}
		}
		c.mu.Unlock()

		return fmt.Errorf("审计日志 Flush 失败: %w", err)
	}

	elapsed := time.Since(start)
	log.Info().
		Int("count", len(entries)).
		Dur("duration", elapsed).
		Msg("审计日志批量写入完成")

	return nil
}

// FlushToDB 将审计日志写入 PostgreSQL
// 此函数作为 Flush 的 flushFn 参数使用
// 注意：实际数据库写入需要适配具体的数据库接口
func FlushToDB(db interface {
	Exec(ctx context.Context, sql string, args ...any) error
}) func(context.Context, []AuditEntry) error {
	return func(ctx context.Context, entries []AuditEntry) error {
		if len(entries) == 0 {
			return nil
		}

		for _, entry := range entries {
			inputStr := fmt.Sprintf("%v", entry.Input)
			outputStr := fmt.Sprintf("%v", entry.Output)

			err := db.Exec(ctx,
				`INSERT INTO audit_logs (time, session_id, event, node_name, input, output, user_id)
				 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				entry.Time, entry.SessionID, entry.Event, entry.NodeName,
				inputStr, outputStr, entry.UserID,
			)
			if err != nil {
				return fmt.Errorf("写入审计日志失败: %w", err)
			}
		}
		return nil
	}
}

// GetRecent 获取最近 n 条审计日志
func (c *AuditCallback) GetRecent(n int) []AuditEntry {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n <= 0 {
		return nil
	}

	all := c.snapshot()
	if len(all) <= n {
		return all
	}

	result := make([]AuditEntry, n)
	copy(result, all[len(all)-n:])
	return result
}

// Count 返回当前审计日志条目数
func (c *AuditCallback) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// snapshot 返回当前所有条目的副本（调用者需持有 mu 锁）
func (c *AuditCallback) snapshot() []AuditEntry {
	if c.count == 0 {
		return nil
	}

	result := make([]AuditEntry, c.count)
	if c.count < c.maxSize {
		// 未满，直接复制
		copy(result, c.entries[:c.count])
		return result
	}

	// 环形缓冲区已满，按时间顺序复制
	// pos 指向下一个写入位置，所以最早条目在 pos
	copy(result, c.entries[c.pos:])
	copy(result[c.maxSize-c.pos:], c.entries[:c.pos])
	return result
}
