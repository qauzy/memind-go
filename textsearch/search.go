package textsearch

import memind "github.com/openmemind/memind-go"

type SearchTarget string

const (
	TargetItem   SearchTarget = "ITEM"
	TargetInsight SearchTarget = "INSIGHT"
	TargetRawData SearchTarget = "RAW_DATA"
)

type Result struct {
	DocumentID string  `json:"documentId"`
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
}

type MemoryTextSearch interface {
	Search(memoryID memind.MemoryId, query string, topK int, target SearchTarget) ([]Result, error)
	Index(memoryID memind.MemoryId, documentID string, text string, target SearchTarget) error
	IndexBatch(memoryID memind.MemoryId, documents map[string]string, target SearchTarget) error
	Remove(memoryID memind.MemoryId, documentID string, target SearchTarget) error
	Invalidate(memoryID memind.MemoryId) error
}
