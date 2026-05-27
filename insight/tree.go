package insight

import (
	"fmt"
	"time"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/llm"
	"github.com/openmemind/memind-go/store"
)

// TreeBuilder - 洞察树构建器，管理 Leaf → Branch → Root 三层递进
type TreeBuilder struct {
	store  store.InsightOperations
	llm    *llm.ChatClientRegistry
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

// SetLLM - 设置 LLM 客户端注册表（用于洞察合并）
func (tb *TreeBuilder) SetLLM(llm *llm.ChatClientRegistry) {
	tb.llm = llm
}

// Promote - 执行一轮完整晋升：扫描 Leaf→Branch→Root
func (tb *TreeBuilder) Promote(memoryID memind.MemoryId) error {
	// 第一轮：Leaf → Branch
	if err := tb.promoteLeaves(memoryID); err != nil {
		return fmt.Errorf("leaf promotion: %w", err)
	}
	// 第二轮：Branch → Root
	if err := tb.promoteBranches(memoryID); err != nil {
		return fmt.Errorf("branch promotion: %w", err)
	}
	return nil
}

// promoteLeaves - 将同类型的 Leaf 合并晋升为 Branch
func (tb *TreeBuilder) promoteLeaves(memoryID memind.MemoryId) error {
	leaves, err := tb.store.GetInsightsByTier(memoryID, memind.TierLeaf)
	if err != nil {
		return err
	}

	// 按 (type, scope) 分组
	type groupKey struct{ typ, scope string }
	groups := make(map[groupKey][]*memind.MemoryInsight)
	for _, ins := range leaves {
		key := groupKey{typ: ins.Type, scope: string(ins.Scope)}
		groups[key] = append(groups[key], ins)
	}

	threshold := tb.config.LeafLevel.MaxChildren
	if threshold <= 0 {
		threshold = 3
	}

	for key, group := range groups {
		if len(group) < threshold || hasParent(group) {
			continue
		}

		branchIns, err := tb.mergeInsights(memoryID, group, memind.TierBranch, string(key.scope))
		if err != nil {
			return err
		}
		if branchIns == nil {
			continue
		}

		// 将子洞察的 parent 指向新 branch
		childIDs := make([]int64, len(group))
		for i, leaf := range group {
			leaf.ParentInsightID = &branchIns.ID
			leaf.Tier = memind.TierBranch
			childIDs[i] = leaf.ID
		}
		branchIns.ChildInsightIDs = childIDs
		_ = tb.store.UpsertInsights(memoryID, append([]*memind.MemoryInsight{branchIns}, group...))
	}
	return nil
}

// promoteBranches - 将同类型的 Branch 合并晋升为 Root
func (tb *TreeBuilder) promoteBranches(memoryID memind.MemoryId) error {
	branches, err := tb.store.GetInsightsByTier(memoryID, memind.TierBranch)
	if err != nil {
		return err
	}

	type groupKey struct{ typ, scope string }
	groups := make(map[groupKey][]*memind.MemoryInsight)
	for _, ins := range branches {
		if ins.ParentInsightID != nil {
			continue
		}
		key := groupKey{typ: ins.Type, scope: string(ins.Scope)}
		groups[key] = append(groups[key], ins)
	}

	threshold := tb.config.BranchLevel.MaxChildren
	if threshold <= 0 {
		threshold = 2
	}

	for key, group := range groups {
		if len(group) < threshold {
			continue
		}

		rootIns, err := tb.mergeInsights(memoryID, group, memind.TierRoot, string(key.scope))
		if err != nil {
			return err
		}
		if rootIns == nil {
			continue
		}

		childIDs := make([]int64, len(group))
		for i, branch := range group {
			branch.ParentInsightID = &rootIns.ID
			childIDs[i] = branch.ID
		}
		rootIns.ChildInsightIDs = childIDs
		_ = tb.store.UpsertInsights(memoryID, append([]*memind.MemoryInsight{rootIns}, group...))
	}
	return nil
}

// mergeInsights - 将一组子洞察合并为上层洞察（Branch/Root）
func (tb *TreeBuilder) mergeInsights(memoryID memind.MemoryId, children []*memind.MemoryInsight, targetTier memind.InsightTier, scope string) (*memind.MemoryInsight, error) {
	if len(children) == 0 {
		return nil, nil
	}

	now := time.Now()
	ins := &memind.MemoryInsight{
		MemoryID:  memoryID.Identifier(),
		Type:      children[0].Type,
		Scope:     memind.MemoryScope(scope),
		Name:      children[0].Name,
		Points:    tb.mergePoints(children),
		CreatedAt: now,
		UpdatedAt: now,
		Tier:      targetTier,
		Version:   1,
	}

	if err := tb.store.UpsertInsights(memoryID, []*memind.MemoryInsight{ins}); err != nil {
		return nil, err
	}
	return ins, nil
}

// mergePoints - 合并多个子洞察的 points，附带 source 引用
func (tb *TreeBuilder) mergePoints(children []*memind.MemoryInsight) []memind.InsightPoint {
	var merged []memind.InsightPoint
	for _, child := range children {
		for _, p := range child.Points {
			refs := []memind.InsightPointRef{
				{InsightID: child.ID, PointID: p.PointID},
			}
			merged = append(merged, memind.InsightPoint{
				PointID:       fmt.Sprintf("mp-%s-%d", p.PointID, time.Now().UnixNano()),
				Type:          p.Type,
				Content:       p.Content,
				SourceItemIDs: p.SourceItemIDs,
				SourceRefs:    refs,
				Metadata:      p.Metadata,
			})
		}
	}
	return merged
}

// hasParent - 检查一组洞察中是否有任一已挂载父节点
func hasParent(insights []*memind.MemoryInsight) bool {
	for _, ins := range insights {
		if ins.ParentInsightID != nil {
			return true
		}
	}
	return false
}
