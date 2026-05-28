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

// memoryImpl - Memory 接口的默认实现，组合所有子模块
type memoryImpl struct {
	memStore    store.MemoryStore
	extractor   *extraction.DefaultExtractor
	retriever   *retrieval.DefaultRetriever
	buf         *buffer.InMemoryBuffer
	vecStore    vector.MemoryVector
	textSearch  tsearch.MemoryTextSearch
	llm         *llm.ChatClientRegistry
	reorganizer *insight.TreeReorganizer
	options     memind.MemoryBuildOptions
}

// newMemory - 创建 memoryImpl 实例（包内私有）
func newMemory(
	memStore store.MemoryStore,
	extractor *extraction.DefaultExtractor,
	retriever *retrieval.DefaultRetriever,
	buf *buffer.InMemoryBuffer,
	vec vector.MemoryVector,
	ts tsearch.MemoryTextSearch,
	llm *llm.ChatClientRegistry,
	reorganizer *insight.TreeReorganizer,
	opts memind.MemoryBuildOptions,
) *memoryImpl {
	return &memoryImpl{
		memStore:    memStore,
		extractor:   extractor,
		retriever:   retriever,
		buf:         buf,
		vecStore:    vec,
		textSearch:  ts,
		llm:         llm,
		reorganizer: reorganizer,
		options:     opts,
	}
}

// Extract - 直接提取记忆：原始内容 → 原始数据 → 条目 → 洞察 → 树晋升
func (m *memoryImpl) Extract(req memind.ExtractionRequest) (*memind.ExtractionResult, error) {
	if req.Config == (memind.ExtractionConfig{}) {
		req.Config = memind.DefaultExtractionConfig()
	}
	if req.Config.Timeout == 0 {
		req.Config.Timeout = 10 * time.Minute
	}

	result, err := m.extractor.Extract(req)
	if err != nil {
		return nil, err
	}

	// 提取完成后按洞察类型执行树重组织
	if len(result.Insights.Insights) > 0 && m.reorganizer != nil {
		cfg := memind.DefaultInsightTreeConfig()
		language := req.Config.Language
		if language == "" {
			language = "English"
		}
		for _, t := range result.Items.Types {
			leafs := result.Insights.ByType[t.Name]
			if len(leafs) == 0 {
				continue
			}
			_ = m.reorganizer.OnLeafsUpdated(req.MemoryID, t.Name, t, leafs, cfg, language)
		}
	}

	return result, nil
}

// AddMessages - 批量添加消息到缓冲区
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

// AddMessage - 添加单条消息到缓冲区，达到批处理阈值时自动触发提取
func (m *memoryImpl) AddMessage(memoryID memind.MemoryId, message memind.Message, config memind.ExtractionConfig) (*memind.ExtractionResult, error) {
	if config == (memind.ExtractionConfig{}) {
		config = memind.DefaultExtractionConfig()
	}
	return m.extractor.AddMessage(memoryID, message, config)
}

// Commit - 强制刷新缓冲区，将暂存消息发送到提取管线
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

	// 将消息列表拼接为对话文本
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

// Retrieve - 执行记忆检索：准入检查 → 意图路由 → 策略分发
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

// GetContext - 构建 LLM 上下文窗口：近期消息 + 检索到的记忆
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

// DeleteItems - 删除指定 ID 的记忆条目
func (m *memoryImpl) DeleteItems(memoryID memind.MemoryId, itemIDs []int64) error {
	return m.memStore.Items().DeleteItems(memoryID, itemIDs)
}

// DeleteInsights - 删除指定 ID 的洞察
func (m *memoryImpl) DeleteInsights(memoryID memind.MemoryId, insightIDs []int64) error {
	return m.memStore.Insights().DeleteInsights(memoryID, insightIDs)
}

// Close - 释放资源（内存模式下无操作）
func (m *memoryImpl) Close() error {
	return nil
}

// ---------- Builder ----------

// MemoryBuilder - Memory 实例的构建器，采用依赖注入方式组装子模块
type MemoryBuilder struct {
	memStore        store.MemoryStore
	buf             *buffer.InMemoryBuffer
	vecStore        vector.MemoryVector
	textSearch      tsearch.MemoryTextSearch
	llmReg          *llm.ChatClientRegistry
	embeddingClient llm.EmbeddingClient
	observer        tracing.MemoryObserver
	options         memind.MemoryBuildOptions
}

// Builder - 创建新的 MemoryBuilder 实例
func Builder() *MemoryBuilder {
	return &MemoryBuilder{
		llmReg:   llm.NewChatClientRegistry(),
		options:  memind.DefaultBuildOptions(),
		observer: &tracing.NoOpObserver{},
	}
}

// Store - 设置持久化存储实现（默认 InMemoryStore）
func (b *MemoryBuilder) Store(s store.MemoryStore) *MemoryBuilder {
	b.memStore = s
	return b
}

// Buffer - 设置消息缓冲区实现（默认 InMemoryBuffer）
func (b *MemoryBuilder) Buffer(buf *buffer.InMemoryBuffer) *MemoryBuilder {
	b.buf = buf
	return b
}

// Vector - 设置向量存储实现（默认 InMemoryVectorStore）
func (b *MemoryBuilder) Vector(v vector.MemoryVector) *MemoryBuilder {
	b.vecStore = v
	return b
}

// TextSearch - 设置全文搜索实现（默认 InMemoryBM25Search）
func (b *MemoryBuilder) TextSearch(ts tsearch.MemoryTextSearch) *MemoryBuilder {
	b.textSearch = ts
	return b
}

// EmbeddingClient - 设置外部嵌入客户端，替代默认哈希嵌入
// 注册后 InMemoryVectorStore 的 Embed() 会优先调用此客户端
func (b *MemoryBuilder) EmbeddingClient(client llm.EmbeddingClient) *MemoryBuilder {
	b.embeddingClient = client
	return b
}

// ChatClient - 设置默认 LLM 聊天客户端（所有槽位共用）
func (b *MemoryBuilder) ChatClient(client llm.StructuredChatClient) *MemoryBuilder {
	b.llmReg.SetDefault(client)
	return b
}

// ChatClientForSlot - 为指定槽位注册专用的 LLM 客户端
func (b *MemoryBuilder) ChatClientForSlot(slot llm.ChatClientSlot, client llm.StructuredChatClient) *MemoryBuilder {
	b.llmReg.Register(slot, client)
	return b
}

// Observer - 设置可观测性收集器
func (b *MemoryBuilder) Observer(o tracing.MemoryObserver) *MemoryBuilder {
	b.observer = o
	return b
}

// Options - 设置构建选项
func (b *MemoryBuilder) Options(opts memind.MemoryBuildOptions) *MemoryBuilder {
	b.options = opts
	return b
}

// Build - 组装所有模块，返回完整的 Memory 实例
func (b *MemoryBuilder) Build() memind.Memory {
	// 未显式设置时使用内存默认实现
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

	// 若设置了嵌入客户端，注入到向量存储
	if b.embeddingClient != nil {
		if vs, ok := b.vecStore.(*vector.InMemoryVectorStore); ok {
			vs.SetEmbeddingClient(b.embeddingClient)
		}
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

	gen := insight.NewLlmInsightGenerator(b.llmReg)
	bubble := insight.NewBubbleTracker()
	graph := insight.NewNoOpGraphAssistant()
	reorganizer := insight.NewTreeReorganizer(b.memStore, gen, b.vecStore, bubble, graph)

	mem := newMemory(b.memStore, ext, ret, b.buf, b.vecStore, b.textSearch, b.llmReg, reorganizer, b.options)

	return mem
}
