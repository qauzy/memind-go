package retrieval

import (
	"log"
	"strings"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/llm"
	"github.com/openmemind/memind-go/store"
	tsearch "github.com/openmemind/memind-go/textsearch"
	"github.com/openmemind/memind-go/vector"
)

// MemoryRetriever - 记忆检索器接口
type MemoryRetriever interface {
	Retrieve(req memind.RetrievalRequest) (*memind.RetrievalResult, error)
	RegisterStrategy(s RetrievalStrategy)
	OnDataChanged(memoryID memind.MemoryId)
}

// DefaultRetriever - 默认记忆检索器，组合策略工厂和意图路由器
type DefaultRetriever struct {
	factory *StrategyFactory
	router  IntentionRouter
	config  memind.RetrievalOptions
}

// NewRetriever - 创建检索器，注册 Simple 和 Deep 两种策略
func NewRetriever(
	memStore store.MemoryStore,
	vecStore vector.MemoryVector,
	textSearch tsearch.MemoryTextSearch,
	llm *llm.ChatClientRegistry,
	opts memind.RetrievalOptions,
) *DefaultRetriever {
	factory := NewStrategyFactory()
	factory.Register(NewSimpleStrategy(memStore, vecStore, textSearch))
	factory.Register(NewDeepStrategy(memStore, vecStore, textSearch, llm))

	return &DefaultRetriever{
		factory: factory,
		router:  &DefaultIntentionRouter{},
		config:  opts,
	}
}

// RegisterStrategy - 注册自定义检索策略
func (r *DefaultRetriever) RegisterStrategy(s RetrievalStrategy) {
	r.factory.Register(s)
}

// OnDataChanged - 数据变更回调（当前为空操作）
func (r *DefaultRetriever) OnDataChanged(memoryID memind.MemoryId) {}

// Retrieve - 执行检索：准入检查 → 意图路由 → 策略分发
func (r *DefaultRetriever) Retrieve(req memind.RetrievalRequest) (*memind.RetrievalResult, error) {
	query := strings.TrimSpace(req.Query)
	log.Printf("[retriever.Retrieve] query=%q", query)
	if query == "" {
		return &memind.RetrievalResult{Status: memind.RetrievalEmpty}, nil
	}

	intent, err := r.router.Route(req.MemoryID, query, req.ConversationHistory)
	if err != nil {
		return nil, err
	}
	log.Printf("[retriever.Retrieve] intent=%v", intent)
	if intent == IntentSkip {
		return &memind.RetrievalResult{Status: memind.RetrievalEmpty}, nil
	}

	strategyName := string(r.config.Common.DefaultStrategy)
	log.Printf("[retriever.Retrieve] strategy=%q", strategyName)
	if req.Config.StrategyConfig != (memind.StrategyConfig{}) {
	}

	strategy, err := r.factory.Get(strategyName)
	if err != nil {
		return nil, err
	}

	ctx := QueryContext{
		MemoryID:            req.MemoryID,
		OriginalQuery:       query,
		ConversationHistory: req.ConversationHistory,
		Metadata:            req.Metadata,
		Scope:               req.Scope,
		Categories:          req.Categories,
	}

	config := req.Config
	if config == (memind.RetrievalConfig{}) {
		if strategyName == string(memind.StrategyDeep) {
			config = memind.DeepRetrievalConfig()
		} else {
			config = memind.SimpleRetrievalConfig()
		}
	}

	return strategy.Retrieve(ctx, config)
}
