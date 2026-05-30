package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OpenAIEmbeddingClient - OpenAI API 兼容的文本嵌入客户端
type OpenAIEmbeddingClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client

	// 用于适配非标准 API（如 MiniMax）
	inputField string // 请求中文本字段名，默认 "input"，MiniMax 为 "texts"
	dataField  string // 响应中数据字段名，默认 "data"，MiniMax 为 "vectors"
	errorField string // 响应中错误消息字段路径，默认 "error.message"，MiniMax 为 "base_resp"
}

// OpenAIEmbeddingOption - 嵌入客户端配置选项
type OpenAIEmbeddingOption func(*OpenAIEmbeddingClient)

// WithEmbeddingBaseURL - 设置 embedding API 端点
func WithEmbeddingBaseURL(url string) OpenAIEmbeddingOption {
	return func(c *OpenAIEmbeddingClient) { c.baseURL = url }
}

// WithEmbeddingModel - 设置嵌入模型
func WithEmbeddingModel(model string) OpenAIEmbeddingOption {
	return func(c *OpenAIEmbeddingClient) { c.model = model }
}

// WithEmbeddingTimeout - 设置 HTTP 超时
func WithEmbeddingTimeout(d time.Duration) OpenAIEmbeddingOption {
	return func(c *OpenAIEmbeddingClient) { c.httpClient.Timeout = d }
}

// NewOpenAIEmbeddingClient - 创建嵌入客户端，自动检测 MiniMax 等非标准 API
func NewOpenAIEmbeddingClient(apiKey string, opts ...OpenAIEmbeddingOption) *OpenAIEmbeddingClient {
	c := &OpenAIEmbeddingClient{
		apiKey:     apiKey,
		baseURL:    "https://api.openai.com/v1",
		model:      "text-embedding-3-small",
		httpClient: &http.Client{Timeout: 30 * time.Second},
		inputField: "input",
		dataField:  "data",
		errorField: "error.message",
	}
	for _, opt := range opts {
		opt(c)
	}
	// MiniMax 自动适配
	if strings.Contains(c.baseURL, "minimaxi") {
		c.inputField = "texts"
		c.dataField = "vectors"
		c.errorField = "base_resp"
	}
	return c
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

	var input any
	if len(texts) == 1 && c.inputField == "texts" {
		// MiniMax 单条时也传数组
		input = texts
	} else if len(texts) == 1 {
		input = texts[0]
	} else {
		input = texts
	}

	body := map[string]any{
		"model":      c.model,
		c.inputField: input,
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

	// 解析为通用 JSON
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	// MiniMax 错误检测
	if c.errorField == "base_resp" {
		if br, ok := result["base_resp"].(map[string]any); ok {
			if code, _ := br["status_code"].(float64); code != 0 {
				msg, _ := br["status_msg"].(string)
				return nil, fmt.Errorf("api error: %s (code=%.0f)", msg, code)
			}
		}
	} else {
		if errObj, ok := result["error"].(map[string]any); ok {
			msg, _ := errObj["message"].(string)
			return nil, fmt.Errorf("api error: %s", msg)
		}
	}

	// 提取向量数据
	dataRaw, ok := result[c.dataField]
	if !ok || dataRaw == nil {
		log.Printf("[EmbedAll] no %q field in response: %s", c.dataField, string(raw))
		return nil, fmt.Errorf("empty embeddings in response")
	}

	dataArr, ok := dataRaw.([]any)
	if !ok || len(dataArr) == 0 {
		log.Printf("[EmbedAll] empty %q field in response: %s", c.dataField, string(raw))
		return nil, fmt.Errorf("empty embeddings in response")
	}

	results := make([][]float32, 0, len(dataArr))
	for i, item := range dataArr {
		switch v := item.(type) {
		case map[string]any:
			// OpenAI 风格: {"embedding": [...], "index": 0}
			emb, ok := v["embedding"].([]any)
			if !ok {
				return nil, fmt.Errorf("data[%d]: missing embedding field", i)
			}
			vec := make([]float32, len(emb))
			for j, val := range emb {
				f, _ := val.(float64)
				vec[j] = float32(f)
			}
			results = append(results, vec)
		case []any:
			// MiniMax 风格: 直接是向量数组
			vec := make([]float32, len(v))
			for j, val := range v {
				f, _ := val.(float64)
				vec[j] = float32(f)
			}
			results = append(results, vec)
		default:
			return nil, fmt.Errorf("data[%d]: unexpected type %T", i, item)
		}
	}

	return results, nil
}
