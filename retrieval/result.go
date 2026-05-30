package retrieval

import (
	"sort"

	memind "github.com/openmemind/memind-go"
)

// AdmissionResult - 准入检查结果
type AdmissionResult struct {
	Decision   AdmissionDecision
	Reason     string
	TokenCount int
	CharCount  int
}

// AdmissionDecision - 准入决策类型
type AdmissionDecision int

const (
	Admit        AdmissionDecision = 0
	Skip         AdmissionDecision = 1
	QueryTooLong AdmissionDecision = 2
	Reject       AdmissionDecision = 3
)

// QueryContext - 搜索上下文，包含原始查询、重写查询、对话历史等
type QueryContext struct {
	MemoryID            memind.MemoryId
	OriginalQuery       string
	RewrittenQuery      string
	ConversationHistory []string
	Metadata            map[string]any
	Scope               *memind.MemoryScope
	Categories          []memind.MemoryCategory
}

// SearchQuery - 返回重写后的查询（如有），否则返回原始查询
func (q QueryContext) SearchQuery() string {
	if q.RewrittenQuery != "" {
		return q.RewrittenQuery
	}
	return q.OriginalQuery
}

// RetrievalIntent - 检索意图类型
type RetrievalIntent string

const (
	IntentRetrieve RetrievalIntent = "RETRIEVE"
	IntentSkip     RetrievalIntent = "SKIP"
)

// IntentionRouter - 意图路由器接口，决定是否需要执行检索
type IntentionRouter interface {
	Route(memoryID memind.MemoryId, query string, history []string) (RetrievalIntent, error)
}

// DefaultIntentionRouter - 默认意图路由器：非空查询即检索
type DefaultIntentionRouter struct{}

func (r *DefaultIntentionRouter) Route(memoryID memind.MemoryId, query string, history []string) (RetrievalIntent, error) {
	if query == "" {
		return IntentSkip, nil
	}
	return IntentRetrieve, nil
}

// ScoredResult - 带评分的结果项（类型别名，引用根包定义）
type ScoredResult = memind.ScoredResult

// MergeByRRF - 使用互惠排名融合（RRF）算法合并多个结果流
// RRF 评分 = Σ weight / (k + rank)，k 为平滑参数
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
			if result.VectorScore > 0 {
				vecScores[key] = result.VectorScore
			}
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

// AdaptiveTruncate - 自适应截断：按最大条目数和 Token 数限制
func AdaptiveTruncate(results []ScoredResult, maxItems int, maxTokens int) []ScoredResult {
	if len(results) <= maxItems {
		return results
	}
	return results[:maxItems]
}

// StrategyFactory - 检索策略工厂，按名称注册和获取策略
type StrategyFactory struct {
	strategies map[string]RetrievalStrategy
}

// NewStrategyFactory - 创建策略工厂
func NewStrategyFactory() *StrategyFactory {
	return &StrategyFactory{
		strategies: make(map[string]RetrievalStrategy),
	}
}

// Register - 注册检索策略
func (f *StrategyFactory) Register(s RetrievalStrategy) {
	f.strategies[s.Name()] = s
}

// Get - 按名称获取检索策略
func (f *StrategyFactory) Get(name string) (RetrievalStrategy, error) {
	if s, ok := f.strategies[name]; ok {
		return s, nil
	}
	return nil, memind.ErrStrategyNotFound
}

// RetrievalStrategy - 检索策略接口
type RetrievalStrategy interface {
	Name() string
	Retrieve(ctx QueryContext, config memind.RetrievalConfig) (*memind.RetrievalResult, error)
}
