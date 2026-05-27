package memind

import "time"

type Memory interface {
	Extract(req ExtractionRequest) (*ExtractionResult, error)
	AddMessages(memoryID MemoryId, messages []Message, config ExtractionConfig) (*ExtractionResult, error)
	AddMessage(memoryID MemoryId, message Message, config ExtractionConfig) (*ExtractionResult, error)
	Commit(memoryID MemoryId, config ExtractionConfig) (*ExtractionResult, error)
	Retrieve(req RetrievalRequest) (*RetrievalResult, error)
	GetContext(req ContextRequest) (*ContextWindow, error)
	DeleteItems(memoryID MemoryId, itemIDs []int64) error
	DeleteInsights(memoryID MemoryId, insightIDs []int64) error
	Close() error
}

type StoreProvider interface {
	RawData() RawDataStore
	Items() ItemStore
	Insights() InsightStore
}

type RawDataStore interface {
	UpsertRawData(memoryID MemoryId, rawData []*MemoryRawData) error
	GetRawData(memoryID MemoryId, rawDataID string) (*MemoryRawData, error)
	GetRawDataByContentID(memoryID MemoryId, contentID string) (*MemoryRawData, error)
	ListRawDataByContentID(memoryID MemoryId, contentID string) ([]*MemoryRawData, error)
	ListRawData(memoryID MemoryId) ([]*MemoryRawData, error)
	PollRawDataWithoutVector(memoryID MemoryId, limit int, minAge time.Duration) ([]*MemoryRawData, error)
	UpdateRawDataVectorIDs(memoryID MemoryId, vectorIDs map[string]string, metadataPatch map[string]any) error
	DeleteRawData(memoryID MemoryId, rawDataID string) error
}

type ItemStore interface {
	UpsertItems(memoryID MemoryId, items []*MemoryItem) error
	GetItem(memoryID MemoryId, itemID int64) (*MemoryItem, error)
	ListItems(memoryID MemoryId) ([]*MemoryItem, error)
	DeleteItems(memoryID MemoryId, itemIDs []int64) error
	GetItemByHash(memoryID MemoryId, hash string) (*MemoryItem, error)
}

type InsightStore interface {
	UpsertInsightTypes(types []*MemoryInsightType) error
	GetInsightType(name string) (*MemoryInsightType, error)
	ListInsightTypes() ([]*MemoryInsightType, error)
	UpsertInsights(memoryID MemoryId, insights []*MemoryInsight) error
	GetInsight(memoryID MemoryId, insightID int64) (*MemoryInsight, error)
	ListInsights(memoryID MemoryId) ([]*MemoryInsight, error)
	GetInsightsByType(memoryID MemoryId, insightType string) ([]*MemoryInsight, error)
	GetInsightsByTier(memoryID MemoryId, tier InsightTier) ([]*MemoryInsight, error)
	DeleteInsights(memoryID MemoryId, insightIDs []int64) error
}

type ExtractorProvider interface {
	Extract(req ExtractionRequest) (*ExtractionResult, error)
	AddMessage(memoryID MemoryId, msg Message, config ExtractionConfig) (*ExtractionResult, error)
}

type RetrieverProvider interface {
	Retrieve(req RetrievalRequest) (*RetrievalResult, error)
	RegisterStrategy(strategyName string, strategy any)
	OnDataChanged(memoryID MemoryId)
}

func (m Message) ContentString() string {
	var s string
	for _, b := range m.Content {
		s += b.Text + " "
	}
	return s
}
