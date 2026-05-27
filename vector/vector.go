package vector

import memind "github.com/openmemind/memind-go"

// SearchResult - 向量搜索结果
type SearchResult struct {
	VectorID string         `json:"vectorId"`
	Text     string         `json:"text"`
	Score    float32        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// MemoryVector - 向量存储接口，支持文本嵌入、存储和相似度搜索
type MemoryVector interface {
	Store(memoryID memind.MemoryId, text string, metadata map[string]any) (string, error)
	StoreWithID(memoryID memind.MemoryId, vectorID string, text string, metadata map[string]any) error
	StoreBatch(memoryID memind.MemoryId, texts []string, metadatas []map[string]any) ([]string, error)
	Delete(memoryID memind.MemoryId, vectorID string) error
	DeleteBatch(memoryID memind.MemoryId, vectorIDs []string) error
	Search(memoryID memind.MemoryId, query string, topK int) ([]SearchResult, error)
	SearchWithFilter(memoryID memind.MemoryId, query string, topK int, minScore float64, filter map[string]any) ([]SearchResult, error)
	Embed(text string) ([]float32, error)
	EmbedAll(texts []string) ([][]float32, error)
}
