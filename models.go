package memind

import "time"

type Role string

const (
	RoleUser      Role = "USER"
	RoleAssistant Role = "ASSISTANT"
)

type Strategy string

const (
	StrategySimple Strategy = "SIMPLE"
	StrategyDeep   Strategy = "DEEP"
)

type MemoryScope string

const (
	ScopeUser  MemoryScope = "USER"
	ScopeAgent MemoryScope = "AGENT"
)

type MemoryCategory string

const (
	CategoryProfile     MemoryCategory = "PROFILE"
	CategoryBehavior    MemoryCategory = "BEHAVIOR"
	CategoryEvent       MemoryCategory = "EVENT"
	CategoryTool        MemoryCategory = "TOOL"
	CategoryDirective   MemoryCategory = "DIRECTIVE"
	CategoryPlaybook    MemoryCategory = "PLAYBOOK"
	CategoryResolution  MemoryCategory = "RESOLUTION"
)

func UserCategories() []MemoryCategory {
	return []MemoryCategory{CategoryProfile, CategoryBehavior, CategoryEvent}
}

func AgentCategories() []MemoryCategory {
	return []MemoryCategory{CategoryTool, CategoryDirective, CategoryPlaybook, CategoryResolution}
}

type MemoryItemType string

const (
	ItemTypeFact     MemoryItemType = "FACT"
	ItemTypeForesight MemoryItemType = "FORESIGHT"
)

type InsightTier string

const (
	TierLeaf   InsightTier = "LEAF"
	TierBranch InsightTier = "BRANCH"
	TierRoot   InsightTier = "ROOT"
)

type PointType string

const (
	PointTypeSummary   PointType = "SUMMARY"
	PointTypeReasoning PointType = "REASONING"
)

type OpType string

const (
	OpAdd    OpType = "ADD"
	OpUpdate OpType = "UPDATE"
	OpDelete OpType = "DELETE"
)

type ExtractionStatus string

const (
	ExtractionSuccess         ExtractionStatus = "SUCCESS"
	ExtractionPartialSuccess  ExtractionStatus = "PARTIAL_SUCCESS"
	ExtractionFailed          ExtractionStatus = "FAILED"
)

type RetrievalStatus string

const (
	RetrievalSuccess  RetrievalStatus = "SUCCESS"
	RetrievalEmpty    RetrievalStatus = "EMPTY"
	RetrievalDegraded RetrievalStatus = "DEGRADED"
)

type MemoryId struct {
	UserID  string `json:"userId"`
	AgentID string `json:"agentId"`
}

func NewMemoryId(userId, agentId string) MemoryId {
	return MemoryId{UserID: userId, AgentID: agentId}
}

func (m MemoryId) Identifier() string {
	if m.AgentID != "" {
		return m.UserID + ":" + m.AgentID
	}
	return m.UserID
}

func (m MemoryId) HasAgent() bool {
	return m.AgentID != ""
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type Message struct {
	Role         Role           `json:"role"`
	Content      []ContentBlock `json:"content"`
	Timestamp    *time.Time     `json:"timestamp,omitempty"`
	UserName     string         `json:"userName,omitempty"`
	SourceClient string         `json:"sourceClient,omitempty"`
}

type MemoryItem struct {
	ID              int64             `json:"id"`
	MemoryID        string            `json:"memoryId"`
	Content         string            `json:"content"`
	Scope           MemoryScope       `json:"scope"`
	Category        MemoryCategory    `json:"category"`
	ContentType     string            `json:"contentType"`
	SourceClient    string            `json:"sourceClient"`
	VectorID        string            `json:"vectorId,omitempty"`
	RawDataID       string            `json:"rawDataId,omitempty"`
	ContentHash     string            `json:"contentHash"`
	OccurredAt      *time.Time        `json:"occurredAt,omitempty"`
	OccurredStart   *time.Time        `json:"occurredStart,omitempty"`
	OccurredEnd     *time.Time        `json:"occurredEnd,omitempty"`
	TimeGranularity string            `json:"timeGranularity,omitempty"`
	ObservedAt      *time.Time        `json:"observedAt,omitempty"`
	Metadata        map[string]any    `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	Type            MemoryItemType    `json:"type"`
}

type InsightPoint struct {
	PointID       string            `json:"pointId"`
	Type          PointType         `json:"type"`
	Content       string            `json:"content"`
	SourceItemIDs []string          `json:"sourceItemIds,omitempty"`
	SourceRefs    []InsightPointRef `json:"sourcePointRefs,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type InsightPointRef struct {
	InsightID int64  `json:"insightId"`
	PointID   string `json:"pointId"`
}

type MemoryInsight struct {
	ID              int64             `json:"id"`
	MemoryID        string            `json:"memoryId"`
	Type            string            `json:"type"`
	Scope           MemoryScope       `json:"scope"`
	Name            string            `json:"name"`
	Categories      []string          `json:"categories"`
	Points          []InsightPoint    `json:"points"`
	Group           string            `json:"group"`
	LastReasonedAt  *time.Time        `json:"lastReasonedAt,omitempty"`
	SummaryEmbedding []float32        `json:"summaryEmbedding,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
	Tier            InsightTier       `json:"tier"`
	ParentInsightID *int64            `json:"parentInsightId,omitempty"`
	ChildInsightIDs []int64           `json:"childInsightIds,omitempty"`
	Version         int               `json:"version"`
}

func (m MemoryInsight) PointsContent() string {
	var s string
	for i, p := range m.Points {
		if i > 0 {
			s += " "
		}
		s += p.Content
	}
	return s
}

func (m MemoryInsight) AllSourceItemIDs() []string {
	seen := make(map[string]bool)
	var ids []string
	for _, p := range m.Points {
		for _, id := range p.SourceItemIDs {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

type Segment struct {
	Type      string `json:"type"`
	ContentID string `json:"contentId"`
	Content   string `json:"content"`
}

type MemoryRawData struct {
	ID              string            `json:"id"`
	MemoryID        string            `json:"memoryId"`
	ContentType     string            `json:"contentType"`
	SourceClient    string            `json:"sourceClient"`
	ContentID       string            `json:"contentId"`
	Segment         *Segment          `json:"segment,omitempty"`
	Caption         string            `json:"caption,omitempty"`
	CaptionVectorID string            `json:"captionVectorId,omitempty"`
	Metadata        map[string]any    `json:"metadata,omitempty"`
	ResourceID      string            `json:"resourceId,omitempty"`
	MimeType        string            `json:"mimeType,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	StartTime       *time.Time        `json:"startTime,omitempty"`
	EndTime         *time.Time        `json:"endTime,omitempty"`
}

type MemoryInsightType struct {
	ID                  int64             `json:"id"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	DescriptionVectorID string            `json:"descriptionVectorId,omitempty"`
	Categories          []string          `json:"categories"`
	TargetTokens        int               `json:"targetTokens"`
	LastUpdatedAt       time.Time         `json:"lastUpdatedAt"`
	CreatedAt           time.Time         `json:"createdAt"`
	UpdatedAt           time.Time         `json:"updatedAt"`
	Scope               MemoryScope       `json:"scope"`
}

type MemoryResource struct {
	ID        string            `json:"id"`
	MemoryID  string            `json:"memoryId"`
	SourceURI string            `json:"sourceUri"`
	StorageURI string           `json:"storageUri"`
	FileName  string            `json:"fileName"`
	MimeType  string            `json:"mimeType"`
	Checksum  string            `json:"checksum"`
	SizeBytes int64             `json:"sizeBytes"`
	Metadata  map[string]any    `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

type RawContent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type ExtractionRequest struct {
	MemoryID MemoryId            `json:"memoryId"`
	Content  RawContent          `json:"content"`
	Config   ExtractionConfig    `json:"config,omitempty"`
	Metadata map[string]any      `json:"metadata,omitempty"`
}

type ExtractionConfig struct {
	EnableInsight  bool          `json:"enableInsight"`
	Scope          MemoryScope   `json:"scope"`
	EnableForesight bool         `json:"enableForesight"`
	Timeout        time.Duration `json:"-"`
	Language       string        `json:"language"`
}

func DefaultExtractionConfig() ExtractionConfig {
	return ExtractionConfig{
		EnableInsight:  true,
		Scope:          ScopeUser,
		EnableForesight: false,
		Timeout:        10 * time.Minute,
		Language:       "English",
	}
}

func (c ExtractionConfig) WithoutInsight() ExtractionConfig {
	c.EnableInsight = false
	return c
}

type ExtractionResult struct {
	MemoryID       MemoryId         `json:"memoryId"`
	RawData        RawDataResult    `json:"rawData"`
	Items          MemoryItemResult `json:"items"`
	Insights       InsightResult    `json:"insights"`
	Status         ExtractionStatus `json:"status"`
	Duration       time.Duration    `json:"-"`
	ErrorMessage   string           `json:"errorMessage,omitempty"`
	InsightPending bool             `json:"insightPending"`
}

type RawDataResult struct {
	RawDataList []*MemoryRawData `json:"rawDataList"`
	Existed     bool             `json:"existed"`
}

type MemoryItemResult struct {
	NewItems []*MemoryItem    `json:"newItems"`
	Types    []MemoryInsightType `json:"resolvedTypes"`
}

type InsightResult struct {
	Insights []*MemoryInsight `json:"insights"`
}

type RetrievedItem struct {
	ID          string     `json:"id"`
	Text        string     `json:"text"`
	VectorScore float32    `json:"vectorScore"`
	FinalScore  float64    `json:"finalScore"`
	OccurredAt  *time.Time `json:"occurredAt,omitempty"`
}

type RetrievedInsight struct {
	ID   string     `json:"id"`
	Text string     `json:"text"`
	Tier InsightTier `json:"tier,omitempty"`
}

type RetrievedRawData struct {
	RawDataID string   `json:"rawDataId"`
	Caption   string   `json:"caption,omitempty"`
	MaxScore  float64  `json:"maxScore"`
	ItemIDs   []string `json:"itemIds,omitempty"`
}

type ScoredResult struct {
	SourceType  string     `json:"sourceType"`
	SourceID    string     `json:"sourceId"`
	Text        string     `json:"text"`
	VectorScore float32    `json:"vectorScore"`
	FinalScore  float64    `json:"finalScore"`
	OccurredAt  *time.Time `json:"occurredAt,omitempty"`
}

func (s ScoredResult) DedupKey() string {
	return s.SourceType + ":" + s.SourceID
}

type RetrievalRequest struct {
	MemoryID           MemoryId            `json:"memoryId"`
	Query              string              `json:"query"`
	ConversationHistory []string           `json:"conversationHistory,omitempty"`
	Config             RetrievalConfig     `json:"config,omitempty"`
	Metadata           map[string]any      `json:"metadata,omitempty"`
	Scope              *MemoryScope        `json:"scope,omitempty"`
	Categories         []MemoryCategory    `json:"categories,omitempty"`
}

type RetrievalResult struct {
	Items     []ScoredResult   `json:"items"`
	Insights  []RetrievedInsight `json:"insights"`
	RawData   []RetrievedRawData `json:"rawData"`
	Evidences []string         `json:"evidences"`
	Strategy  string           `json:"strategy"`
	Query     string           `json:"query"`
	Status    RetrievalStatus  `json:"status"`
}

func (r RetrievalResult) IsEmpty() bool {
	return len(r.Items) == 0 && len(r.Insights) == 0 && len(r.RawData) == 0
}

type ContextWindow struct {
	RecentMessages []Message        `json:"recentMessages"`
	Memories       *RetrievalResult `json:"memories,omitempty"`
	TotalTokens    int              `json:"totalTokens"`
}

func (c ContextWindow) HasMemories() bool {
	return c.Memories != nil && !c.Memories.IsEmpty()
}

type ContextRequest struct {
	MemoryID          MemoryId `json:"memoryId"`
	MaxTokens         int      `json:"maxTokens"`
	IncludeMemories   bool     `json:"includeMemories"`
	Strategy          Strategy `json:"strategy,omitempty"`
	RecentMessageLimit int     `json:"recentMessageLimit,omitempty"`
}

type OperationAccepted struct {
	OperationID string `json:"operationId"`
	Status      string `json:"status"`
	Mode        string `json:"mode"`
}

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type AddMessageRequest struct {
	MemoryID    MemoryId `json:"memoryId"`
	Message     Message  `json:"message"`
	SourceClient string  `json:"sourceClient,omitempty"`
}

type AddMessageResponse struct {
	Triggered bool              `json:"triggered"`
	Result    *ExtractionResult `json:"result,omitempty"`
}
