package retrieval

import (
	"strings"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/llm"
	"github.com/openmemind/memind-go/store"
	tsearch "github.com/openmemind/memind-go/textsearch"
	"github.com/openmemind/memind-go/vector"
)

type MemoryRetriever interface {
	Retrieve(req memind.RetrievalRequest) (*memind.RetrievalResult, error)
	RegisterStrategy(s RetrievalStrategy)
	OnDataChanged(memoryID memind.MemoryId)
}

type DefaultRetriever struct {
	factory      *StrategyFactory
	router       IntentionRouter
	config       memind.RetrievalOptions
}

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

func (r *DefaultRetriever) RegisterStrategy(s RetrievalStrategy) {
	r.factory.Register(s)
}

func (r *DefaultRetriever) OnDataChanged(memoryID memind.MemoryId) {
}

func (r *DefaultRetriever) Retrieve(req memind.RetrievalRequest) (*memind.RetrievalResult, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return &memind.RetrievalResult{Status: memind.RetrievalEmpty}, nil
	}

	intent, err := r.router.Route(req.MemoryID, query, req.ConversationHistory)
	if err != nil {
		return nil, err
	}
	if intent == IntentSkip {
		return &memind.RetrievalResult{Status: memind.RetrievalEmpty}, nil
	}

	strategyName := string(r.config.Common.DefaultStrategy)
	if req.Config.StrategyConfig != (memind.StrategyConfig{}) {
		if req.Config != (memind.RetrievalConfig{}) {
		}
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
