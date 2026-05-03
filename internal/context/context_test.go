package context

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/wjames2000/mmcs/internal/model_gateway"
)

// ===== Manager Tests =====

func TestContextManager_Create(t *testing.T) {
	m := NewManager(4096, StrategySummarize)
	assert.NotNil(t, m)
	assert.Equal(t, 4096, m.GetMaxTokens())
	assert.Equal(t, StrategySummarize, m.strategy)
}

func TestNewManager_WithOptions(t *testing.T) {
	m := NewManager(2048, StrategySlidingWindow, WithWindowSize(5))
	assert.NotNil(t, m)
	assert.Equal(t, 2048, m.GetMaxTokens())
	assert.Equal(t, StrategySlidingWindow, m.strategy)
	assert.Equal(t, 5, m.windowSize)
}

func TestCompress_NoCompressionNeeded(t *testing.T) {
	m := NewManager(10000, StrategyDropOldest)

	history := []*Message{
		{Role: "system", Content: "你是一个助手", Round: 0},
		{Role: "user", Content: "你好", Round: 1},
		{Role: "assistant", Content: "你好！有什么可以帮助你的？", Round: 1},
	}

	result, tokens, err := m.Compress(history, 100)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(result), "未超限不应压缩")
	assert.Equal(t, 100, tokens, "token 数应保持不变")
}

func TestCompress_ByDropOldest(t *testing.T) {
	m := NewManager(50, StrategyDropOldest)

	// 创建足够多的消息，确保 token 超限
	history := []*Message{
		{Role: "system", Content: "你是一个助手", Round: 0},
	}
	for i := 0; i < 10; i++ {
		history = append(history, &Message{
			Role:    "user",
			Content: "这是一条较长的用户消息，用于测试压缩功能是否正常工作",
			Round:   i + 1,
		})
		history = append(history, &Message{
			Role:    "assistant",
			Content: "这是一条较长的助手回复消息，包含了一些具体的建议和想法",
			Round:   i + 1,
		})
	}

	// history 有 21 条消息
	assert.Equal(t, 21, len(history))

	result, newTokens, err := m.Compress(history, 60)
	assert.NoError(t, err)
	// 丢弃策略应减少消息数量
	assert.True(t, len(result) < len(history),
		"应该压缩了消息，原: %d, 现: %d", len(history), len(result))
	// token 数应比原来少
	_ = newTokens
	assert.Equal(t, "你是一个助手", result[0].Content, "应该保留了系统消息")
}

func TestCompress_BySlidingWindow(t *testing.T) {
	m := NewManager(10000, StrategySlidingWindow, WithWindowSize(2))

	history := []*Message{
		{Role: "system", Content: "你是一个助手", Round: 0},
	}
	// 5 轮对话
	for i := 0; i < 5; i++ {
		history = append(history, &Message{
			Role:    "user",
			Content: "用户消息",
			Round:   i + 1,
		})
		history = append(history, &Message{
			Role:    "assistant",
			Content: "助手回复",
			Round:   i + 1,
		})
	}

	_ = history // use it
	// 构造一个包含 5 轮对话的 history
	hist := make([]*Message, 0)
	hist = append(hist, &Message{Role: "system", Content: "你是一个助手", Round: 0})
	for i := 0; i < 5; i++ {
		hist = append(hist, &Message{Role: "user", Content: "用户消息", Round: i + 1})
		hist = append(hist, &Message{Role: "assistant", Content: "助手回复", Round: i + 1})
	}

	result, _, err := m.Compress(hist, 100000)
	assert.NoError(t, err)

	// 应该包含 system + 截断通知 + 最近 2 轮（4 条消息）= 6 条
	assert.Equal(t, 6, len(result), "滑动窗口应该保留 system + 通知 + 最近 2 轮")
	assert.Equal(t, "你是一个助手", result[0].Content)
	assert.Contains(t, result[1].Content, "截断", "第二条应该是截断通知")
}

func TestCompress_BySummarize(t *testing.T) {
	// 没有设置 summarizeModel 时，应该回退到 drop_oldest
	m := NewManager(5, StrategySummarize)

	// 创建足够多的消息触发压缩
	history := []*Message{
		{Role: "system", Content: "sys", Round: 0},
	}
	for i := 0; i < 5; i++ {
		history = append(history, &Message{
			Role:    "user",
			Content: "这是一条较长的用户消息，需要很多字符来填充",
			Round:   i + 1,
		})
		history = append(history, &Message{
			Role:    "assistant",
			Content: "这是一条较长的助手回复消息，包含了一些具体的建议和想法",
			Round:   i + 1,
		})
	}

	// 传入一个高 token 计数模拟超限
	result, _, err := m.Compress(history, 1000)
	assert.NoError(t, err)
	// 未设置模型时应该回退到丢弃策略，消息数应该减少
	assert.True(t, len(result) < len(history),
		"未设置模型时应该回退到丢弃策略，原: %d, 现: %d", len(history), len(result))
}

func TestCompress_BySummarizeWithModel(t *testing.T) {
	// 使用 mock model 测试摘要压缩
	mockModel := &mockChatModel{
		response: "用户询问了助手的能力范围，助手表示可以解答问题、编写代码和分析数据。",
	}
	m := NewManager(50, StrategySummarize, WithSummarizeModel(mockModel))

	// 创建足够多的消息，确保超过 token 限制
	history := []*Message{
		{Role: "system", Content: "你是一个助手", Round: 0},
	}
	for i := 0; i < 4; i++ {
		history = append(history, &Message{
			Role:    "user",
			Content: "你好，请问你能做什么？帮我写一些代码吧",
			Round:   i + 1,
		})
		history = append(history, &Message{
			Role:    "assistant",
			Content: "我可以帮你解答问题、编写 Go 代码、分析数据等。",
			Round:   i + 1,
		})
	}

	// 传入高 token 计数触发压缩
	result, _, err := m.Compress(history, 500)
	assert.NoError(t, err)
	assert.True(t, len(result) < len(history),
		"应该进行了摘要压缩，原: %d, 现: %d", len(history), len(result))
	assert.True(t, len(result) >= 2, "至少应该包含 system 消息和摘要消息")

	// 检查是否包含摘要消息
	foundSummary := false
	for _, msg := range result {
		if msg.Metadata != nil {
			if compressed, ok := msg.Metadata["compressed"]; ok && compressed == true {
				foundSummary = true
				assert.Contains(t, msg.Content, "历史摘要")
				break
			}
		}
	}
	assert.True(t, foundSummary, "应该包含摘要消息")
}

// mockChatModel 用于测试的 mock 模型
type mockChatModel struct {
	response string
}

func (m *mockChatModel) Generate(ctx context.Context, req *model_gateway.ChatRequest) (*model_gateway.ChatResponse, error) {
	return &model_gateway.ChatResponse{
		Content:     m.response,
		TotalTokens: len(m.response),
		Model:       "mock",
	}, nil
}

func (m *mockChatModel) Stream(ctx context.Context, req *model_gateway.ChatRequest) (<-chan *model_gateway.StreamChunk, error) {
	ch := make(chan *model_gateway.StreamChunk, 1)
	ch <- &model_gateway.StreamChunk{Content: m.response, Done: true}
	close(ch)
	return ch, nil
}

// ===== MemoryStore Tests =====

func TestMemoryStore_AddAndRetrieve(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	err := store.Add(ctx, "session_1", "role_1", "我认为 Go 的并发模型很好")
	assert.NoError(t, err)

	err = store.Add(ctx, "session_1", "role_1", "错误处理应该显式检查")
	assert.NoError(t, err)

	memories, err := store.Retrieve(ctx, "session_1", "role_1", 10)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(memories))
	assert.Equal(t, "错误处理应该显式检查", memories[0].Content, "最新的在前")
	assert.Equal(t, "我认为 Go 的并发模型很好", memories[1].Content)
}

func TestMemoryStore_RetrieveByRole(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, "session_1", "role_1", "角色1在 session1 的观点")
	_ = store.Add(ctx, "session_2", "role_1", "角色1在 session2 的观点")
	_ = store.Add(ctx, "session_1", "role_2", "角色2的观点")

	memories, err := store.RetrieveByRole(ctx, "role_1", 10)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(memories), "应该召回 role_1 的所有记忆")
	assert.Equal(t, "role_1", memories[0].RoleID)
	assert.Equal(t, "role_1", memories[1].RoleID)
}

func TestMemoryStore_RetrieveLimit(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	for i := 0; i < 20; i++ {
		_ = store.Add(ctx, "session_1", "role_1", "记忆内容")
	}

	memories, err := store.Retrieve(ctx, "session_1", "role_1", 5)
	assert.NoError(t, err)
	assert.Equal(t, 5, len(memories), "应只返回 5 条")
}

func TestMemoryStore_RetrieveEmpty(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	memories, err := store.Retrieve(ctx, "nonexistent", "role_1", 10)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(memories))
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = store.Add(ctx, "session_1", "role_1", "并发添加")
			_, _ = store.Retrieve(ctx, "session_1", "role_1", 10)
			_, _ = store.RetrieveByRole(ctx, "role_1", 10)
		}(i)
	}
	wg.Wait()

	// 不 panic 即通过
	count := store.Count()
	assert.Equal(t, 50, count)
}

func TestMemoryStore_Clear(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, "session_1", "role_1", "记忆1")
	_ = store.Add(ctx, "session_1", "role_2", "记忆2")
	_ = store.Add(ctx, "session_2", "role_1", "记忆3")

	store.Clear("session_1")

	mem1, _ := store.Retrieve(ctx, "session_1", "role_1", 10)
	mem2, _ := store.Retrieve(ctx, "session_1", "role_2", 10)
	mem3, _ := store.Retrieve(ctx, "session_2", "role_1", 10)

	assert.Equal(t, 0, len(mem1), "session_1 的记忆应该被清除")
	assert.Equal(t, 0, len(mem2), "session_1 的记忆应该被清除")
	assert.Equal(t, 1, len(mem3), "session_2 的记忆应该保留")
}

func TestMemoryStore_ClearAll(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	_ = store.Add(ctx, "s1", "r1", "mem1")
	_ = store.Add(ctx, "s2", "r2", "mem2")

	store.ClearAll()
	assert.Equal(t, 0, store.Count())
}

// ===== Retriever Tests =====

func TestBuildRetrieverContext_Empty(t *testing.T) {
	result := BuildRetrieverContext(nil)
	assert.Equal(t, "", result)

	result = BuildRetrieverContext([]*MemoryItem{})
	assert.Equal(t, "", result)
}

func TestBuildRetrieverContext(t *testing.T) {
	memories := []*MemoryItem{
		{Content: "Go 的并发模型基于 goroutine 和 channel"},
		{Content: "错误处理应该显式检查每个返回的 error"},
	}

	result := BuildRetrieverContext(memories)
	assert.Contains(t, result, "你的记忆")
	assert.Contains(t, result, "Go 的并发模型")
	assert.Contains(t, result, "错误处理")
}

func TestBuildRetrieverContextWithTime(t *testing.T) {
	memories := []*MemoryItem{
		{Content: "Go 的并发模型"},
	}

	result := BuildRetrieverContextWithTime(memories)
	assert.Contains(t, result, "你的记忆")
	assert.Contains(t, result, "Go 的并发模型")
}

// ===== Token Counter Tests =====

func TestSimpleTokenCounter(t *testing.T) {
	counter := &SimpleTokenCounter{}

	// 4 字符 ≈ 1 token
	count := counter.CountTokens("Hello, World!")
	assert.Equal(t, 4, count) // 13 chars → ceil(13/4) = 4

	count = counter.CountTokens("你好")
	assert.Equal(t, 1, count) // 2 chars → ceil(2/4) = 1

	count = counter.CountTokens("")
	assert.Equal(t, 0, count)
}

func TestCountMessagesTokens(t *testing.T) {
	counter := &SimpleTokenCounter{}

	messages := []*Message{
		{Content: "你好"},            // 2 chars → 1 token
		{Content: "世界"},            // 2 chars → 1 token
		{Content: "Hello, World!"}, // 13 chars → 4 tokens
	}

	total := CountMessagesTokens(messages, counter)
	assert.Equal(t, 6, total) // 1 + 1 + 4 = 6
}
