package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIClient - OpenAI API 兼容的 LLM 客户端实现
// 支持标准文本对话（Call）和 JSON 结构化输出（CallStructured）
type OpenAIClient struct {
	apiKey      string
	baseURL     string
	model       string
	temperature float64
	httpClient  *http.Client
}

// OpenAIClientOption - 客户端配置选项
type OpenAIClientOption func(*OpenAIClient)

// WithModel - 设置模型名称（默认 gpt-4o）
func WithModel(model string) OpenAIClientOption {
	return func(c *OpenAIClient) { c.model = model }
}

// WithBaseURL - 设置 API 端点（默认 https://api.openai.com/v1）
func WithBaseURL(url string) OpenAIClientOption {
	return func(c *OpenAIClient) { c.baseURL = url }
}

// WithTemperature - 设置温度参数（默认 0.0）
func WithTemperature(t float64) OpenAIClientOption {
	return func(c *OpenAIClient) { c.temperature = t }
}

// WithTimeout - 设置 HTTP 超时（默认 60s）
func WithTimeout(d time.Duration) OpenAIClientOption {
	return func(c *OpenAIClient) { c.httpClient.Timeout = d }
}

// NewOpenAIClient - 创建 OpenAI 客户端
func NewOpenAIClient(apiKey string, opts ...OpenAIClientOption) *OpenAIClient {
	c := &OpenAIClient{
		apiKey:      apiKey,
		baseURL:     "https://api.openai.com/v1",
		model:       "gpt-4o",
		temperature: 0.0,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// chatMessage - OpenAI API 请求中的消息结构
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest - OpenAI API 请求体
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream,omitempty"`
}

// chatRequestStructured - 带 JSON 模式的结构化请求体
type chatRequestStructured struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

// chatResponse - OpenAI API 响应体
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// convertMessages - 将内部 ChatMessage 转为 OpenAI 格式
func convertMessages(msgs []ChatMessage) []chatMessage {
	out := make([]chatMessage, len(msgs))
	for i, m := range msgs {
		out[i] = chatMessage{Role: string(m.Role), Content: m.Content}
		// 统一角色名：SYSTEM→system, USER→user, ASSISTANT→assistant
		switch m.Role {
		case RoleSystem:
			out[i].Role = "system"
		case RoleUser:
			out[i].Role = "user"
		case RoleAssistant:
			out[i].Role = "assistant"
		}
	}
	return out
}

// Call - 发送对话请求，返回文本响应
func (c *OpenAIClient) Call(messages []ChatMessage) (string, error) {
	body := chatRequest{
		Model:       c.model,
		Messages:    convertMessages(messages),
		Temperature: c.temperature,
	}
	return c.doRequest(body)
}

// CallStructured - 发送对话请求，将响应解析到指定类型（强制 JSON 模式）
func (c *OpenAIClient) CallStructured(messages []ChatMessage, responseType any) error {
	body := chatRequestStructured{
		Model:          c.model,
		Messages:       convertMessages(messages),
		Temperature:    c.temperature,
		ResponseFormat: &responseFormat{Type: "json_object"},
	}
	resp, err := c.doRequest(body)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(resp), responseType)
}

// doRequest - 通用的 API 请求执行逻辑
func (c *OpenAIClient) doRequest(body any) (string, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(raw, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("api error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}
