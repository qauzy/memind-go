package store

import (
	"time"

	memind "github.com/openmemind/memind-go"
)

type RawDataOperations interface {
	UpsertRawData(memoryID memind.MemoryId, rawData []*memind.MemoryRawData) error
	GetRawData(memoryID memind.MemoryId, rawDataID string) (*memind.MemoryRawData, error)
	GetRawDataByContentID(memoryID memind.MemoryId, contentID string) (*memind.MemoryRawData, error)
	ListRawDataByContentID(memoryID memind.MemoryId, contentID string) ([]*memind.MemoryRawData, error)
	ListRawData(memoryID memind.MemoryId) ([]*memind.MemoryRawData, error)
	PollRawDataWithoutVector(memoryID memind.MemoryId, limit int, minAge time.Duration) ([]*memind.MemoryRawData, error)
	UpdateRawDataVectorIDs(memoryID memind.MemoryId, vectorIDs map[string]string, metadataPatch map[string]any) error
	DeleteRawData(memoryID memind.MemoryId, rawDataID string) error
}

type ItemOperations interface {
	UpsertItems(memoryID memind.MemoryId, items []*memind.MemoryItem) error
	GetItem(memoryID memind.MemoryId, itemID int64) (*memind.MemoryItem, error)
	ListItems(memoryID memind.MemoryId) ([]*memind.MemoryItem, error)
	DeleteItems(memoryID memind.MemoryId, itemIDs []int64) error
	GetItemByHash(memoryID memind.MemoryId, hash string) (*memind.MemoryItem, error)
}

type InsightOperations interface {
	UpsertInsightTypes(types []*memind.MemoryInsightType) error
	GetInsightType(name string) (*memind.MemoryInsightType, error)
	ListInsightTypes() ([]*memind.MemoryInsightType, error)
	UpsertInsights(memoryID memind.MemoryId, insights []*memind.MemoryInsight) error
	GetInsight(memoryID memind.MemoryId, insightID int64) (*memind.MemoryInsight, error)
	ListInsights(memoryID memind.MemoryId) ([]*memind.MemoryInsight, error)
	GetInsightsByType(memoryID memind.MemoryId, insightType string) ([]*memind.MemoryInsight, error)
	GetInsightsByTier(memoryID memind.MemoryId, tier memind.InsightTier) ([]*memind.MemoryInsight, error)
	DeleteInsights(memoryID memind.MemoryId, insightIDs []int64) error
}

type MemoryStore interface {
	RawData() RawDataOperations
	Items() ItemOperations
	Insights() InsightOperations
}
