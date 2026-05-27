package retrieval

import (
	"sort"

	memind "github.com/openmemind/memind-go"
)

type AdmissionResult struct {
	Decision  AdmissionDecision
	Reason    string
	TokenCount int
	CharCount  int
}

type AdmissionDecision int

const (
	Admit       AdmissionDecision = 0
	Skip        AdmissionDecision = 1
	QueryTooLong AdmissionDecision = 2
	Reject      AdmissionDecision = 3
)

type QueryContext struct {
	MemoryID            memind.MemoryId
	OriginalQuery       string
	RewrittenQuery      string
	ConversationHistory []string
	Metadata            map[string]any
	Scope               *memind.MemoryScope
	Categories          []memind.MemoryCategory
}

func (q QueryContext) SearchQuery() string {
	if q.RewrittenQuery != "" {
		return q.RewrittenQuery
	}
	return q.OriginalQuery
}

type RetrievalIntent string

const (
	IntentRetrieve RetrievalIntent = "RETRIEVE"
	IntentSkip     RetrievalIntent = "SKIP"
)

type IntentionRouter interface {
	Route(memoryID memind.MemoryId, query string, history []string) (RetrievalIntent, error)
}

type DefaultIntentionRouter struct{}

func (r *DefaultIntentionRouter) Route(memoryID memind.MemoryId, query string, history []string) (RetrievalIntent, error) {
	if query == "" {
		return IntentSkip, nil
	}
	return IntentRetrieve, nil
}

type ScoredResult = memind.ScoredResult

func MergeByRRF(results [][]ScoredResult, k int, vectorWeight, keywordWeight float64) []ScoredResult {
	seen := make(map[string]bool)
	scores := make(map[string]float64)
	vecScores := make(map[string]float32)
	texts := make(map[string]string)
	items := make(map[string]ScoredResult)

	for streamIdx, stream := range results {
		weight := 1.0
		if streamIdx == 0 {
			weight = vectorWeight
		} else if streamIdx == 1 {
			weight = keywordWeight
		}
		for rank, result := range stream {
			key := result.DedupKey()
			if !seen[key] {
				seen[key] = true
			}
			scores[key] += weight / float64(k+rank+1)
			vecScores[key] = result.VectorScore
			texts[key] = result.Text
			items[key] = result
		}
	}

	merged := make([]ScoredResult, 0, len(scores))
	for key := range scores {
		item := items[key]
		item.FinalScore = scores[key]
		item.VectorScore = vecScores[key]
		item.Text = texts[key]
		merged = append(merged, item)
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].FinalScore > merged[j].FinalScore
	})
	return merged
}

func AdaptiveTruncate(results []ScoredResult, maxItems int, maxTokens int) []ScoredResult {
	if len(results) <= maxItems {
		return results
	}
	return results[:maxItems]
}

type StrategyFactory struct {
	strategies map[string]RetrievalStrategy
}

func NewStrategyFactory() *StrategyFactory {
	return &StrategyFactory{
		strategies: make(map[string]RetrievalStrategy),
	}
}

func (f *StrategyFactory) Register(s RetrievalStrategy) {
	f.strategies[s.Name()] = s
}

func (f *StrategyFactory) Get(name string) (RetrievalStrategy, error) {
	if s, ok := f.strategies[name]; ok {
		return s, nil
	}
	return nil, memind.ErrStrategyNotFound
}

type RetrievalStrategy interface {
	Name() string
	Retrieve(ctx QueryContext, config memind.RetrievalConfig) (*memind.RetrievalResult, error)
}
