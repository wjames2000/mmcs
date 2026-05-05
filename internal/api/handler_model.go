package api

import (
	"encoding/json"
	"net/http"

	"github.com/wjames2000/mmcs/internal/api/middleware"
	"github.com/wjames2000/mmcs/internal/model_gateway"
)

type ModelHandler struct {
	gateway *model_gateway.Gateway
}

func NewModelHandler(gateway *model_gateway.Gateway) *ModelHandler {
	return &ModelHandler{gateway: gateway}
}

func (h *ModelHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	middleware.WriteSuccess(w, h.gateway.ListProviders())
}

func (h *ModelHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	middleware.WriteSuccess(w, h.gateway.ListModelProviders())
}

func (h *ModelHandler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var p model_gateway.ModelProvider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}
	if err := h.gateway.CreateModelProvider(&p); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	middleware.WriteCreated(w, p)
}

func (h *ModelHandler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少提供商 ID")
		return
	}
	var p model_gateway.ModelProvider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		middleware.WriteBadRequest(w, "无效的请求体")
		return
	}
	p.ID = id
	if err := h.gateway.UpdateModelProvider(&p); err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	middleware.WriteSuccess(w, p)
}

func (h *ModelHandler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少提供商 ID")
		return
	}
	if err := h.gateway.DeleteModelProvider(id); err != nil {
		middleware.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ModelHandler) ToggleProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		middleware.WriteBadRequest(w, "缺少提供商 ID")
		return
	}
	if err := h.gateway.ToggleModelProvider(id); err != nil {
		middleware.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	middleware.WriteSuccess(w, map[string]string{"status": "ok"})
}

func (h *ModelHandler) RefreshModels(w http.ResponseWriter, r *http.Request) {
	providerName := r.PathValue("providerName")
	if providerName == "" {
		middleware.WriteBadRequest(w, "缺少提供商名称")
		return
	}
	models, err := h.gateway.RefreshModelsFromProvider(providerName)
	if err != nil {
		middleware.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	middleware.WriteSuccess(w, models)
}
