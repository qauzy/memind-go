package llm

type ChatRole string

const (
	RoleSystem    ChatRole = "SYSTEM"
	RoleUser      ChatRole = "USER"
	RoleAssistant ChatRole = "ASSISTANT"
)

type ChatMessage struct {
	Role    ChatRole `json:"role"`
	Content string   `json:"content"`
}

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

type StructuredChatClient interface {
	Call(messages []ChatMessage) (string, error)
	CallStructured(messages []ChatMessage, responseType any) error
}

type NoOpChatClient struct{}

func (c *NoOpChatClient) Call(messages []ChatMessage) (string, error) {
	return "", nil
}
func (c *NoOpChatClient) CallStructured(messages []ChatMessage, responseType any) error {
	return nil
}

type EmbeddingClient interface {
	Embed(text string) ([]float32, error)
	EmbedAll(texts []string) ([][]float32, error)
}

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

type ChatClientRegistry struct {
	clients map[ChatClientSlot]StructuredChatClient
	defaultClient StructuredChatClient
}

func NewChatClientRegistry() *ChatClientRegistry {
	return &ChatClientRegistry{
		clients:       make(map[ChatClientSlot]StructuredChatClient),
		defaultClient: &NoOpChatClient{},
	}
}

func (r *ChatClientRegistry) Register(slot ChatClientSlot, client StructuredChatClient) {
	r.clients[slot] = client
}

func (r *ChatClientRegistry) SetDefault(client StructuredChatClient) {
	r.defaultClient = client
}

func (r *ChatClientRegistry) Resolve(slot ChatClientSlot) StructuredChatClient {
	if c, ok := r.clients[slot]; ok {
		return c
	}
	return r.defaultClient
}
