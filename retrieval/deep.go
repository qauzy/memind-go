package retrieval

import (
	"encoding/json"
	"fmt"
	"strings"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/llm"
	"github.com/openmemind/memind-go/store"
	tsearch "github.com/openmemind/memind-go/textsearch"
	"github.com/openmemind/memind-go/vector"
)

// rerankSystemPrompt - LLM 重排序系统提示词
const rerankSystemPrompt = `You are a search relevance judge. Score each candidate by relevance to the query.
Return a JSON array of scores (0.0 = irrelevant, 1.0 = highly relevant), one per candidate in order.`

// rerankUserPrompt - LLM 重排序用户提示词模板
// 参数依次为：查询、候选列表（每行 "文本"）
const rerankUserPrompt = `Query: %s

Candidates:
%s

Return ONLY a JSON array of float scores, e.g. [0.9, 0.3, 0.7]`

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

// rerank - 对检索结果执行重排序：优先 LLM 评分，回退为位置加成
func (s *DeepStrategy) rerank(query string, items []ScoredResult, cfg memind.RerankConfig) []ScoredResult {
	reranked := s.llmRerank(query, items)
	if reranked != nil {
		if len(reranked) > cfg.TopK && cfg.TopK > 0 {
			reranked = reranked[:cfg.TopK]
		}
		return reranked
	}

	// 回退：基于位置的加分重排序
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

// llmRerank - 调用 LLM 对候选列表进行相关性评分重排序
// LLM 不可用时返回 nil，由上层回退到位置加成
func (s *DeepStrategy) llmRerank(query string, items []ScoredResult) []ScoredResult {
	client := s.llm.Resolve(llm.SlotSufficiencyGate)
	if _, ok := client.(*llm.NoOpChatClient); ok {
		return nil
	}

	// 构建候选文本列表（最多 50 个，超出截断）
	maxCandidates := 50
	if len(items) > maxCandidates {
		items = items[:maxCandidates]
	}

	var candidateLines []string
	for i, item := range items {
		text := item.Text
		if len(text) > 200 {
			text = text[:200]
		}
		candidateLines = append(candidateLines, fmt.Sprintf("%d. %s", i+1, text))
	}
	candidatesStr := strings.Join(candidateLines, "\n")
	prompt := fmt.Sprintf(rerankUserPrompt, query, candidatesStr)

	resp, err := client.Call([]llm.ChatMessage{
		{Role: llm.RoleSystem, Content: rerankSystemPrompt},
		{Role: llm.RoleUser, Content: prompt},
	})
	if err != nil || resp == "" {
		return nil
	}

	var scores []float64
	if err := json.Unmarshal([]byte(resp), &scores); err != nil {
		return nil
	}
	if len(scores) != len(items) {
		return nil
	}

	// 用 LLM 评分覆盖 FinalScore 后重排序
	out := make([]ScoredResult, len(items))
	copy(out, items)
	for i := range out {
		out[i].FinalScore = scores[i]
	}
	// 冒泡排序（降序）
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].FinalScore > out[i].FinalScore {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
