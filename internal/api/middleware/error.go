// Package middleware 提供 HTTP 中间件组件
package middleware

import (
	"encoding/json"
	"net/http"
)

// APIError 统一 API 错误响应
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// APIResponse 统一 API 成功响应
type APIResponse struct {
	Code int         `json:"code"`
	Data interface{} `json:"data,omitempty"`
	Meta interface{} `json:"meta,omitempty"`
}

// WriteJSON 写入 JSON 响应
func WriteJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// WriteSuccess 写入成功响应
func WriteSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, APIResponse{Code: 0, Data: data})
}

// WriteCreated 写入创建成功响应
func WriteCreated(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusCreated, APIResponse{Code: 0, Data: data})
}

// WriteError 写入错误响应
func WriteError(w http.ResponseWriter, statusCode int, message string, detail ...string) {
	err := APIError{
		Code:    statusCode,
		Message: message,
	}
	if len(detail) > 0 {
		err.Detail = detail[0]
	}
	WriteJSON(w, statusCode, err)
}

// WriteBadRequest 写入 400 错误
func WriteBadRequest(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusBadRequest, message)
}

// WriteUnauthorized 写入 401 错误
func WriteUnauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "未授权访问"
	}
	WriteError(w, http.StatusUnauthorized, message)
}

// WriteNotFound 写入 404 错误
func WriteNotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "资源不存在"
	}
	WriteError(w, http.StatusNotFound, message)
}

// WriteInternalError 写入 500 错误
func WriteInternalError(w http.ResponseWriter, message string) {
	if message == "" {
		message = "服务器内部错误"
	}
	WriteError(w, http.StatusInternalServerError, message)
}

// PanicRecovery 恐慌恢复中间件
// 捕获 handler 中的 panic，返回 500 错误
func PanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				WriteInternalError(w, "服务器内部错误")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ContentTypeJSON 设置 Content-Type 为 JSON 的中间件
func ContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

// CORS 跨域中间件
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
