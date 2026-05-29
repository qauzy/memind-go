package textsearch

import memind "github.com/openmemind/memind-go"

// SearchTarget - 全文搜索目标类型
type SearchTarget string

const (
	TargetItem    SearchTarget = "ITEM"
	TargetInsight SearchTarget = "INSIGHT"
	TargetRawData SearchTarget = "RAW_DATA"
)

// Result - 全文搜索结果
type Result struct {
	DocumentID string  `json:"documentId"`
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
}

// MemoryTextSearch - 全文搜索接口，基于 BM25 实现
type MemoryTextSearch interface {
	Search(memoryID memind.MemoryId, query string, topK int, target SearchTarget) ([]Result, error)
	Index(memoryID memind.MemoryId, documentID string, text string, target SearchTarget) error
	IndexBatch(memoryID memind.MemoryId, documents map[string]string, target SearchTarget) error
	Remove(memoryID memind.MemoryId, documentID string, target SearchTarget) error
	Invalidate(memoryID memind.MemoryId) error
	ClearAll() error
}
