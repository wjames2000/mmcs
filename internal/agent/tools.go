package agent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Tool 通用工具接口
// 所有工具必须实现 Name、Description 和 Execute 方法
type Tool interface {
	// Name 返回工具唯一名称
	Name() string
	// Description 返回工具功能描述
	Description() string
	// Execute 执行工具逻辑
	// params: 工具参数键值对
	// 返回执行结果和可能的错误
	Execute(ctx context.Context, params map[string]any) (any, error)
}

// ToolRegistry 工具注册表
// 线程安全，支持并发注册和查找
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewToolRegistry 创建工具注册表
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register 注册工具
// 如果名称已存在则返回错误
func (r *ToolRegistry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("工具 %s 已注册", name)
	}

	r.tools[name] = t
	return nil
}

// Get 获取已注册的工具
func (r *ToolRegistry) Get(name string) (Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("工具 %s 未注册", name)
	}
	return tool, nil
}

// List 返回所有已注册的工具名称
func (r *ToolRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// ===== 预置工具 =====

// TaskCreator 任务创建器接口
// 用于 CreateTaskTool 向外部存储写入任务记录
type TaskCreator interface {
	// CreateTask 创建一条任务记录
	// 返回创建的任务 ID
	CreateTask(ctx context.Context, title, description, assignee string) (string, error)
}

// CreateTaskTool 创建任务工具
// 调用 TaskCreator 接口在数据库中创建任务记录
type CreateTaskTool struct {
	creator TaskCreator
}

// NewCreateTaskTool 创建任务创建工具
// creator: 任务创建器实现（可直接操作数据库或调用 task service）
func NewCreateTaskTool(creator TaskCreator) *CreateTaskTool {
	return &CreateTaskTool{creator: creator}
}

// Name 返回工具名称
func (t *CreateTaskTool) Name() string {
	return "create_task"
}

// Description 返回工具描述
func (t *CreateTaskTool) Description() string {
	return "创建一条新的任务记录，需要 title（标题）、description（描述）、assignee（负责人）参数"
}

// Execute 执行任务创建
// 期望 params 中包含 title, description, assignee 字段
func (t *CreateTaskTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	title, _ := params["title"].(string)
	description, _ := params["description"].(string)
	assignee, _ := params["assignee"].(string)

	if title == "" {
		return nil, fmt.Errorf("任务标题不能为空")
	}
	if assignee == "" {
		return nil, fmt.Errorf("任务负责人不能为空")
	}

	taskID, err := t.creator.CreateTask(ctx, title, description, assignee)
	if err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	return map[string]interface{}{
		"task_id": taskID,
		"title":   title,
		"status":  "created",
	}, nil
}

// QueryExecutor 查询执行器接口
// 用于 QueryDatabaseTool 执行只读 SQL 查询
type QueryExecutor interface {
	// Query 执行只读 SQL 查询，返回结果集
	Query(ctx context.Context, sql string) ([]map[string]any, error)
}

// QueryDatabaseTool 数据库查询工具
// 执行只读 SQL 查询，返回结果集
type QueryDatabaseTool struct {
	executor QueryExecutor
}

// NewQueryDatabaseTool 创建数据库查询工具
// executor: 查询执行器
func NewQueryDatabaseTool(executor QueryExecutor) *QueryDatabaseTool {
	return &QueryDatabaseTool{executor: executor}
}

// Name 返回工具名称
func (t *QueryDatabaseTool) Name() string {
	return "query_database"
}

// Description 返回工具描述
func (t *QueryDatabaseTool) Description() string {
	return "执行只读 SQL 查询语句，需要 sql 参数"
}

// Execute 执行数据库查询
// 期望 params 中包含 sql 字段
func (t *QueryDatabaseTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	sql, _ := params["sql"].(string)
	if sql == "" {
		return nil, fmt.Errorf("SQL 语句不能为空")
	}

	results, err := t.executor.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	return map[string]interface{}{
		"rows":  results,
		"count": len(results),
	}, nil
}

// ExecuteCodeTool 模拟代码执行工具
// 在沙箱模式下模拟返回执行结果（当前为模拟实现）
type ExecuteCodeTool struct {
	execTimeout time.Duration
}

// NewExecuteCodeTool 创建代码执行工具
// execTimeout: 单次执行超时时间
func NewExecuteCodeTool(timeout time.Duration) *ExecuteCodeTool {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &ExecuteCodeTool{execTimeout: timeout}
}

// Name 返回工具名称
func (t *ExecuteCodeTool) Name() string {
	return "execute_code"
}

// Description 返回工具描述
func (t *ExecuteCodeTool) Description() string {
	return "在沙箱环境中执行代码片段并返回结果（当前版本为模拟执行），需要 code（代码内容）、language（编程语言）参数"
}

// Execute 模拟执行代码
// 期望 params 中包含 code（代码内容）、language（编程语言）参数
// 当前版本始终返回模拟结果（sandbox 模式）
func (t *ExecuteCodeTool) Execute(ctx context.Context, params map[string]any) (any, error) {
	code, _ := params["code"].(string)
	language, _ := params["language"].(string)

	if code == "" {
		return nil, fmt.Errorf("代码内容不能为空")
	}
	if language == "" {
		language = "unknown"
	}

	// 模拟执行延迟
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	return map[string]interface{}{
		"stdout":    fmt.Sprintf("[模拟输出] 已执行 %s 代码（%d 字符）", language, len(code)),
		"stderr":    "",
		"exit_code": 0,
		"duration":  "100ms",
		"note":      "当前为模拟执行模式，实际代码未运行",
	}, nil
}
