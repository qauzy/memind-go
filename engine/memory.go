package engine

import (
	"fmt"
	"time"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/buffer"
	"github.com/openmemind/memind-go/extraction"
	"github.com/openmemind/memind-go/insight"
	"github.com/openmemind/memind-go/llm"
	"github.com/openmemind/memind-go/retrieval"
	"github.com/openmemind/memind-go/store"
	tsearch "github.com/openmemind/memind-go/textsearch"
	"github.com/openmemind/memind-go/tracing"
	"github.com/openmemind/memind-go/vector"
)

type memoryImpl struct {
	memStore   store.MemoryStore
	extractor  *extraction.DefaultExtractor
	retriever  *retrieval.DefaultRetriever
	buf        *buffer.InMemoryBuffer
	vecStore   vector.MemoryVector
	textSearch tsearch.MemoryTextSearch
	llm        *llm.ChatClientRegistry
	options    memind.MemoryBuildOptions
}

func newMemory(
	memStore store.MemoryStore,
	extractor *extraction.DefaultExtractor,
	retriever *retrieval.DefaultRetriever,
	buf *buffer.InMemoryBuffer,
	vec vector.MemoryVector,
	ts tsearch.MemoryTextSearch,
	llm *llm.ChatClientRegistry,
	opts memind.MemoryBuildOptions,
) *memoryImpl {
	return &memoryImpl{
		memStore:   memStore,
		extractor:  extractor,
		retriever:  retriever,
		buf:        buf,
		vecStore:   vec,
		textSearch: ts,
		llm:        llm,
		options:    opts,
	}
}

func (m *memoryImpl) Extract(req memind.ExtractionRequest) (*memind.ExtractionResult, error) {
	if req.Config == (memind.ExtractionConfig{}) {
		req.Config = memind.DefaultExtractionConfig()
	}
	if req.Config.Timeout == 0 {
		req.Config.Timeout = 10 * time.Minute
	}
	return m.extractor.Extract(req)
}

func (m *memoryImpl) AddMessages(memoryID memind.MemoryId, messages []memind.Message, config memind.ExtractionConfig) (*memind.ExtractionResult, error) {
	if config == (memind.ExtractionConfig{}) {
		config = memind.DefaultExtractionConfig()
	}
	var lastResult *memind.ExtractionResult
	for _, msg := range messages {
		result, err := m.AddMessage(memoryID, msg, config)
		if err != nil {
			return nil, err
		}
		lastResult = result
	}
	return lastResult, nil
}

func (m *memoryImpl) AddMessage(memoryID memind.MemoryId, message memind.Message, config memind.ExtractionConfig) (*memind.ExtractionResult, error) {
	if config == (memind.ExtractionConfig{}) {
		config = memind.DefaultExtractionConfig()
	}
	return m.extractor.AddMessage(memoryID, message, config)
}

func (m *memoryImpl) Commit(memoryID memind.MemoryId, config memind.ExtractionConfig) (*memind.ExtractionResult, error) {
	if config == (memind.ExtractionConfig{}) {
		config = memind.DefaultExtractionConfig()
	}
	pending, _ := m.buf.PendingConversation().Get(memoryID)
	if len(pending) == 0 {
		return &memind.ExtractionResult{
			MemoryID: memoryID,
			Status:   memind.ExtractionSuccess,
		}, nil
	}

	var text string
	for i, msg := range pending {
		role := "user"
		if msg.Role == memind.RoleAssistant {
			role = "assistant"
		}
		text += fmt.Sprintf("[%s] %s", role, msg.ContentString())
		if i < len(pending)-1 {
			text += "\n"
		}
	}
	defer m.buf.PendingConversation().Clear(memoryID)

	return m.Extract(memind.ExtractionRequest{
		MemoryID: memoryID,
		Content:  memind.RawContent{Type: "ConversationContent", Content: text},
		Config:   config,
	})
}

func (m *memoryImpl) Retrieve(req memind.RetrievalRequest) (*memind.RetrievalResult, error) {
	if req.Config == (memind.RetrievalConfig{}) {
		if m.options.Retrieval.Common.DefaultStrategy == memind.StrategyDeep {
			req.Config = memind.DeepRetrievalConfig()
		} else {
			req.Config = memind.SimpleRetrievalConfig()
		}
	}
	return m.retriever.Retrieve(req)
}

func (m *memoryImpl) GetContext(req memind.ContextRequest) (*memind.ContextWindow, error) {
	recentMsgs, _ := m.buf.RecentConversation().GetRecent(req.MemoryID, req.RecentMessageLimit)
	if req.RecentMessageLimit <= 0 {
		recentMsgs, _ = m.buf.RecentConversation().GetRecent(req.MemoryID, 10)
	}

	window := &memind.ContextWindow{
		RecentMessages: recentMsgs,
		TotalTokens:    req.MaxTokens,
	}

	if req.IncludeMemories && len(recentMsgs) > 0 {
		var history []string
		for _, msg := range recentMsgs {
			history = append(history, msg.ContentString())
		}

		retResult, err := m.Retrieve(memind.RetrievalRequest{
			MemoryID:            req.MemoryID,
			Query:               recentMsgs[len(recentMsgs)-1].ContentString(),
			ConversationHistory: history,
		})
		if err == nil && !retResult.IsEmpty() {
			window.Memories = retResult
		}
	}

	return window, nil
}

func (m *memoryImpl) DeleteItems(memoryID memind.MemoryId, itemIDs []int64) error {
	return m.memStore.Items().DeleteItems(memoryID, itemIDs)
}

func (m *memoryImpl) DeleteInsights(memoryID memind.MemoryId, insightIDs []int64) error {
	return m.memStore.Insights().DeleteInsights(memoryID, insightIDs)
}

func (m *memoryImpl) Close() error {
	return nil
}

type MemoryBuilder struct {
	memStore   store.MemoryStore
	buf        *buffer.InMemoryBuffer
	vecStore   vector.MemoryVector
	textSearch tsearch.MemoryTextSearch
	llmReg     *llm.ChatClientRegistry
	observer   tracing.MemoryObserver
	options    memind.MemoryBuildOptions
}

func Builder() *MemoryBuilder {
	return &MemoryBuilder{
		llmReg:   llm.NewChatClientRegistry(),
		options:  memind.DefaultBuildOptions(),
		observer: &tracing.NoOpObserver{},
	}
}

func (b *MemoryBuilder) Store(s store.MemoryStore) *MemoryBuilder {
	b.memStore = s
	return b
}

func (b *MemoryBuilder) Buffer(buf *buffer.InMemoryBuffer) *MemoryBuilder {
	b.buf = buf
	return b
}

func (b *MemoryBuilder) Vector(v vector.MemoryVector) *MemoryBuilder {
	b.vecStore = v
	return b
}

func (b *MemoryBuilder) TextSearch(ts tsearch.MemoryTextSearch) *MemoryBuilder {
	b.textSearch = ts
	return b
}

func (b *MemoryBuilder) ChatClient(client llm.StructuredChatClient) *MemoryBuilder {
	b.llmReg.SetDefault(client)
	return b
}

func (b *MemoryBuilder) ChatClientForSlot(slot llm.ChatClientSlot, client llm.StructuredChatClient) *MemoryBuilder {
	b.llmReg.Register(slot, client)
	return b
}

func (b *MemoryBuilder) Observer(o tracing.MemoryObserver) *MemoryBuilder {
	b.observer = o
	return b
}

func (b *MemoryBuilder) Options(opts memind.MemoryBuildOptions) *MemoryBuilder {
	b.options = opts
	return b
}

func (b *MemoryBuilder) Build() memind.Memory {
	if b.memStore == nil {
		b.memStore = store.NewInMemoryStore()
	}
	if b.buf == nil {
		b.buf = buffer.NewInMemoryBuffer()
	}
	if b.vecStore == nil {
		b.vecStore = vector.NewInMemoryVectorStore()
	}
	if b.textSearch == nil {
		b.textSearch = tsearch.NewInMemoryBM25Search()
	}

	ext := extraction.NewExtractor(
		b.memStore,
		b.buf,
		b.vecStore,
		b.textSearch,
		b.llmReg,
		b.options.Extraction,
	)

	ret := retrieval.NewRetriever(
		b.memStore,
		b.vecStore,
		b.textSearch,
		b.llmReg,
		b.options.Retrieval,
	)

	mem := newMemory(b.memStore, ext, ret, b.buf, b.vecStore, b.textSearch, b.llmReg, b.options)

	_ = insight.NewTreeBuilder(b.memStore, memind.DefaultInsightTreeConfig())

	return mem
}
