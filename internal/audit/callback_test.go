package audit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuditCallback(t *testing.T) {
	cb := NewAuditCallback(100)
	require.NotNil(t, cb)
	assert.Equal(t, 0, cb.Count())

	defaultCb := NewAuditCallback(0)
	assert.Equal(t, 10000, defaultCb.maxSize)
}

func TestAuditCallback_Record(t *testing.T) {
	cb := NewAuditCallback(10)
	ctx := context.Background()

	cb.Record(ctx, "session1", "node_start", "expert_speak", "input data", "output data")
	assert.Equal(t, 1, cb.Count())

	cb.Record(ctx, "session1", "node_end", "expert_speak", nil, nil)
	assert.Equal(t, 2, cb.Count())
}

func TestAuditCallback_RecordWithUser(t *testing.T) {
	cb := NewAuditCallback(10)
	ctx := context.Background()

	cb.RecordWithUser(ctx, "session1", "agent_call", "agent1", "input", "output", "user123")
	assert.Equal(t, 1, cb.Count())

	entries := cb.GetRecent(10)
	require.Len(t, entries, 1)
	assert.Equal(t, "user123", entries[0].UserID)
}

func TestAuditCallback_GetRecent(t *testing.T) {
	cb := NewAuditCallback(10)
	ctx := context.Background()

	// 记录 5 条
	for i := 0; i < 5; i++ {
		cb.Record(ctx, "session1", "node_start", "node1", i, i*2)
	}

	// 获取全部
	entries := cb.GetRecent(10)
	assert.Len(t, entries, 5)

	// 获取 3 条
	entries = cb.GetRecent(3)
	assert.Len(t, entries, 3)
	assert.Equal(t, 2, entries[0].Input) // 第 3 条（0-indexed: 2）

	// n <= 0
	entries = cb.GetRecent(0)
	assert.Nil(t, entries)
}

func TestAuditCallback_RingBuffer(t *testing.T) {
	cb := NewAuditCallback(5) // 环形缓冲区大小为 5
	ctx := context.Background()

	// 写入 7 条，超过容量
	for i := 0; i < 7; i++ {
		cb.Record(ctx, "session1", "event", "node1", i, i*2)
	}

	assert.Equal(t, 5, cb.Count()) // 最多保留 5 条

	entries := cb.GetRecent(10)
	assert.Len(t, entries, 5)

	// 验证是最新的 5 条（index 2-6）
	for i, entry := range entries {
		expectedInput := i + 2
		assert.Equal(t, expectedInput, entry.Input)
	}
}

func TestAuditCallback_Flush(t *testing.T) {
	cb := NewAuditCallback(10)
	ctx := context.Background()

	// 记录 5 条
	for i := 0; i < 5; i++ {
		cb.Record(ctx, "session1", "event", "node1", "input", "output")
	}

	// Flush 成功
	flushed := false
	err := cb.Flush(ctx, func(ctx context.Context, entries []AuditEntry) error {
		assert.Len(t, entries, 5)
		flushed = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, flushed)

	// Flush 后缓冲区应清空
	assert.Equal(t, 0, cb.Count())

	// Flush 空缓冲区不应报错
	err = cb.Flush(ctx, func(ctx context.Context, entries []AuditEntry) error {
		return nil
	})
	require.NoError(t, err)
}

func TestAuditCallback_FlushError(t *testing.T) {
	cb := NewAuditCallback(10)
	ctx := context.Background()

	cb.Record(ctx, "session1", "event", "node1", "input", "output")

	expectedErr := errors.New("db error")
	err := cb.Flush(ctx, func(ctx context.Context, entries []AuditEntry) error {
		return expectedErr
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestAuditCallback_ConcurrentRecord(t *testing.T) {
	cb := NewAuditCallback(100)
	ctx := context.Background()
	var wg sync.WaitGroup

	// 并发记录 100 条
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cb.Record(ctx, "session1", "event", "node1", id, id*2)
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 100, cb.Count())

	// 验证没有数据竞争
	entries := cb.GetRecent(50)
	assert.Len(t, entries, 50)
}

func TestAuditCallback_FlushToDB(t *testing.T) {
	cb := NewAuditCallback(10)
	ctx := context.Background()

	// 记录
	cb.RecordWithUser(ctx, "session1", "node_start", "expert_speak", "input1", "output1", "user1")
	cb.RecordWithUser(ctx, "session1", "node_end", "expert_speak", nil, nil, "user1")

	// 使用 mock DB
	mockDB := &mockDB{entries: make([]auditRow, 0)}
	flushFn := FlushToDB(mockDB)

	err := cb.Flush(ctx, flushFn)
	require.NoError(t, err)
	assert.Equal(t, 2, len(mockDB.entries))
}

// mockDB 模拟数据库连接
type auditRow struct {
	time      time.Time
	sessionID string
	event     string
	nodeName  string
	input     string
	output    string
	userID    string
}

type mockDB struct {
	mu      sync.Mutex
	entries []auditRow
}

func (m *mockDB) Exec(ctx context.Context, sql string, args ...any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(args) < 7 {
		return errors.New("insufficient args")
	}

	t, _ := args[0].(time.Time)
	sessionID, _ := args[1].(string)
	event, _ := args[2].(string)
	nodeName, _ := args[3].(string)
	input, _ := args[4].(string)
	output, _ := args[5].(string)
	userID, _ := args[6].(string)

	m.entries = append(m.entries, auditRow{
		time: t, sessionID: sessionID, event: event,
		nodeName: nodeName, input: input, output: output, userID: userID,
	})
	return nil
}
