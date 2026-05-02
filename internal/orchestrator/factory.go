package orchestrator

import (
	"fmt"

	"github.com/wjames2000/mmcs/internal/role"
)

// ParadigmType 讨论范式类型
type ParadigmType string

const (
	ParadigmRoundRobin ParadigmType = "round_robin"
	ParadigmCourt      ParadigmType = "court"
	ParadigmEvaluation ParadigmType = "evaluation"
	ParadigmFreeChat   ParadigmType = "free_chat"
)

// Factory Graph 工厂
// 根据范式类型分配合适的编排器
type Factory struct {
	roleService   RoleServiceInterface
	skillRegistry *role.SkillRegistry
	gateway       ModelGatewayInterface
}

// NewFactory 创建 Graph 工厂
func NewFactory(roleService RoleServiceInterface, skillRegistry *role.SkillRegistry, gateway ModelGatewayInterface) *Factory {
	return &Factory{
		roleService:   roleService,
		skillRegistry: skillRegistry,
		gateway:       gateway,
	}
}

// CreateOrchestrator 根据范式创建编排器
func (f *Factory) CreateOrchestrator(paradigm ParadigmType) (interface{}, error) {
	switch paradigm {
	case ParadigmRoundRobin:
		return f.createRoundRobin(), nil
	case ParadigmCourt:
		return f.createCourt(), nil
	case ParadigmEvaluation:
		return f.createEvaluation(), nil
	case ParadigmFreeChat:
		return f.createFreeChat(), nil
	default:
		return nil, fmt.Errorf("未知的讨论范式: %s", paradigm)
	}
}

// createRoundRobin 创建轮询发言编排器
func (f *Factory) createRoundRobin() *RoundRobinOrchestrator {
	contextInitNode := NewContextInitNode(f.roleService, f.skillRegistry, f.gateway)
	expertSpeakNode := NewExpertSpeakNode()
	moderatorEvalNode := NewModeratorEvalNode()
	summarizeNode := NewSummarizeNode()

	return NewRoundRobinOrchestrator(
		contextInitNode,
		expertSpeakNode,
		moderatorEvalNode,
		summarizeNode,
	)
}

// createCourt 创建法庭范式编排器
func (f *Factory) createCourt() *CourtOrchestrator {
	contextInitNode := NewContextInitNode(f.roleService, f.skillRegistry, f.gateway)
	expertSpeakNode := NewExpertSpeakNode()
	moderatorEvalNode := NewModeratorEvalNode()
	summarizeNode := NewSummarizeNode()

	return NewCourtOrchestrator(
		contextInitNode,
		expertSpeakNode,
		moderatorEvalNode,
		summarizeNode,
	)
}

// createFreeChat 创建自由群聊范式编排器
func (f *Factory) createFreeChat() *FreeChatOrchestrator {
	contextInitNode := NewContextInitNode(f.roleService, f.skillRegistry, f.gateway)
	expertSpeakNode := NewExpertSpeakNode()
	moderatorEvalNode := NewModeratorEvalNode()
	summarizeNode := NewSummarizeNode()

	return NewFreeChatOrchestrator(
		contextInitNode,
		expertSpeakNode,
		moderatorEvalNode,
		summarizeNode,
	)
}

// createEvaluation 创建加权评估范式编排器
func (f *Factory) createEvaluation() *EvaluationOrchestrator {
	contextInitNode := NewContextInitNode(f.roleService, f.skillRegistry, f.gateway)
	expertSpeakNode := NewExpertSpeakNode()
	moderatorEvalNode := NewModeratorEvalNode()
	summarizeNode := NewSummarizeNode()

	return NewEvaluationOrchestrator(
		contextInitNode,
		expertSpeakNode,
		moderatorEvalNode,
		summarizeNode,
	)
}
