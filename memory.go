package memind

import "time"

// Memory - memind 核心接口，提供记忆提取、检索、上下文构建等全部功能
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

// StoreProvider - 存储层提供者接口，供 engine 依赖注入
type StoreProvider interface {
	RawData() RawDataStore
	Items() ItemStore
	Insights() InsightStore
}

// RawDataStore - 原始数据存储接口
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

// ItemStore - 记忆条目存储接口
type ItemStore interface {
	UpsertItems(memoryID MemoryId, items []*MemoryItem) error
	GetItem(memoryID MemoryId, itemID int64) (*MemoryItem, error)
	ListItems(memoryID MemoryId) ([]*MemoryItem, error)
	DeleteItems(memoryID MemoryId, itemIDs []int64) error
	GetItemByHash(memoryID MemoryId, hash string) (*MemoryItem, error)
}

// InsightStore - 洞察存储接口
type InsightStore interface {
	UpsertInsightTypes(types []*MemoryInsightType) error
	GetInsightType(name string) (*MemoryInsightType, error)
	ListInsightTypes() ([]*MemoryInsightType, error)
	UpsertInsights(memoryID MemoryId, insights []*MemoryInsight) error
	GetInsight(memoryID MemoryId, insightID int64) (*MemoryInsight, error)
	ListInsights(memoryID MemoryId) ([]*MemoryInsight, error)
	GetInsightsByType(memoryID MemoryId, insightType string) ([]*MemoryInsight, error)
	GetInsightsByTier(memoryID MemoryId, tier InsightTier) ([]*MemoryInsight, error)
	GetBranchByType(memoryID MemoryId, typeName string) (*MemoryInsight, error)
	GetRootByType(memoryID MemoryId, rootTypeName string) (*MemoryInsight, error)
	DeleteInsights(memoryID MemoryId, insightIDs []int64) error
}

// ExtractorProvider - 提取器提供者接口
type ExtractorProvider interface {
	Extract(req ExtractionRequest) (*ExtractionResult, error)
	AddMessage(memoryID MemoryId, msg Message, config ExtractionConfig) (*ExtractionResult, error)
}

// RetrieverProvider - 检索器提供者接口
type RetrieverProvider interface {
	Retrieve(req RetrievalRequest) (*RetrievalResult, error)
	RegisterStrategy(strategyName string, strategy any)
	OnDataChanged(memoryID MemoryId)
}
