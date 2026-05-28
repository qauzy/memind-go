package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIEmbeddingClient - OpenAI API 兼容的文本嵌入客户端
type OpenAIEmbeddingClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// OpenAIEmbeddingOption - 嵌入客户端配置选项
type OpenAIEmbeddingOption func(*OpenAIEmbeddingClient)

// WithEmbeddingBaseURL - 设置 embedding API 端点（默认 https://api.openai.com/v1）
func WithEmbeddingBaseURL(url string) OpenAIEmbeddingOption {
	return func(c *OpenAIEmbeddingClient) { c.baseURL = url }
}

// WithEmbeddingModel - 设置嵌入模型（默认 text-embedding-3-small）
func WithEmbeddingModel(model string) OpenAIEmbeddingOption {
	return func(c *OpenAIEmbeddingClient) { c.model = model }
}

// WithEmbeddingTimeout - 设置 HTTP 超时
func WithEmbeddingTimeout(d time.Duration) OpenAIEmbeddingOption {
	return func(c *OpenAIEmbeddingClient) { c.httpClient.Timeout = d }
}

// NewOpenAIEmbeddingClient - 创建 OpenAI 兼容的嵌入客户端
func NewOpenAIEmbeddingClient(apiKey string, opts ...OpenAIEmbeddingOption) *OpenAIEmbeddingClient {
	c := &OpenAIEmbeddingClient{
		apiKey:     apiKey,
		baseURL:    "https://api.openai.com/v1",
		model:      "text-embedding-3-small",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// embeddingRequest - OpenAI embeddings API 请求体
type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embeddingResponse - OpenAI embeddings API 响应体
type embeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed - 对单段文本计算嵌入向量
func (c *OpenAIEmbeddingClient) Embed(text string) ([]float32, error) {
	results, err := c.EmbedAll([]string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty embedding result")
	}
	return results[0], nil
}

// EmbedAll - 对多段文本批量计算嵌入向量
func (c *OpenAIEmbeddingClient) EmbedAll(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	body := embeddingRequest{
		Model: c.model,
		Input: texts,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/embeddings", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var embResp embeddingResponse
	if err := json.Unmarshal(raw, &embResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if embResp.Error != nil {
		return nil, fmt.Errorf("api error: %s", embResp.Error.Message)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("empty embeddings in response")
	}

	results := make([][]float32, len(embResp.Data))
	for i, d := range embResp.Data {
		vec := make([]float32, len(d.Embedding))
		for j, v := range d.Embedding {
			vec[j] = float32(v)
		}
		results[i] = vec
	}
	return results, nil
}
