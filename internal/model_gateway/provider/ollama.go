package provider

import (
	"bufio"
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

// OllamaProvider Ollama 本地模型提供商
type OllamaProvider struct {
	baseURL      string
	defaultModel string
	httpClient   *http.Client
}

// NewOllamaProvider 创建 Ollama 提供商实例
func NewOllamaProvider(cfg config.ProviderConfig) (model_gateway.ChatModel, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &OllamaProvider{
		baseURL:      strings.TrimRight(baseURL, "/"),
		defaultModel: cfg.DefaultModel,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}, nil
}

// ollamaRequest Ollama 聊天请求体
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaResponse Ollama 聊天响应体（非流式）
type ollamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	DoneReason string `json:"done_reason"`
	Done       bool   `json:"done"`
	EvalCount  int    `json:"eval_count"`
}

// ollamaStreamChunk Ollama 流式响应片段
type ollamaStreamChunk struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	DoneReason string `json:"done_reason"`
	Done       bool   `json:"done"`
}

// Generate 同步生成
func (p *OllamaProvider) Generate(ctx context.Context, req *model_gateway.ChatRequest) (*model_gateway.ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	ollamaReq := ollamaRequest{
		Model:  model,
		Stream: false,
	}
	for _, msg := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Ollama API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API 返回错误状态 %d: %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &model_gateway.ChatResponse{
		Content:     ollamaResp.Message.Content,
		TotalTokens: ollamaResp.EvalCount,
		Model:       ollamaResp.Model,
	}, nil
}

// Stream 流式生成
func (p *OllamaProvider) Stream(ctx context.Context, req *model_gateway.ChatRequest) (<-chan *model_gateway.StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	ollamaReq := ollamaRequest{
		Model:  model,
		Stream: true,
	}
	for _, msg := range req.Messages {
		ollamaReq.Messages = append(ollamaReq.Messages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Ollama API 失败: %w", err)
	}

	ch := make(chan *model_gateway.StreamChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 1024*64), 1024*64)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var chunk ollamaStreamChunk
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				ch <- &model_gateway.StreamChunk{Error: fmt.Errorf("解析流式数据失败: %w", err)}
				return
			}

			ch <- &model_gateway.StreamChunk{
				Content: chunk.Message.Content,
				Done:    chunk.Done,
			}

			if chunk.Done {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- &model_gateway.StreamChunk{Error: fmt.Errorf("读取流式数据失败: %w", err)}
		}
	}()

	return ch, nil
}
