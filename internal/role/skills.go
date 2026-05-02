package role

import (
	"fmt"
	"sync"
)

// SkillDefinition 技能定义
// Name 是技能唯一标识，Prompt 是注入到 system_prompt 中的内容
type SkillDefinition struct {
	Name        string
	Description string
	Prompt      string
}

// SkillRegistry 技能注册表（全局单例）
// 线程安全，支持并发读取
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]*SkillDefinition
}

// NewSkillRegistry 创建技能注册表并预置默认技能
func NewSkillRegistry() *SkillRegistry {
	reg := &SkillRegistry{
		skills: make(map[string]*SkillDefinition),
	}
	reg.registerDefaults()
	return reg
}

// registerDefaults 注册预置技能
func (r *SkillRegistry) registerDefaults() {
	defaults := []SkillDefinition{
		{
			Name:        "security-audit",
			Description: "安全审计与代码审查",
			Prompt: `你是安全审计专家。请从以下维度审查代码：
1. SQL 注入、XSS、CSRF 等常见安全漏洞
2. 敏感信息泄露（硬编码密钥、Token 等）
3. 权限校验是否完备
4. 输入验证与输出编码
5. 依赖项安全

对于每个发现的问题，请指定：
- 严重级别（Critical / High / Medium / Low）
- 具体位置
- 修复建议
- 参考 OWASP 分类`,
		},
		{
			Name:        "chinese-code-review",
			Description: "中文代码审查规范",
			Prompt: `你是资深 Go 语言代码审查专家。请遵循以下规范进行审查：
1. 代码可读性：命名清晰、注释恰当、函数长度合理
2. 错误处理：所有 error 必须处理，使用 %w 包装上下文
3. 并发安全：goroutine 生命周期管理、锁粒度、atomic 使用
4. 性能：避免不必要的内存分配、使用 sync.Pool、合理设置容量
5. 测试覆盖：关键路径要有单元测试，使用 t.Run 子测试

评审意见请使用中文书写，每个问题标注行号和严重程度。`,
		},
		{
			Name:        "performance-analysis",
			Description: "性能分析与优化建议",
			Prompt: `你是性能优化专家。请从以下方面分析：
1. CPU 热点：避免不必要的计算、合理使用缓存
2. 内存：减少分配和 GC 压力、使用对象池、避免内存泄漏
3. 并发：合理控制 goroutine 数量、使用 worker pool、避免锁竞争
4. I/O：批量读写、连接池、异步处理
5. 数据库：索引优化、查询优化、N+1 问题

请用数据说话，给出优化前后的预期效果对比。`,
		},
	}

	for _, s := range defaults {
		s := s // 捕获
		r.skills[s.Name] = &s
	}
}

// Register 注册自定义技能
func (r *SkillRegistry) Register(skill SkillDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[skill.Name]; exists {
		return fmt.Errorf("技能 %s 已存在", skill.Name)
	}
	r.skills[skill.Name] = &skill
	return nil
}

// Get 获取技能定义
func (r *SkillRegistry) Get(name string) (*SkillDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[name]
	if !ok {
		return nil, fmt.Errorf("技能 %s 不存在", name)
	}
	return skill, nil
}

// GetAll 获取所有技能
func (r *SkillRegistry) GetAll() []*SkillDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*SkillDefinition, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	return result
}

// ListNames 获取所有技能名称
func (r *SkillRegistry) ListNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	return names
}
