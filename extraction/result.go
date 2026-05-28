package extraction

import memind "github.com/openmemind/memind-go"

// RawDataExtractResult - 原始数据提取中间结果
type RawDataExtractResult struct {
	RawDataList []*memind.MemoryRawData
	Existed     bool
}

// ItemExtractResult - 条目提取中间结果
type ItemExtractResult struct {
	NewItems []*memind.MemoryItem
	Types    []memind.MemoryInsightType
}

// InsightExtractResult - 洞察提取中间结果
type InsightExtractResult struct {
	Insights []*memind.MemoryInsight
	ByType   map[string][]*memind.MemoryInsight
}
