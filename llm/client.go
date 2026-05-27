package llm

// ChatRole - LLM 对话角色
type ChatRole string

const (
	RoleSystem    ChatRole = "SYSTEM"
	RoleUser      ChatRole = "USER"
	RoleAssistant ChatRole = "ASSISTANT"
)

// ChatMessage - LLM 单条消息
type ChatMessage struct {
	Role    ChatRole `json:"role"`
	Content string   `json:"content"`
}

// ChatClientSlot - LLM 调用槽位，标识管线中的每个调用点
type ChatClientSlot string

const (
	SlotItemExtraction      ChatClientSlot = "ITEM_EXTRACTION"
	SlotConversationChunker ChatClientSlot = "CONVERSATION_CHUNKER"
	SlotCaptionGenerator    ChatClientSlot = "CAPTION_GENERATOR"
	SlotCommitDetector      ChatClientSlot = "CONTEXT_COMMIT_DETECTOR"
	SlotInsightGenerator    ChatClientSlot = "INSIGHT_GENERATOR"
	SlotInsightClassifier   ChatClientSlot = "INSIGHT_GROUP_CLASSIFIER"
	SlotQueryExpander       ChatClientSlot = "QUERY_EXPANDER"
	SlotLongQueryCondenser  ChatClientSlot = "LONG_QUERY_CONDENSER"
	SlotSufficiencyGate     ChatClientSlot = "SUFFICIENCY_GATE"
	SlotTypeRouter          ChatClientSlot = "INSIGHT_TYPE_ROUTER"
)

// StructuredChatClient - 结构化 LLM 客户端接口
type StructuredChatClient interface {
	Call(messages []ChatMessage) (string, error)
	CallStructured(messages []ChatMessage, responseType any) error
}

// NoOpChatClient - 空操作 LLM 客户端，当未配置 LLM 时使用
type NoOpChatClient struct{}

func (c *NoOpChatClient) Call(messages []ChatMessage) (string, error)                   { return "", nil }
func (c *NoOpChatClient) CallStructured(messages []ChatMessage, responseType any) error { return nil }

// EmbeddingClient - 文本嵌入客户端接口
type EmbeddingClient interface {
	Embed(text string) ([]float32, error)
	EmbedAll(texts []string) ([][]float32, error)
}

// NoOpEmbeddingClient - 空操作嵌入客户端
type NoOpEmbeddingClient struct{}

func (c *NoOpEmbeddingClient) Embed(text string) ([]float32, error) {
	return make([]float32, 128), nil
}
func (c *NoOpEmbeddingClient) EmbedAll(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = make([]float32, 128)
	}
	return result, nil
}

// ChatClientRegistry - LLM 客户端注册表，按槽位分发客户端
type ChatClientRegistry struct {
	clients       map[ChatClientSlot]StructuredChatClient
	defaultClient StructuredChatClient
}

// NewChatClientRegistry - 创建注册表，默认使用 NoOpClient
func NewChatClientRegistry() *ChatClientRegistry {
	return &ChatClientRegistry{
		clients:       make(map[ChatClientSlot]StructuredChatClient),
		defaultClient: &NoOpChatClient{},
	}
}

// Register - 为指定槽位注册专用客户端
func (r *ChatClientRegistry) Register(slot ChatClientSlot, client StructuredChatClient) {
	r.clients[slot] = client
}

// SetDefault - 设置默认客户端（所有未注册槽位使用此客户端）
func (r *ChatClientRegistry) SetDefault(client StructuredChatClient) {
	r.defaultClient = client
}

// Resolve - 解析指定槽位的客户端：优先返回专用客户端，否则返回默认
func (r *ChatClientRegistry) Resolve(slot ChatClientSlot) StructuredChatClient {
	if c, ok := r.clients[slot]; ok {
		return c
	}
	return r.defaultClient
}
