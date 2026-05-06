package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/wjames2000/mmcs/config"
	"github.com/wjames2000/mmcs/internal/model_gateway"
)

// claudeRequest Anthropic Messages API 请求体
type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	System      string          `json:"system,omitempty"`
	Messages    []claudeMessage `json:"messages"`
	Temperature float32         `json:"temperature,omitempty"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// claudeResponse Anthropic Messages API 响应体
type claudeResponse struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Content []claudeContent `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// ClaudeProvider Anthropic/Claude 模型提供商
type ClaudeProvider struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewClaudeProvider 创建 Claude 提供商实例
func NewClaudeProvider(cfg config.ProviderConfig) (model_gateway.ChatModel, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	model := cfg.DefaultModel
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &ClaudeProvider{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiKey:       cfg.APIKey,
		defaultModel: model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// Generate 同步生成回复
func (p *ClaudeProvider) Generate(ctx context.Context, req *model_gateway.ChatRequest) (*model_gateway.ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	// 分离 system 消息和普通消息
	var systemPrompt string
	var messages []claudeMessage
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			if systemPrompt != "" {
				systemPrompt += "\n" + msg.Content
			} else {
				systemPrompt = msg.Content
			}
		} else {
			role := msg.Role
			if role == "assistant" {
				role = "assistant"
			} else {
				role = "user"
			}
			messages = append(messages, claudeMessage{Role: role, Content: msg.Content})
		}
	}

	claudeReq := claudeRequest{
		Model:       model,
		MaxTokens:   4096,
		System:      systemPrompt,
		Messages:    messages,
		Temperature: req.Temperature,
	}
	if req.MaxTokens > 0 {
		claudeReq.MaxTokens = req.MaxTokens
	}

	body, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Anthropic API 失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic API 返回错误状态 %d: %s", resp.StatusCode, string(respBody))
	}

	var claudeResp claudeResponse
	if err := json.Unmarshal(respBody, &claudeResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("Anthropic API 返回空结果")
	}

	totalTokens := claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens

	return &model_gateway.ChatResponse{
		Content:     claudeResp.Content[0].Text,
		TotalTokens: totalTokens,
		Model:       claudeResp.Model,
	}, nil
}

// Stream 流式生成回复（简化实现：非流式包装）
func (p *ClaudeProvider) Stream(ctx context.Context, req *model_gateway.ChatRequest) (<-chan *model_gateway.StreamChunk, error) {
	ch := make(chan *model_gateway.StreamChunk, 8)
	go func() {
		defer close(ch)
		resp, err := p.Generate(ctx, req)
		if err != nil {
			ch <- &model_gateway.StreamChunk{Error: err}
			return
		}
		ch <- &model_gateway.StreamChunk{Content: resp.Content}
		ch <- &model_gateway.StreamChunk{Done: true}
	}()
	return ch, nil
}
