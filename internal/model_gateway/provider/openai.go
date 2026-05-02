// Package provider 提供模型提供商的具体实现
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

// OpenAIProvider OpenAI-compatible API 提供商
type OpenAIProvider struct {
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// NewOpenAIProvider 创建 OpenAI 提供商实例
func NewOpenAIProvider(cfg config.ProviderConfig) (model_gateway.ChatModel, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("OpenAI provider: base_url 不能为空")
	}
	return &OpenAIProvider{
		baseURL:      strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:       cfg.APIKey,
		defaultModel: cfg.DefaultModel,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

// openAIRequest OpenAI 兼容的请求体
type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float32         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIResponse OpenAI 兼容的响应体（非流式）
type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openAIStreamChunk OpenAI 兼容的流式响应片段
type openAIStreamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// Generate 同步生成回复
func (p *OpenAIProvider) Generate(ctx context.Context, req *model_gateway.ChatRequest) (*model_gateway.ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	openaiReq := openAIRequest{
		Model:       model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      false,
	}
	for _, msg := range req.Messages {
		openaiReq.Messages = append(openaiReq.Messages, openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 OpenAI API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API 返回错误状态 %d: %s", resp.StatusCode, string(respBody))
	}

	var openaiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openaiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI API 返回空结果")
	}

	return &model_gateway.ChatResponse{
		Content:     openaiResp.Choices[0].Message.Content,
		TotalTokens: openaiResp.Usage.TotalTokens,
		Model:       openaiResp.Model,
	}, nil
}

// Stream 流式生成回复
func (p *OpenAIProvider) Stream(ctx context.Context, req *model_gateway.ChatRequest) (<-chan *model_gateway.StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.defaultModel
	}

	openaiReq := openAIRequest{
		Model:       model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}
	for _, msg := range req.Messages {
		openaiReq.Messages = append(openaiReq.Messages, openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 OpenAI API 失败: %w", err)
	}

	ch := make(chan *model_gateway.StreamChunk, 64)
	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		// 增加缓冲区大小，防止大行截断
		scanner.Buffer(make([]byte, 0, 1024*64), 1024*64)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// SSE 格式: data: {...}
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- &model_gateway.StreamChunk{Done: true}
				return
			}

			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				ch <- &model_gateway.StreamChunk{Error: fmt.Errorf("解析流式数据失败: %w", err)}
				return
			}

			if len(chunk.Choices) > 0 {
				ch <- &model_gateway.StreamChunk{
					Content: chunk.Choices[0].Delta.Content,
					Done:    chunk.Choices[0].FinishReason == "stop",
				}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- &model_gateway.StreamChunk{Error: fmt.Errorf("读取流式数据失败: %w", err)}
		}
	}()

	return ch, nil
}
