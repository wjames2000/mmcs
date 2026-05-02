package api

import (
	"encoding/json"
	"net/http"

	"github.com/mmcs/internal/agent"
	"github.com/mmcs/internal/api/middleware"
	"github.com/rs/zerolog/log"
)

// AgentHandler Agent 相关 HTTP handler
type AgentHandler struct {
	executor *agent.Executor
}

// NewAgentHandler 创建 Agent handler
func NewAgentHandler(executor *agent.Executor) *AgentHandler {
	return &AgentHandler{executor: executor}
}

// agentRegisterRequest 注册 Agent 请求体
type agentRegisterRequest struct {
	AgentID string `json:"agent_id"`
}

// agentExecuteRequest 执行 Agent 的请求体
type agentExecuteRequest struct {
	Input string `json:"input"`
}

// Register 注册 Agent
// POST /api/v1/agents
func (h *AgentHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req agentRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	if req.AgentID == "" {
		middleware.WriteBadRequest(w, "agent_id 不能为空")
		return
	}

	// 通过注册表注册 Agent（仅作为 HTTP 端点）
	// 实际 Agent 实例需在启动时预先注册到 Executor
	if _, err := h.executor.Get(req.AgentID); err != nil {
		middleware.WriteBadRequest(w, "Agent "+req.AgentID+" 未在启动时注册到 Executor，请先注册")
		return
	}

	middleware.WriteCreated(w, map[string]string{
		"agent_id": req.AgentID,
		"status":   "registered",
	})
}

// ListAgents 列出已注册 Agent
// GET /api/v1/agents
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	agents := h.executor.List()
	middleware.WriteSuccess(w, map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	})
}

// ExecuteSync 同步执行 Agent
// POST /api/v1/agents/{id}/execute
func (h *AgentHandler) ExecuteSync(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")
	if agentID == "" {
		middleware.WriteBadRequest(w, "缺少 Agent ID")
		return
	}

	var req agentExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	result, err := h.executor.ExecuteSync(r.Context(), agentID, req.Input)
	if err != nil {
		log.Error().Err(err).Str("agent_id", agentID).Msg("同步执行 Agent 失败")
		middleware.WriteInternalError(w, "Agent 执行失败: "+err.Error())
		return
	}

	middleware.WriteSuccess(w, result)
}

// ExecuteAsync 异步执行 Agent
// POST /api/v1/agents/{id}/execute-async
func (h *AgentHandler) ExecuteAsync(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")
	if agentID == "" {
		middleware.WriteBadRequest(w, "缺少 Agent ID")
		return
	}

	var req agentExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}

	taskID, err := h.executor.ExecuteAsync(r.Context(), agentID, req.Input)
	if err != nil {
		log.Error().Err(err).Str("agent_id", agentID).Msg("异步执行 Agent 失败")
		middleware.WriteInternalError(w, "异步执行失败: "+err.Error())
		return
	}

	middleware.WriteCreated(w, map[string]string{
		"task_id": taskID,
		"status":  "queued",
	})
}
