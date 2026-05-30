package memind

import "time"

// Role - 对话参与角色
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Strategy - 检索策略类型
type Strategy string

const (
	StrategySimple Strategy = "simple"
	StrategyDeep   Strategy = "deep"
)

// MemoryScope - 记忆作用域：USER=用户侧 / AGENT=Agent 侧
type MemoryScope string

const (
	ScopeUser  MemoryScope = "user"
	ScopeAgent MemoryScope = "agent"
)

// MemoryCategory - 记忆分类
type MemoryCategory string

const (
	CategoryProfile    MemoryCategory = "profile"
	CategoryBehavior   MemoryCategory = "behavior"
	CategoryEvent      MemoryCategory = "event"
	CategoryTool       MemoryCategory = "tool"
	CategoryDirective  MemoryCategory = "directive"
	CategoryPlaybook   MemoryCategory = "playbook"
	CategoryResolution MemoryCategory = "resolution"
)

// UserCategories - 返回 USER 作用域下的记忆分类列表
func UserCategories() []MemoryCategory {
	return []MemoryCategory{CategoryProfile, CategoryBehavior, CategoryEvent}
}

// AgentCategories - 返回 AGENT 作用域下的记忆分类列表
func AgentCategories() []MemoryCategory {
	return []MemoryCategory{CategoryTool, CategoryDirective, CategoryPlaybook, CategoryResolution}
}

// MemoryItemType - 记忆条目类型：事实 / 预判
type MemoryItemType string

const (
	ItemTypeFact      MemoryItemType = "fact"
	ItemTypeForesight MemoryItemType = "foresight"
)

// InsightTier - 洞察树层级
type InsightTier string

const (
	TierLeaf   InsightTier = "leaf"
	TierBranch InsightTier = "branch"
	TierRoot   InsightTier = "root"
)

// InsightAnalysisMode - 洞察类型分析模式
type InsightAnalysisMode string

const (
	AnalysisModeBranch InsightAnalysisMode = "branch"
	AnalysisModeRoot   InsightAnalysisMode = "root"
)

// PointType - 洞察点类型
type PointType string

const (
	PointTypeSummary   PointType = "summary"
	PointTypeReasoning PointType = "reasoning"
)

// OpType - 洞察点操作类型
type OpType string

const (
	OpAdd    OpType = "add"
	OpUpdate OpType = "update"
	OpDelete OpType = "delete"
)

// ExtractionStatus - 提取结果状态
type ExtractionStatus string

const (
	ExtractionSuccess        ExtractionStatus = "success"
	ExtractionPartialSuccess ExtractionStatus = "partial_success"
	ExtractionFailed         ExtractionStatus = "failed"
)

// RetrievalStatus - 检索结果状态
type RetrievalStatus string

const (
	RetrievalSuccess  RetrievalStatus = "success"
	RetrievalEmpty    RetrievalStatus = "empty"
	RetrievalDegraded RetrievalStatus = "degraded"
)

// MemoryId - 记忆唯一标识，由 userId 和可选 agentId 组成
type MemoryId struct {
	UserID  string `json:"userId"`
	AgentID string `json:"agentId"`
}

// NewMemoryId - 创建 MemoryId
func NewMemoryId(userId, agentId string) MemoryId {
	return MemoryId{UserID: userId, AgentID: agentId}
}

// Identifier - 返回持久化层使用的唯一字符串标识
func (m MemoryId) Identifier() string {
	if m.AgentID != "" {
		return m.UserID + ":" + m.AgentID
	}
	return m.UserID
}

// HasAgent - 判断是否有 agentId
func (m MemoryId) HasAgent() bool {
	return m.AgentID != ""
}

// ContentBlock - 多模态内容块（目前仅 text 类型）
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Message - 单条对话消息
type Message struct {
	Role         Role           `json:"role"`
	Content      []ContentBlock `json:"content"`
	Timestamp    *time.Time     `json:"timestamp,omitempty"`
	UserName     string         `json:"userName,omitempty"`
	SourceClient string         `json:"sourceClient,omitempty"`
}

// ContentString - 将所有 content block 拼接为纯文本
func (m Message) ContentString() string {
	var s string
	for _, b := range m.Content {
		s += b.Text + " "
	}
	return s
}

// MemoryItem - 单条记忆条目（事实或预判）
type MemoryItem struct {
	ID              int64          `json:"id"`
	MemoryID        string         `json:"memoryId"`
	Content         string         `json:"content"`
	Scope           MemoryScope    `json:"scope"`
	Category        MemoryCategory `json:"category"`
	ContentType     string         `json:"contentType"`
	SourceClient    string         `json:"sourceClient"`
	VectorID        string         `json:"vectorId,omitempty"`
	RawDataID       string         `json:"rawDataId,omitempty"`
	ContentHash     string         `json:"contentHash"`
	OccurredAt      *time.Time     `json:"occurredAt,omitempty"`
	OccurredStart   *time.Time     `json:"occurredStart,omitempty"`
	OccurredEnd     *time.Time     `json:"occurredEnd,omitempty"`
	TimeGranularity string         `json:"timeGranularity,omitempty"`
	ObservedAt      *time.Time     `json:"observedAt,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	Type            MemoryItemType `json:"type"`
}

// InsightPoint - 洞察树中的一个节点内容点
type InsightPoint struct {
	PointID       string            `json:"pointId"`
	Type          PointType         `json:"type"`
	Content       string            `json:"content"`
	SourceItemIDs []string          `json:"sourceItemIds,omitempty"`
	SourceRefs    []InsightPointRef `json:"sourcePointRefs,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// InsightPointRef - 洞察点引用，指向其他洞察中的点
type InsightPointRef struct {
	InsightID int64  `json:"insightId"`
	PointID   string `json:"pointId"`
}

// MemoryInsight - 洞察树节点（Leaf / Branch / Root）
type MemoryInsight struct {
	ID               int64          `json:"id"`
	MemoryID         string         `json:"memoryId"`
	Type             string         `json:"type"`
	Scope            MemoryScope    `json:"scope"`
	Name             string         `json:"name"`
	Categories       []string       `json:"categories"`
	Points           []InsightPoint `json:"points"`
	Group            string         `json:"group"`
	LastReasonedAt   *time.Time     `json:"lastReasonedAt,omitempty"`
	SummaryEmbedding []float32      `json:"summaryEmbedding,omitempty"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	Tier             InsightTier    `json:"tier"`
	ParentInsightID  *int64         `json:"parentInsightId,omitempty"`
	ChildInsightIDs  []int64        `json:"childInsightIds,omitempty"`
	Version          int            `json:"version"`
}

// PointsContent - 将洞察中所有点的内容拼接为纯文本，用于向量化
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

// AllSourceItemIDs - 收集所有点引用的 sourceItemId，去重后返回
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

// Segment - 原始内容分段描述
type Segment struct {
	Type      string `json:"type"`
	ContentID string `json:"contentId"`
	Content   string `json:"content"`
}

// MemoryRawData - 原始数据记录，是提取的起点
type MemoryRawData struct {
	ID              string         `json:"id"`
	MemoryID        string         `json:"memoryId"`
	ContentType     string         `json:"contentType"`
	SourceClient    string         `json:"sourceClient"`
	ContentID       string         `json:"contentId"`
	Segment         *Segment       `json:"segment,omitempty"`
	Caption         string         `json:"caption,omitempty"`
	CaptionVectorID string         `json:"captionVectorId,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	ResourceID      string         `json:"resourceId,omitempty"`
	MimeType        string         `json:"mimeType,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	StartTime       *time.Time     `json:"startTime,omitempty"`
	EndTime         *time.Time     `json:"endTime,omitempty"`
}

// InsightPointOp - 洞察点操作指令
type InsightPointOp struct {
	Op            OpType            `json:"op"`
	PointID       string            `json:"pointId"`
	Type          *PointType        `json:"type,omitempty"`
	Content       string            `json:"content,omitempty"`
	SourceItemIDs []string          `json:"sourceItemIds,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// InsightPointOpsResponse - LLM 返回的 Point 操作列表
type InsightPointOpsResponse struct {
	Operations []InsightPointOp `json:"operations"`
}

// InsightPointGenerateResponse - LLM 返回的全量 Point 列表
type InsightPointGenerateResponse struct {
	Points []InsightPoint `json:"points"`
}

// MemoryInsightType - 洞察类型的元定义
type MemoryInsightType struct {
	ID                  int64               `json:"id"`
	Name                string              `json:"name"`
	Description         string              `json:"description"`
	DescriptionVectorID string              `json:"descriptionVectorId,omitempty"`
	Categories          []string            `json:"categories"`
	TargetTokens        int                 `json:"targetTokens"`
	AnalysisMode        InsightAnalysisMode `json:"analysisMode"`
	LastUpdatedAt       time.Time           `json:"lastUpdatedAt"`
	CreatedAt           time.Time           `json:"createdAt"`
	UpdatedAt           time.Time           `json:"updatedAt"`
	Scope               MemoryScope         `json:"scope"`
}

// MemoryResource - 外部资源文件记录
type MemoryResource struct {
	ID         string         `json:"id"`
	MemoryID   string         `json:"memoryId"`
	SourceURI  string         `json:"sourceUri"`
	StorageURI string         `json:"storageUri"`
	FileName   string         `json:"fileName"`
	MimeType   string         `json:"mimeType"`
	Checksum   string         `json:"checksum"`
	SizeBytes  int64          `json:"sizeBytes"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
}

// RawContent - 提取请求中的原始内容载体
type RawContent struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// ExtractionRequest - 提取请求
type ExtractionRequest struct {
	MemoryID MemoryId         `json:"memoryId"`
	Content  RawContent       `json:"content"`
	Config   ExtractionConfig `json:"config,omitempty"`
	Metadata map[string]any   `json:"metadata,omitempty"`
}

// ExtractionConfig - 提取配置参数
type ExtractionConfig struct {
	EnableInsight   bool          `json:"enableInsight"`
	Scope           MemoryScope   `json:"scope"`
	EnableForesight bool          `json:"enableForesight"`
	Timeout         time.Duration `json:"-"`
	Language        string        `json:"language"`
}

// DefaultExtractionConfig - 返回默认提取配置
func DefaultExtractionConfig() ExtractionConfig {
	return ExtractionConfig{
		EnableInsight:   true,
		Scope:           ScopeUser,
		EnableForesight: false,
		Timeout:         10 * time.Minute,
		Language:        "English",
	}
}

// WithoutInsight - 关闭洞察提取，返回副本
func (c ExtractionConfig) WithoutInsight() ExtractionConfig {
	c.EnableInsight = false
	return c
}

// ExtractionResult - 提取管线执行结果
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

// RawDataResult - 原始数据提取结果
type RawDataResult struct {
	RawDataList []*MemoryRawData `json:"rawDataList"`
	Existed     bool             `json:"existed"`
}

// MemoryItemResult - 记忆条目提取结果
type MemoryItemResult struct {
	NewItems []*MemoryItem       `json:"newItems"`
	Types    []MemoryInsightType `json:"resolvedTypes"`
}

// InsightResult - 洞察提取结果
type InsightResult struct {
	Insights []*MemoryInsight            `json:"insights"`
	ByType   map[string][]*MemoryInsight `json:"-"`
}

// RetrievedItem - 检索返回的单条记忆条目
type RetrievedItem struct {
	ID          string     `json:"id"`
	Text        string     `json:"text"`
	VectorScore float32    `json:"vectorScore"`
	FinalScore  float64    `json:"finalScore"`
	OccurredAt  *time.Time `json:"occurredAt,omitempty"`
}

// RetrievedInsight - 检索返回的单条洞察
type RetrievedInsight struct {
	ID          string      `json:"id"`
	Text        string      `json:"text"`
	VectorScore float32     `json:"vectorScore"`
	FinalScore  float64     `json:"finalScore"`
	Tier        InsightTier `json:"tier,omitempty"`
}

// RetrievedRawData - 检索返回的原始数据摘要
type RetrievedRawData struct {
	RawDataID string   `json:"rawDataId"`
	Caption   string   `json:"caption,omitempty"`
	MaxScore  float64  `json:"maxScore"`
	ItemIDs   []string `json:"itemIds,omitempty"`
}

// ScoredResult - 带分数的检索中间结果，用于 RRF 合并
type ScoredResult struct {
	SourceType  string     `json:"sourceType"`
	SourceID    string     `json:"sourceId"`
	Text        string     `json:"text"`
	VectorScore float32    `json:"vectorScore"`
	FinalScore  float64    `json:"finalScore"`
	OccurredAt  *time.Time `json:"occurredAt,omitempty"`
}

// DedupKey - 用于 RRF 合并时去重
func (s ScoredResult) DedupKey() string {
	return s.SourceType + ":" + s.SourceID
}

// RetrievalRequest - 检索请求
type RetrievalRequest struct {
	MemoryID            MemoryId         `json:"memoryId"`
	Query               string           `json:"query"`
	ConversationHistory []string         `json:"conversationHistory,omitempty"`
	Config              RetrievalConfig  `json:"config,omitempty"`
	Metadata            map[string]any   `json:"metadata,omitempty"`
	Scope               *MemoryScope     `json:"scope,omitempty"`
	Categories          []MemoryCategory `json:"categories,omitempty"`
}

// RetrievalResult - 检索结果
type RetrievalResult struct {
	Items     []ScoredResult     `json:"items"`
	Insights  []RetrievedInsight `json:"insights"`
	RawData   []RetrievedRawData `json:"rawData"`
	Evidences []string           `json:"evidences"`
	Strategy  string             `json:"strategy"`
	Query     string             `json:"query"`
	Status    RetrievalStatus    `json:"status"`
}

// IsEmpty - 判断检索结果是否为空
func (r RetrievalResult) IsEmpty() bool {
	return len(r.Items) == 0 && len(r.Insights) == 0 && len(r.RawData) == 0
}

// ContextWindow - LLM 上下文窗口，包含近期消息和检索到的记忆
type ContextWindow struct {
	RecentMessages []Message        `json:"recentMessages"`
	Memories       *RetrievalResult `json:"memories,omitempty"`
	TotalTokens    int              `json:"totalTokens"`
}

// HasMemories - 判断上下文窗口中是否包含检索记忆
func (c ContextWindow) HasMemories() bool {
	return c.Memories != nil && !c.Memories.IsEmpty()
}

// ContextRequest - 上下文窗口构建请求
type ContextRequest struct {
	MemoryID           MemoryId `json:"memoryId"`
	MaxTokens          int      `json:"maxTokens"`
	IncludeMemories    bool     `json:"includeMemories"`
	Strategy           Strategy `json:"strategy,omitempty"`
	RecentMessageLimit int      `json:"recentMessageLimit,omitempty"`
}

// OperationAccepted - 异步操作接受响应
type OperationAccepted struct {
	OperationID string `json:"operationId"`
	Status      string `json:"status"`
	Mode        string `json:"mode"`
}

// HealthResponse - 健康检查响应
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// AddMessageRequest - 添加消息请求
type AddMessageRequest struct {
	MemoryID     MemoryId `json:"memoryId"`
	Message      Message  `json:"message"`
	SourceClient string   `json:"sourceClient,omitempty"`
}

// AddMessageResponse - 添加消息响应
type AddMessageResponse struct {
	Triggered bool              `json:"triggered"`
	Result    *ExtractionResult `json:"result,omitempty"`
}
