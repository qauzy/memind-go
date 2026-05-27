package vector

import memind "github.com/openmemind/memind-go"

type SearchResult struct {
	VectorID string         `json:"vectorId"`
	Text     string         `json:"text"`
	Score    float32        `json:"score"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

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
