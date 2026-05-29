package memind

import "time"

// ---------- 检索配置 ----------

// RetrievalConfig - 检索策略完整配置
type RetrievalConfig struct {
	Tier1          TierConfig     `json:"tier1"`
	Tier2          TierConfig     `json:"tier2"`
	Tier3          TierConfig     `json:"tier3"`
	Rerank         RerankConfig   `json:"rerank"`
	Scoring        ScoringConfig  `json:"scoring"`
	Timeout        time.Duration  `json:"-"`
	EnableCache    bool           `json:"enableCache"`
	StrategyConfig StrategyConfig `json:"strategyConfig"`
}

// TierConfig - 单层检索配置（Insight Tier 1 / Item Tier 2 / RawData Tier 3）
type TierConfig struct {
	Enabled    bool             `json:"enabled"`
	TopK       int              `json:"topK"`
	MinScore   float64          `json:"minScore"`
	Truncation TruncationConfig `json:"truncation"`
}

// EnablesTier - 创建启用状态的层级配置
func EnablesTier(topK int) TierConfig {
	return TierConfig{Enabled: true, TopK: topK, MinScore: 0.0}
}

// DisabledTier - 创建禁用状态的层级配置
func DisabledTier() TierConfig {
	return TierConfig{Enabled: false}
}

// TruncationConfig - 自适应截断配置
type TruncationConfig struct {
	Enabled      bool `json:"enabled"`
	MaxItems     int  `json:"maxItems"`
	TargetTokens int  `json:"targetTokens"`
}

// RerankConfig - LLM 重排序配置
type RerankConfig struct {
	Enabled            bool    `json:"enabled"`
	BlendWithRetrieval bool    `json:"blendWithRetrieval"`
	Top3Weight         float64 `json:"top3Weight"`
	Top10Weight        float64 `json:"top10Weight"`
	OtherWeight        float64 `json:"otherWeight"`
	TopK               int     `json:"topK"`
}

// PureRerank - 创建纯重排序配置（不混合原始检索分数）
func PureRerank(topK int) RerankConfig {
	return RerankConfig{Enabled: true, BlendWithRetrieval: false, TopK: topK}
}

// DisabledRerank - 创建禁用重排序配置
func DisabledRerank() RerankConfig {
	return RerankConfig{Enabled: false, TopK: 0}
}

// ScoringConfig - 检索打分全参数
type ScoringConfig struct {
	Fusion                 FusionConfig        `json:"fusion"`
	TimeDecay              TimeDecayConfig     `json:"timeDecay"`
	Recency                RecencyConfig       `json:"recency"`
	PositionBonus          PositionBonusConfig `json:"positionBonus"`
	KeywordSearch          KeywordSearchConfig `json:"keywordSearch"`
	QueryWeight            QueryWeightConfig   `json:"queryWeight"`
	CandidateMultiplier    int                 `json:"candidateMultiplier"`
	RerankCandidateLimit   int                 `json:"rerankCandidateLimit"`
	RawDataKeyInfoMaxLines int                 `json:"rawDataKeyInfoMaxLines"`
	InsightLlmThreshold    int                 `json:"insightLlmThreshold"`
}

// FusionConfig - RRF 融合参数
type FusionConfig struct {
	K             int     `json:"k"`
	VectorWeight  float64 `json:"vectorWeight"`
	KeywordWeight float64 `json:"keywordWeight"`
}

// TimeDecayConfig - 时间衰减参数
type TimeDecayConfig struct {
	Rate              float64 `json:"rate"`
	Floor             float64 `json:"floor"`
	OutOfRangePenalty float64 `json:"outOfRangePenalty"`
}

// RecencyConfig - 近期性加成参数
type RecencyConfig struct {
	Rate  float64 `json:"rate"`
	Floor float64 `json:"floor"`
}

// PositionBonusConfig - 排序位置加分
type PositionBonusConfig struct {
	Top1 float64 `json:"top1"`
	Top3 float64 `json:"top3"`
}

// KeywordSearchConfig - 关键词探测配置
type KeywordSearchConfig struct {
	ProbeTopK            int     `json:"probeTopK"`
	StrongSignalMinScore float64 `json:"strongSignalMinScore"`
	StrongSignalMinGap   float64 `json:"strongSignalMinGap"`
}

// QueryWeightConfig - 原始查询与扩展查询的权重
type QueryWeightConfig struct {
	OriginalWeight float64 `json:"originalWeight"`
	ExpandedWeight float64 `json:"expandedWeight"`
}

// StrategyConfig - 检索策略容器
type StrategyConfig struct {
	Simple SimpleStrategyConfig `json:"simple"`
	Deep   DeepStrategyConfig   `json:"deep"`
}

// SimpleStrategyConfig - Simple 策略配置（当前无特定参数）
type SimpleStrategyConfig struct{}

// DeepStrategyConfig - Deep 策略配置（当前无特定参数）
type DeepStrategyConfig struct{}

// defaultScoringConfig - 返回默认打分参数
func defaultScoringConfig() ScoringConfig {
	return ScoringConfig{
		Fusion:                 FusionConfig{K: 60, VectorWeight: 1.5, KeywordWeight: 1.0},
		TimeDecay:              TimeDecayConfig{Rate: 0.023, Floor: 0.3, OutOfRangePenalty: 0.5},
		Recency:                RecencyConfig{Rate: 0.0019, Floor: 0.7},
		PositionBonus:          PositionBonusConfig{Top1: 0.05, Top3: 0.02},
		KeywordSearch:          KeywordSearchConfig{ProbeTopK: 10, StrongSignalMinScore: 0.85, StrongSignalMinGap: 0.15},
		QueryWeight:            QueryWeightConfig{OriginalWeight: 2.0, ExpandedWeight: 1.0},
		CandidateMultiplier:    3,
		RerankCandidateLimit:   50,
		RawDataKeyInfoMaxLines: 20,
		InsightLlmThreshold:    5,
	}
}

// SimpleRetrievalConfig - 返回 Simple 策略的完整默认配置
func SimpleRetrievalConfig() RetrievalConfig {
	return RetrievalConfig{
		Tier1:   TierConfig{Enabled: true, TopK: 5, MinScore: 0.3},
		Tier2:   TierConfig{Enabled: true, TopK: 15, MinScore: 0.0, Truncation: TruncationConfig{Enabled: true, MaxItems: 30, TargetTokens: 4000}},
		Tier3:   TierConfig{Enabled: true, TopK: 5, MinScore: 0.0},
		Rerank:  DisabledRerank(),
		Scoring: defaultScoringConfig(),
		Timeout: 10 * time.Second,
	}
}

// DeepRetrievalConfig - 返回 Deep 策略的完整默认配置
func DeepRetrievalConfig() RetrievalConfig {
	return RetrievalConfig{
		Tier1:   TierConfig{Enabled: true, TopK: 5, MinScore: 0.3},
		Tier2:   TierConfig{Enabled: true, TopK: 50, MinScore: 0.2},
		Tier3:   DisabledTier(),
		Rerank:  PureRerank(10),
		Scoring: defaultScoringConfig(),
		Timeout: 120 * time.Second,
	}
}

// ---------- 构建选项 ----------

// MemoryBuildOptions - Memory 构建时的全局选项
type MemoryBuildOptions struct {
	Extraction ExtractionOptions `json:"extraction"`
	Retrieval  RetrievalOptions  `json:"retrieval"`
}

// ExtractionOptions - 提取子选项
type ExtractionOptions struct {
	Common  ExtractionCommonOptions  `json:"common"`
	Item    ItemExtractionOptions    `json:"item"`
	Insight InsightExtractionOptions `json:"insight"`
	RawData RawDataExtractionOptions `json:"rawData"`
}

// ExtractionCommonOptions - 提取通用选项
type ExtractionCommonOptions struct {
	MaxMessageBatchSize int `json:"maxMessageBatchSize"`
}

// ItemExtractionOptions - 记忆条目提取选项
type ItemExtractionOptions struct {
	Enabled bool `json:"enabled"`
}

// InsightExtractionOptions - 洞察提取选项
type InsightExtractionOptions struct {
	Enabled bool `json:"enabled"`
}

// RawDataExtractionOptions - 原始数据提取选项
type RawDataExtractionOptions struct {
	Enabled bool `json:"enabled"`
}

// RetrievalOptions - 检索子选项
type RetrievalOptions struct {
	Common RetrievalCommonOptions `json:"common"`
	Simple SimpleRetrievalOptions `json:"simple"`
	Deep   DeepRetrievalOptions   `json:"deep"`
}

// RetrievalCommonOptions - 检索通用选项
type RetrievalCommonOptions struct {
	DefaultStrategy Strategy `json:"defaultStrategy"`
}

// SimpleRetrievalOptions - Simple 策略选项
type SimpleRetrievalOptions struct {
	Enabled bool `json:"enabled"`
}

// DeepRetrievalOptions - Deep 策略选项
type DeepRetrievalOptions struct {
	Enabled bool `json:"enabled"`
}

// DefaultBuildOptions - 返回默认构建选项
func DefaultBuildOptions() MemoryBuildOptions {
	return MemoryBuildOptions{
		Extraction: ExtractionOptions{
			Common:  ExtractionCommonOptions{MaxMessageBatchSize: 20},
			Item:    ItemExtractionOptions{Enabled: true},
			Insight: InsightExtractionOptions{Enabled: true},
			RawData: RawDataExtractionOptions{Enabled: true},
		},
		Retrieval: RetrievalOptions{
			Common: RetrievalCommonOptions{DefaultStrategy: StrategySimple},
			Simple: SimpleRetrievalOptions{Enabled: true},
			Deep:   DeepRetrievalOptions{Enabled: false},
		},
	}
}

// ---------- 洞察树配置 ----------

// InsightTreeConfig - 洞察树构建的参数配置
type InsightTreeConfig struct {
	BranchBubbleThreshold int `json:"branchBubbleThreshold"`
	RootBubbleThreshold   int `json:"rootBubbleThreshold"`
	MinBranchesForRoot    int `json:"minBranchesForRoot"`
	RootTargetTokens      int `json:"rootTargetTokens"`
}

// DefaultInsightTreeConfig - 返回默认洞察树配置
func DefaultInsightTreeConfig() InsightTreeConfig {
	return InsightTreeConfig{
		BranchBubbleThreshold: 3,
		RootBubbleThreshold:   2,
		MinBranchesForRoot:    2,
		RootTargetTokens:      800,
	}
}
