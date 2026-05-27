package retrieval

import (
	"math"
	"sort"
	"strings"
	"time"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/store"
	tsearch "github.com/openmemind/memind-go/textsearch"
	"github.com/openmemind/memind-go/vector"
)

type SimpleStrategy struct {
	memStore  store.MemoryStore
	vecStore  vector.MemoryVector
	textSearch tsearch.MemoryTextSearch
}

func NewSimpleStrategy(
	memStore store.MemoryStore,
	vecStore vector.MemoryVector,
	textSearch tsearch.MemoryTextSearch,
) *SimpleStrategy {
	return &SimpleStrategy{
		memStore:   memStore,
		vecStore:   vecStore,
		textSearch: textSearch,
	}
}

func (s *SimpleStrategy) Name() string { return string(memind.StrategySimple) }

func (s *SimpleStrategy) Retrieve(ctx QueryContext, config memind.RetrievalConfig) (*memind.RetrievalResult, error) {
	query := ctx.SearchQuery()
	if query == "" {
		return &memind.RetrievalResult{Status: memind.RetrievalEmpty}, nil
	}

	var allResults [][]ScoredResult

	if config.Tier1.Enabled && s.vecStore != nil {
		insightResults, err := s.searchInsights(ctx.MemoryID, query, config.Tier1)
		if err == nil && len(insightResults) > 0 {
			allResults = append(allResults, insightResults)
		}
	}

	if config.Tier2.Enabled {
		itemVecResults, err := s.searchItemsVector(ctx.MemoryID, query, config.Tier2)
		if err == nil && len(itemVecResults) > 0 {
			allResults = append(allResults, itemVecResults)
		}

		itemTextResults, err := s.searchItemsText(ctx.MemoryID, query, config.Tier2)
		if err == nil && len(itemTextResults) > 0 {
			allResults = append(allResults, itemTextResults)
		}
	}

	if config.Tier3.Enabled && s.vecStore != nil {
		rdResults, err := s.searchRawData(ctx.MemoryID, query, config.Tier3)
		if err == nil && len(rdResults) > 0 {
			allResults = append(allResults, rdResults)
		}
	}

	if len(allResults) == 0 {
		return &memind.RetrievalResult{Status: memind.RetrievalEmpty}, nil
	}

	sc := config.Scoring
	merged := MergeByRRF(allResults, sc.Fusion.K, sc.Fusion.VectorWeight, sc.Fusion.KeywordWeight)

	if config.Tier2.Truncation.Enabled {
		maxItems := config.Tier2.Truncation.MaxItems
		if maxItems <= 0 {
			maxItems = 30
		}
		merged = AdaptiveTruncate(merged, maxItems, config.Tier2.Truncation.TargetTokens)
	}

	items := make([]ScoredResult, 0)
	rInsights := make([]memind.RetrievedInsight, 0)
	rRawData := make([]memind.RetrievedRawData, 0)

	for _, r := range merged {
		if r.SourceType == "INSIGHT" {
			rInsights = append(rInsights, memind.RetrievedInsight{
				ID: r.SourceID, Text: r.Text,
			})
		} else if r.SourceType == "RAW_DATA" {
			rRawData = append(rRawData, memind.RetrievedRawData{
				RawDataID: r.SourceID, Caption: r.Text, MaxScore: r.FinalScore,
			})
		} else {
			items = append(items, r)
		}
	}

	return &memind.RetrievalResult{
		Items:    items,
		Insights: rInsights,
		RawData:  rRawData,
		Strategy: string(memind.StrategySimple),
		Query:    query,
		Status:   memind.RetrievalSuccess,
	}, nil
}

func (s *SimpleStrategy) searchInsights(memoryID memind.MemoryId, query string, cfg memind.TierConfig) ([]ScoredResult, error) {
	insights, err := s.memStore.Insights().ListInsights(memoryID)
	if err != nil {
		return nil, err
	}

	vecResults, err := s.vecStore.SearchWithFilter(memoryID, query, cfg.TopK*2, cfg.MinScore, map[string]any{"type": "insight"})
	if err != nil {
		return nil, err
	}

	insightMap := make(map[int64]*memind.MemoryInsight)
	for _, ins := range insights {
		insightMap[ins.ID] = ins
	}

	var results []ScoredResult
	for _, vr := range vecResults {
		id, _ := extractInsightID(vr.Metadata)
		text := vr.Text
		if ins, ok := insightMap[id]; ok {
			text = ins.PointsContent()
		}
		results = append(results, ScoredResult{
			SourceType:  "INSIGHT",
			SourceID:    vr.VectorID,
			Text:        text,
			VectorScore: vr.Score,
			FinalScore:  float64(vr.Score),
		})
	}
	return results, nil
}

func (s *SimpleStrategy) searchItemsVector(memoryID memind.MemoryId, query string, cfg memind.TierConfig) ([]ScoredResult, error) {
	vecResults, err := s.vecStore.SearchWithFilter(memoryID, query, cfg.TopK*2, cfg.MinScore, nil)
	if err != nil {
		return nil, err
	}

	var results []ScoredResult
	for _, vr := range vecResults {
		if vr.Metadata != nil {
			if t, ok := vr.Metadata["type"]; ok && t == "insight" {
				continue
			}
		}
		results = append(results, ScoredResult{
			SourceType:  "ITEM",
			SourceID:    vr.VectorID,
			Text:        vr.Text,
			VectorScore: vr.Score,
			FinalScore:  float64(vr.Score),
		})
	}
	return results, nil
}

func (s *SimpleStrategy) searchItemsText(memoryID memind.MemoryId, query string, cfg memind.TierConfig) ([]ScoredResult, error) {
	textResults, err := s.textSearch.Search(memoryID, query, cfg.TopK*2, tsearch.TargetItem)
	if err != nil {
		return nil, err
	}

	var results []ScoredResult
	for _, tr := range textResults {
		results = append(results, ScoredResult{
			SourceType:  "ITEM",
			SourceID:    tr.DocumentID,
			Text:        tr.Text,
			VectorScore: float32(0),
			FinalScore:  tr.Score,
		})
	}
	return results, nil
}

func (s *SimpleStrategy) searchRawData(memoryID memind.MemoryId, query string, cfg memind.TierConfig) ([]ScoredResult, error) {
	vecResults, err := s.vecStore.SearchWithFilter(memoryID, query, cfg.TopK*2, cfg.MinScore, map[string]any{"type": "rawdata"})
	if err != nil {
		return nil, err
	}

	var results []ScoredResult
	for _, vr := range vecResults {
		results = append(results, ScoredResult{
			SourceType:  "RAW_DATA",
			SourceID:    vr.VectorID,
			Text:        vr.Text,
			VectorScore: vr.Score,
			FinalScore:  float64(vr.Score),
		})
	}
	return results, nil
}

type TimeDecayFilter struct {
	Rate  float64
	Floor float64
}

func (f TimeDecayFilter) Apply(results []ScoredResult) {
	now := time.Now()
	for i, r := range results {
		if r.OccurredAt != nil {
			hours := now.Sub(*r.OccurredAt).Hours()
			decay := math.Exp(-f.Rate * hours)
			if decay < f.Floor {
				decay = f.Floor
			}
			results[i].FinalScore *= decay
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].FinalScore > results[j].FinalScore
	})
}

func extractInsightID(metadata map[string]any) (int64, bool) {
	if metadata == nil {
		return 0, false
	}
	if id, ok := metadata["insight_id"]; ok {
		switch v := id.(type) {
		case int64:
			return v, true
		case float64:
			return int64(v), true
		}
	}
	return 0, false
}

func admissionCheck(query string) AdmissionResult {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return AdmissionResult{Decision: Reject, Reason: "empty query", CharCount: 0}
	}
	if len([]rune(trimmed)) > 8000 {
		return AdmissionResult{Decision: QueryTooLong, Reason: "query too long", CharCount: len(trimmed)}
	}
	return AdmissionResult{Decision: Admit, CharCount: len(trimmed)}
}
