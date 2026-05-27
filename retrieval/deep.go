package retrieval

import (
	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/llm"
	"github.com/openmemind/memind-go/store"
	tsearch "github.com/openmemind/memind-go/textsearch"
	"github.com/openmemind/memind-go/vector"
)

// DeepStrategy - 深度检索策略：Simple + LLM 重排序
type DeepStrategy struct {
	memStore   store.MemoryStore
	vecStore   vector.MemoryVector
	textSearch tsearch.MemoryTextSearch
	llm        *llm.ChatClientRegistry
	simple     *SimpleStrategy
}

// NewDeepStrategy - 创建深度策略
func NewDeepStrategy(
	memStore store.MemoryStore,
	vecStore vector.MemoryVector,
	textSearch tsearch.MemoryTextSearch,
	llm *llm.ChatClientRegistry,
) *DeepStrategy {
	return &DeepStrategy{
		memStore:   memStore,
		vecStore:   vecStore,
		textSearch: textSearch,
		llm:        llm,
		simple:     NewSimpleStrategy(memStore, vecStore, textSearch),
	}
}

// Name - 返回策略名称
func (s *DeepStrategy) Name() string { return string(memind.StrategyDeep) }

// Retrieve - 先执行 Simple 检索，再对结果执行 LLM 重排序
func (s *DeepStrategy) Retrieve(ctx QueryContext, config memind.RetrievalConfig) (*memind.RetrievalResult, error) {
	query := ctx.SearchQuery()
	if query == "" {
		return &memind.RetrievalResult{Status: memind.RetrievalEmpty}, nil
	}

	result, err := s.simple.Retrieve(ctx, config)
	if err != nil {
		return nil, err
	}

	if config.Rerank.Enabled && len(result.Items) > 0 {
		reranked := s.rerank(query, result.Items, config.Rerank)
		result.Items = reranked
	}

	result.Strategy = string(memind.StrategyDeep)
	result.Status = memind.RetrievalSuccess
	return result, nil
}

// rerank - 基于位置的简单重排序（后续可替换为 LLM 重排序）
func (s *DeepStrategy) rerank(query string, items []ScoredResult, cfg memind.RerankConfig) []ScoredResult {
	if cfg.BlendWithRetrieval {
		for i := range items {
			baseBoost := 0.0
			if i < 3 {
				baseBoost = cfg.Top3Weight
			} else if i < 10 {
				baseBoost = cfg.Top10Weight
			} else {
				baseBoost = cfg.OtherWeight
			}
			items[i].FinalScore += baseBoost
		}
	}
	if len(items) > cfg.TopK && cfg.TopK > 0 {
		items = items[:cfg.TopK]
	}
	return items
}
