package insight

import (
	"github.com/openmemind/memind-go/store"
)

// TreeBuilder - 洞察树构建器，管理 Leaf → Branch → Root 三层递进
type TreeBuilder struct {
	store  store.InsightOperations
	config Config
}

// Config - 洞察树构建配置
type Config struct {
	LeafLevel   TierConfig
	BranchLevel TierConfig
	RootLevel   TierConfig
}

// TierConfig - 单层配置
type TierConfig struct {
	MaxChildren  int
	SimilarityFn func(a, b string) float64
}

// NewTreeBuilder - 创建洞察树构建器
func NewTreeBuilder(s store.MemoryStore, cfg Config) *TreeBuilder {
	return &TreeBuilder{
		store:  s.Insights(),
		config: cfg,
	}
}
