package extraction

// llmItemExtractionResponse - LLM 条目提取的完整响应结构
// Modified: 2026-05-28 - 从 Java MemoryItemExtractionResponse.java 移植
type llmItemExtractionResponse struct {
	Items []llmExtractedItem `json:"items"`
}

// llmExtractedItem - LLM 返回的单条提取条目
// Modified: 2026-05-28 - 对应 Java ExtractedItem record
type llmExtractedItem struct {
	Content        string         `json:"content"`
	Confidence     float64        `json:"confidence"`
	OccurredAt     string         `json:"occurredAt,omitempty"`
	InsightTypes   []string       `json:"insightTypes"`
	Category       string         `json:"category"`
	CategoryReason string         `json:"category_reason,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}
