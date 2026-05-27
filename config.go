package memind

import "time"

type RetrievalConfig struct {
	Tier1         TierConfig     `json:"tier1"`
	Tier2         TierConfig     `json:"tier2"`
	Tier3         TierConfig     `json:"tier3"`
	Rerank        RerankConfig   `json:"rerank"`
	Scoring       ScoringConfig  `json:"scoring"`
	Timeout       time.Duration  `json:"-"`
	EnableCache   bool           `json:"enableCache"`
	StrategyConfig StrategyConfig `json:"strategyConfig"`
}

type TierConfig struct {
	Enabled    bool             `json:"enabled"`
	TopK       int              `json:"topK"`
	MinScore   float64          `json:"minScore"`
	Truncation TruncationConfig `json:"truncation"`
}

func EnablesTier(topK int) TierConfig {
	return TierConfig{Enabled: true, TopK: topK, MinScore: 0.0}
}

func DisabledTier() TierConfig {
	return TierConfig{Enabled: false}
}

type TruncationConfig struct {
	Enabled      bool `json:"enabled"`
	MaxItems     int  `json:"maxItems"`
	TargetTokens int  `json:"targetTokens"`
}

type RerankConfig struct {
	Enabled           bool    `json:"enabled"`
	BlendWithRetrieval bool   `json:"blendWithRetrieval"`
	Top3Weight        float64 `json:"top3Weight"`
	Top10Weight       float64 `json:"top10Weight"`
	OtherWeight       float64 `json:"otherWeight"`
	TopK              int     `json:"topK"`
}

func PureRerank(topK int) RerankConfig {
	return RerankConfig{Enabled: true, BlendWithRetrieval: false, TopK: topK}
}

func DisabledRerank() RerankConfig {
	return RerankConfig{Enabled: false, TopK: 0}
}

type ScoringConfig struct {
	Fusion              FusionConfig         `json:"fusion"`
	TimeDecay           TimeDecayConfig      `json:"timeDecay"`
	Recency             RecencyConfig        `json:"recency"`
	PositionBonus       PositionBonusConfig  `json:"positionBonus"`
	KeywordSearch       KeywordSearchConfig  `json:"keywordSearch"`
	QueryWeight         QueryWeightConfig    `json:"queryWeight"`
	CandidateMultiplier int                  `json:"candidateMultiplier"`
	RerankCandidateLimit int                 `json:"rerankCandidateLimit"`
	RawDataKeyInfoMaxLines int               `json:"rawDataKeyInfoMaxLines"`
	InsightLlmThreshold  int                 `json:"insightLlmThreshold"`
}

type FusionConfig struct {
	K             int     `json:"k"`
	VectorWeight  float64 `json:"vectorWeight"`
	KeywordWeight float64 `json:"keywordWeight"`
}

type TimeDecayConfig struct {
	Rate               float64 `json:"rate"`
	Floor              float64 `json:"floor"`
	OutOfRangePenalty  float64 `json:"outOfRangePenalty"`
}

type RecencyConfig struct {
	Rate  float64 `json:"rate"`
	Floor float64 `json:"floor"`
}

type PositionBonusConfig struct {
	Top1 float64 `json:"top1"`
	Top3 float64 `json:"top3"`
}

type KeywordSearchConfig struct {
	ProbeTopK          int     `json:"probeTopK"`
	StrongSignalMinScore float64 `json:"strongSignalMinScore"`
	StrongSignalMinGap   float64 `json:"strongSignalMinGap"`
}

type QueryWeightConfig struct {
	OriginalWeight  float64 `json:"originalWeight"`
	ExpandedWeight  float64 `json:"expandedWeight"`
}

type StrategyConfig struct {
	Simple SimpleStrategyConfig `json:"simple"`
	Deep   DeepStrategyConfig   `json:"deep"`
}

type SimpleStrategyConfig struct {
}

type DeepStrategyConfig struct {
}

func defaultScoringConfig() ScoringConfig {
	return ScoringConfig{
		Fusion:              FusionConfig{K: 60, VectorWeight: 1.5, KeywordWeight: 1.0},
		TimeDecay:           TimeDecayConfig{Rate: 0.023, Floor: 0.3, OutOfRangePenalty: 0.5},
		Recency:             RecencyConfig{Rate: 0.0019, Floor: 0.7},
		PositionBonus:       PositionBonusConfig{Top1: 0.05, Top3: 0.02},
		KeywordSearch:       KeywordSearchConfig{ProbeTopK: 10, StrongSignalMinScore: 0.85, StrongSignalMinGap: 0.15},
		QueryWeight:         QueryWeightConfig{OriginalWeight: 2.0, ExpandedWeight: 1.0},
		CandidateMultiplier: 3,
		RerankCandidateLimit: 50,
		RawDataKeyInfoMaxLines: 20,
		InsightLlmThreshold: 5,
	}
}

func SimpleRetrievalConfig() RetrievalConfig {
	return RetrievalConfig{
		Tier1:   TierConfig{Enabled: true, TopK: 5, MinScore: 0.3},
		Tier2:   TierConfig{Enabled: true, TopK: 15, MinScore: 0.1, Truncation: TruncationConfig{Enabled: true, MaxItems: 30, TargetTokens: 4000}},
		Tier3:   TierConfig{Enabled: true, TopK: 5, MinScore: 0.0},
		Rerank:  DisabledRerank(),
		Scoring: defaultScoringConfig(),
		Timeout: 10 * time.Second,
	}
}

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

type MemoryBuildOptions struct {
	Extraction ExtractionOptions `json:"extraction"`
	Retrieval  RetrievalOptions  `json:"retrieval"`
}

type ExtractionOptions struct {
	Common   ExtractionCommonOptions `json:"common"`
	Item     ItemExtractionOptions    `json:"item"`
	Insight  InsightExtractionOptions `json:"insight"`
	RawData  RawDataExtractionOptions `json:"rawData"`
}

type ExtractionCommonOptions struct {
	MaxMessageBatchSize int `json:"maxMessageBatchSize"`
}

type ItemExtractionOptions struct {
	Enabled bool `json:"enabled"`
}

type InsightExtractionOptions struct {
	Enabled bool `json:"enabled"`
}

type RawDataExtractionOptions struct {
	Enabled bool `json:"enabled"`
}

type RetrievalOptions struct {
	Common  RetrievalCommonOptions `json:"common"`
	Simple  SimpleRetrievalOptions `json:"simple"`
	Deep    DeepRetrievalOptions   `json:"deep"`
}

type RetrievalCommonOptions struct {
	DefaultStrategy Strategy `json:"defaultStrategy"`
}

type SimpleRetrievalOptions struct {
	Enabled bool `json:"enabled"`
}

type DeepRetrievalOptions struct {
	Enabled bool `json:"enabled"`
}

func DefaultBuildOptions() MemoryBuildOptions {
	return MemoryBuildOptions{
		Extraction: ExtractionOptions{
			Common:  ExtractionCommonOptions{MaxMessageBatchSize: 20},
			Item:    ItemExtractionOptions{Enabled: true},
			Insight: InsightExtractionOptions{Enabled: true},
			RawData: RawDataExtractionOptions{Enabled: true},
		},
		Retrieval: RetrievalOptions{
			Common:  RetrievalCommonOptions{DefaultStrategy: StrategySimple},
			Simple:  SimpleRetrievalOptions{Enabled: true},
			Deep:    DeepRetrievalOptions{Enabled: false},
		},
	}
}

type InsightTreeConfig struct {
	BranchBubbleThreshold int `json:"branchBubbleThreshold"`
	RootBubbleThreshold   int `json:"rootBubbleThreshold"`
	MinBranchesForRoot    int `json:"minBranchesForRoot"`
	RootTargetTokens      int `json:"rootTargetTokens"`
}

func DefaultInsightTreeConfig() InsightTreeConfig {
	return InsightTreeConfig{
		BranchBubbleThreshold: 3,
		RootBubbleThreshold:   2,
		MinBranchesForRoot:    2,
		RootTargetTokens:      800,
	}
}
