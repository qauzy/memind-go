package extraction

import memind "github.com/openmemind/memind-go"

type RawDataExtractResult struct {
	RawDataList []*memind.MemoryRawData
	Existed     bool
}

type ItemExtractResult struct {
	NewItems []*memind.MemoryItem
	Types    []memind.MemoryInsightType
}

type InsightExtractResult struct {
	Insights []*memind.MemoryInsight
}
