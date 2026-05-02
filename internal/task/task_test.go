package task

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mmcs/internal/minutes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService() *Service {
	return NewService(NewMemoryStore())
}

func TestCreateTask(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	req := &CreateRequest{
		SessionID:          "s_test123",
		WorkspaceID:        "w_test123",
		Title:              "实现用户登录功能",
		Description:        "需要实现 JWT 登录、刷新和登出",
		AcceptanceCriteria: "登录成功返回 token，刷新 token 有效，登出清除 token",
		Priority:           PriorityHigh,
		AssignedBy:         "user_abc",
		SourceRound:        1,
	}

	task, err := svc.Create(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, req.Title, task.Title)
	assert.Equal(t, req.WorkspaceID, task.WorkspaceID)
	assert.Equal(t, PriorityHigh, task.Priority)
	assert.Equal(t, StatusPending, task.Status)
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.UpdatedAt.IsZero())
	assert.Nil(t, task.CompletedAt)
}

func TestCreateTask_Validation(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// 空标题
	_, err := svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w_test",
		Title:       "",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "标题不能为空")

	// 空工作区 ID
	_, err = svc.Create(ctx, &CreateRequest{
		WorkspaceID: "",
		Title:       "test",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "工作区 ID 不能为空")

	// 默认优先级
	req := &CreateRequest{
		WorkspaceID: "w_test",
		Title:       "test",
	}
	task, err := svc.Create(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, PriorityMedium, task.Priority)
}

func TestUpdateStatus_ValidTransitions(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	task, err := svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w_test",
		Title:       "test task",
	})
	require.NoError(t, err)

	// pending → in_progress
	err = svc.UpdateStatus(ctx, task.ID, StatusInProgress)
	assert.NoError(t, err)
	t1, _ := svc.Get(ctx, task.ID)
	assert.Equal(t, StatusInProgress, t1.Status)

	// in_progress → reviewing
	err = svc.UpdateStatus(ctx, task.ID, StatusReviewing)
	assert.NoError(t, err)
	t2, _ := svc.Get(ctx, task.ID)
	assert.Equal(t, StatusReviewing, t2.Status)

	// reviewing → completed
	err = svc.UpdateStatus(ctx, task.ID, StatusCompleted)
	assert.NoError(t, err)
	t3, _ := svc.Get(ctx, task.ID)
	assert.Equal(t, StatusCompleted, t3.Status)
	assert.NotNil(t, t3.CompletedAt)

	// 重新创建任务用于 rejected 测试
	task2, err := svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w_test",
		Title:       "test task 2",
	})
	require.NoError(t, err)

	// pending → in_progress
	_ = svc.UpdateStatus(ctx, task2.ID, StatusInProgress)
	// in_progress → reviewing
	_ = svc.UpdateStatus(ctx, task2.ID, StatusReviewing)
	// reviewing → rejected
	err = svc.UpdateStatus(ctx, task2.ID, StatusRejected)
	assert.NoError(t, err)
	t4, _ := svc.Get(ctx, task2.ID)
	assert.Equal(t, StatusRejected, t4.Status)

	// rejected → in_progress
	err = svc.UpdateStatus(ctx, task2.ID, StatusInProgress)
	assert.NoError(t, err)
	t5, _ := svc.Get(ctx, task2.ID)
	assert.Equal(t, StatusInProgress, t5.Status)
}

func TestUpdateStatus_InvalidTransitions(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// 创建任务
	task, err := svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w_test",
		Title:       "test task",
	})
	require.NoError(t, err)

	// pending → completed（非法）
	err = svc.UpdateStatus(ctx, task.ID, StatusCompleted)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "非法状态转换")

	// pending → rejected（非法）
	err = svc.UpdateStatus(ctx, task.ID, StatusRejected)
	assert.Error(t, err)

	// pending → reviewing（非法）
	err = svc.UpdateStatus(ctx, task.ID, StatusReviewing)
	assert.Error(t, err)

	// in_progress → completed 后再尝试转换
	_ = svc.UpdateStatus(ctx, task.ID, StatusInProgress)
	_ = svc.UpdateStatus(ctx, task.ID, StatusReviewing)
	_ = svc.UpdateStatus(ctx, task.ID, StatusCompleted)

	// completed → anything（非法）
	err = svc.UpdateStatus(ctx, task.ID, StatusInProgress)
	assert.Error(t, err)

	err = svc.UpdateStatus(ctx, task.ID, StatusRejected)
	assert.Error(t, err)
}

func TestAssignAgent(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	task, err := svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w_test",
		Title:       "test task",
	})
	require.NoError(t, err)
	assert.Equal(t, StatusPending, task.Status)

	// 分配 Agent，应自动转为 in_progress
	err = svc.Assign(ctx, task.ID, "agent_llm", "user_admin")
	assert.NoError(t, err)

	t1, _ := svc.Get(ctx, task.ID)
	assert.Equal(t, "agent_llm", t1.AssignedAgent)
	assert.Equal(t, "user_admin", t1.AssignedBy)
	assert.Equal(t, StatusInProgress, t1.Status)
}

func TestExtractFromMinutes_WithDecisions(t *testing.T) {
	mm := &minutes.MeetingMinutes{
		SessionID: "s_test",
		Title:     "架构设计讨论",
		Decisions: []minutes.Decision{
			{
				Title:       "使用微服务架构",
				Description: "决定采用微服务架构，每个模块独立部署",
				Accepted:    true,
			},
			{
				Title:       "采用 Go 语言",
				Description: "后端统一使用 Go 语言开发，利用其并发优势",
				Accepted:    true,
			},
			{
				Title:       "使用 PostgreSQL",
				Description: "提议使用 MongoDB，但被否决",
				Accepted:    false,
			},
		},
	}

	tasks, err := ExtractFromMinutes(mm)
	require.NoError(t, err)
	require.Len(t, tasks, 2) // 只有两个 accepted 的 Decision

	assert.Equal(t, "使用微服务架构", tasks[0].Title)
	assert.Equal(t, "采用 Go 语言", tasks[1].Title)
	assert.Equal(t, StatusPending, tasks[0].Status)
	assert.Equal(t, PriorityMedium, tasks[0].Priority) // 默认 medium
}

func TestExtractFromMinutes_FromConclusion(t *testing.T) {
	mm := &minutes.MeetingMinutes{
		SessionID:  "s_test",
		Title:      "代码审查",
		Decisions:  []minutes.Decision{}, // 空决策列表
		Conclusion: "我们需要重构用户模块，应该添加单元测试，必须修复性能问题。\n建议引入 CI/CD 流程。",
	}

	tasks, err := ExtractFromMinutes(mm)
	require.NoError(t, err)
	require.NotEmpty(t, tasks)

	// 应该从 conclusion 中提取包含行动关键词的句子
	foundRefactor := false
	for _, t := range tasks {
		if t.Title == "" {
			continue
		}
		if len(t.AcceptanceCriteria) > 0 {
			foundRefactor = true
		}
	}
	assert.True(t, foundRefactor, "should extract at least one task from conclusion")
}

func TestExtractFromMinutes_NilMinutes(t *testing.T) {
	_, err := ExtractFromMinutes(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能为空")
}

func TestExtractFromMinutes_EmptyConclusion(t *testing.T) {
	mm := &minutes.MeetingMinutes{
		SessionID:  "s_test",
		Title:      "空讨论",
		Decisions:  []minutes.Decision{},
		Conclusion: "",
	}

	tasks, err := ExtractFromMinutes(mm)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestAutoAssign(t *testing.T) {
	agents := []AgentInfo{
		{ID: "agent_frontend", Tags: []string{"frontend", "react", "vue", "ui"}, CurrentLoad: 2},
		{ID: "agent_backend", Tags: []string{"backend", "go", "api", "database"}, CurrentLoad: 1},
		{ID: "agent_devops", Tags: []string{"devops", "ci", "cd", "deploy"}, CurrentLoad: 3},
	}

	task := &Task{
		Title:       "实现用户登录 API",
		Description: "使用 Go 实现 JWT 登录接口，连接 PostgreSQL",
	}

	// 应匹配 backend agent
	agentID := AutoAssign(task, agents)
	assert.Equal(t, "agent_backend", agentID)

	// Devops 任务
	task2 := &Task{
		Title:       "配置 CI/CD 流水线",
		Description: "使用 GitHub Actions 实现自动部署",
	}
	agentID2 := AutoAssign(task2, agents)
	assert.Equal(t, "agent_devops", agentID2)
}

func TestAutoAssign_EmptyAgents(t *testing.T) {
	task := &Task{
		Title:       "test",
		Description: "test",
	}

	agentID := AutoAssign(task, nil)
	assert.Empty(t, agentID)

	agentID = AutoAssign(task, []AgentInfo{})
	assert.Empty(t, agentID)
}

func TestAutoAssign_NoMatch(t *testing.T) {
	agents := []AgentInfo{
		{ID: "agent_marketing", Tags: []string{"marketing", "social"}, CurrentLoad: 0},
	}

	task := &Task{
		Title:       "实现登录功能",
		Description: "Go backend API",
	}

	agentID := AutoAssign(task, agents)
	assert.Empty(t, agentID)
}

func TestAutoAssign_LoadBalancing(t *testing.T) {
	// 两个 agent 都有匹配，但负载不同
	agents := []AgentInfo{
		{ID: "agent_backend_1", Tags: []string{"backend", "go", "api"}, CurrentLoad: 5},
		{ID: "agent_backend_2", Tags: []string{"backend", "go", "api"}, CurrentLoad: 1},
	}

	task := &Task{
		Title:       "实现登录 API",
		Description: "Go backend login API",
	}

	// 应选负载较低的
	agentID := AutoAssign(task, agents)
	assert.Equal(t, "agent_backend_2", agentID)
}

func TestListByWorkspace(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// 创建两个工作区的任务
	_, _ = svc.Create(ctx, &CreateRequest{WorkspaceID: "w1", Title: "task1"})
	_, _ = svc.Create(ctx, &CreateRequest{WorkspaceID: "w1", Title: "task2"})
	_, _ = svc.Create(ctx, &CreateRequest{WorkspaceID: "w2", Title: "task3"})

	tasks, err := svc.ListByWorkspace(ctx, "w1")
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	tasks2, err := svc.ListByWorkspace(ctx, "w2")
	require.NoError(t, err)
	assert.Len(t, tasks2, 1)

	// 不存在的 workspace
	tasks3, err := svc.ListByWorkspace(ctx, "w_nonexist")
	require.NoError(t, err)
	assert.Empty(t, tasks3)
}

func TestListBySession(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, _ = svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w1",
		SessionID:   "s1",
		Title:       "task1",
	})
	_, _ = svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w1",
		SessionID:   "s1",
		Title:       "task2",
	})
	_, _ = svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w1",
		SessionID:   "s2",
		Title:       "task3",
	})

	tasks, err := svc.ListBySession(ctx, "s1")
	require.NoError(t, err)
	assert.Len(t, tasks, 2)

	tasks2, err := svc.ListBySession(ctx, "s2")
	require.NoError(t, err)
	assert.Len(t, tasks2, 1)
}

func TestConcurrentAccess(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	// 并发创建
	var wg sync.WaitGroup
	const numTasks = 50
	errs := make(chan error, numTasks)

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.Create(ctx, &CreateRequest{
				WorkspaceID: "w_concurrent",
				Title:       "concurrent task",
				SourceRound: i,
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}

	tasks, err := svc.ListByWorkspace(ctx, "w_concurrent")
	require.NoError(t, err)
	assert.Len(t, tasks, numTasks)

	// 并发更新状态
	var wg2 sync.WaitGroup
	for _, t := range tasks {
		wg2.Add(1)
		go func(taskID string) {
			defer wg2.Done()
			_ = svc.UpdateStatus(ctx, taskID, StatusInProgress)
			_ = svc.UpdateStatus(ctx, taskID, StatusReviewing)
			_ = svc.UpdateStatus(ctx, taskID, StatusCompleted)
		}(t.ID)
	}
	wg2.Wait()

	// 验证所有任务最终状态
	allTasks, _ := svc.ListByWorkspace(ctx, "w_concurrent")
	for _, taskItem := range allTasks {
		assert.Equal(t, StatusCompleted, taskItem.Status)
	}
}

func TestGetNonExistentTask(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	_, err := svc.Get(ctx, "non_existent_id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

func TestValidatorTransition_PendingToInProgress(t *testing.T) {
	assert.True(t, IsValidTransition(StatusPending, StatusInProgress))
	assert.False(t, IsValidTransition(StatusPending, StatusCompleted))
	assert.False(t, IsValidTransition(StatusPending, StatusRejected))
}

func TestTerminalStatus(t *testing.T) {
	assert.True(t, TerminalStatus(StatusCompleted))
	assert.False(t, TerminalStatus(StatusPending))
	assert.False(t, TerminalStatus(StatusInProgress))
	assert.False(t, TerminalStatus(StatusRejected))
}

func TestAssign_HappyPath(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	task, err := svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w_test",
		Title:       "assign test",
	})
	require.NoError(t, err)

	err = svc.Assign(ctx, task.ID, "agent_test", "user_test")
	assert.NoError(t, err)

	updated, err := svc.Get(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, "agent_test", updated.AssignedAgent)
	assert.Equal(t, StatusInProgress, updated.Status)
}

func TestUpdateTaskContent(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()

	task, err := svc.Create(ctx, &CreateRequest{
		WorkspaceID: "w_test",
		Title:       "original title",
	})
	require.NoError(t, err)

	// 更新标题
	updated, err := svc.Update(ctx, task.ID, map[string]interface{}{
		"title":       "new title",
		"description": "new description",
	})
	require.NoError(t, err)
	assert.Equal(t, "new title", updated.Title)
	assert.Equal(t, "new description", updated.Description)

	// 验证原始状态未变
	assert.Equal(t, StatusPending, updated.Status)
}

func BenchmarkCreateTask(b *testing.B) {
	svc := newTestService()
	ctx := context.Background()
	req := &CreateRequest{
		WorkspaceID: "w_bench",
		Title:       "benchmark task",
		Priority:    PriorityMedium,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.Create(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestStoreConcurrency(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	// 并发写入
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			task := &Task{
				ID:          "t_" + string(rune('A'+i%26)) + string(rune('0'+i/10)),
				WorkspaceID: "w_conc",
				Title:       "conc task",
				Status:      StatusPending,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			_ = store.Create(ctx, task)
		}(i)
	}
	wg.Wait()

	// 并发读取
	var wg2 sync.WaitGroup
	store.mu.RLock()
	keys := make([]string, 0, len(store.tasks))
	for k := range store.tasks {
		keys = append(keys, k)
	}
	store.mu.RUnlock()

	for _, k := range keys {
		wg2.Add(1)
		go func(id string) {
			defer wg2.Done()
			_, _ = store.Get(ctx, id)
		}(k)
	}
	wg2.Wait()
}
