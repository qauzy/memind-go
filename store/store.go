package store

import (
	"time"

	memind "github.com/openmemind/memind-go"
)

// RawDataOperations - 原始数据存储操作接口
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

// ItemOperations - 记忆条目存储操作接口
type ItemOperations interface {
	UpsertItems(memoryID memind.MemoryId, items []*memind.MemoryItem) error
	GetItem(memoryID memind.MemoryId, itemID int64) (*memind.MemoryItem, error)
	ListItems(memoryID memind.MemoryId) ([]*memind.MemoryItem, error)
	DeleteItems(memoryID memind.MemoryId, itemIDs []int64) error
	GetItemByHash(memoryID memind.MemoryId, hash string) (*memind.MemoryItem, error)
}

// InsightOperations - 洞察存储操作接口
type InsightOperations interface {
	UpsertInsightTypes(types []*memind.MemoryInsightType) error
	GetInsightType(name string) (*memind.MemoryInsightType, error)
	ListInsightTypes() ([]*memind.MemoryInsightType, error)
	UpsertInsights(memoryID memind.MemoryId, insights []*memind.MemoryInsight) error
	GetInsight(memoryID memind.MemoryId, insightID int64) (*memind.MemoryInsight, error)
	ListInsights(memoryID memind.MemoryId) ([]*memind.MemoryInsight, error)
	GetInsightsByType(memoryID memind.MemoryId, insightType string) ([]*memind.MemoryInsight, error)
	GetInsightsByTier(memoryID memind.MemoryId, tier memind.InsightTier) ([]*memind.MemoryInsight, error)
	GetLeafByGroup(memoryID memind.MemoryId, insightType, groupName string) (*memind.MemoryInsight, error)
	GetBranchByType(memoryID memind.MemoryId, typeName string) (*memind.MemoryInsight, error)
	GetRootByType(memoryID memind.MemoryId, rootTypeName string) (*memind.MemoryInsight, error)
	DeleteInsights(memoryID memind.MemoryId, insightIDs []int64) error
}

// MemoryStore - 存储层总接口，聚合原始数据、条目、洞察三类操作
type MemoryStore interface {
	RawData() RawDataOperations
	Items() ItemOperations
	Insights() InsightOperations
}
