package insight

import (
	memind "github.com/openmemind/memind-go"
)

// BranchAssist - Graph 助手返回的 Branch 级别辅助信息
// Modified: 2026-05-28 - 原版 Java InsightGraphAssistContext 的 Go 移植
type BranchAssist struct {
	OrderedLeafInsights []*memind.MemoryInsight
	AdditionalContext   string
}

// RootAssist - Graph 助手返回的 Root 级别辅助信息
type RootAssist struct {
	OrderedBranchInsights []*memind.MemoryInsight
	AdditionalContext     string
}

// GraphAssistant - 图辅助接口，可选的排序/上下文增强
// Modified: 2026-05-28 - 原版 Java InsightGraphAssistant 的 Go 移植
type GraphAssistant interface {
	BranchAssist(memoryID memind.MemoryId, insightType memind.MemoryInsightType, leafInsights []*memind.MemoryInsight) *BranchAssist
	RootAssist(memoryID memind.MemoryId, rootType memind.MemoryInsightType, branchInsights []*memind.MemoryInsight) *RootAssist
}

// NoOpGraphAssistant - 空操作图辅助（默认）
type NoOpGraphAssistant struct{}

// NewNoOpGraphAssistant - 创建空操作图辅助
func NewNoOpGraphAssistant() *NoOpGraphAssistant {
	return &NoOpGraphAssistant{}
}

func (g *NoOpGraphAssistant) BranchAssist(memoryID memind.MemoryId, insightType memind.MemoryInsightType, leafInsights []*memind.MemoryInsight) *BranchAssist {
	return nil
}

func (g *NoOpGraphAssistant) RootAssist(memoryID memind.MemoryId, rootType memind.MemoryInsightType, branchInsights []*memind.MemoryInsight) *RootAssist {
	return nil
}

// branchAssistIdentity - 返回标识性辅助（无排序调整）
func branchAssistIdentity(leaves []*memind.MemoryInsight) *BranchAssist {
	return &BranchAssist{
		OrderedLeafInsights: leaves,
	}
}

// resolveBranchAssist - 统一处理 Graph 助手的 BranchAssist 结果
func resolveBranchAssist(graph GraphAssistant, memoryID memind.MemoryId, insightType memind.MemoryInsightType, leaves []*memind.MemoryInsight) *BranchAssist {
	if graph == nil {
		return branchAssistIdentity(leaves)
	}
	assist := graph.BranchAssist(memoryID, insightType, leaves)
	if assist == nil {
		return branchAssistIdentity(leaves)
	}
	if len(assist.OrderedLeafInsights) != len(leaves) {
		assist.OrderedLeafInsights = leaves
	}
	return assist
}

// rootAssistIdentity - 返回标识性 Root 辅助
func rootAssistIdentity(branches []*memind.MemoryInsight) *RootAssist {
	return &RootAssist{
		OrderedBranchInsights: branches,
	}
}

// resolveRootAssist - 统一处理 Graph 助手的 RootAssist 结果
func resolveRootAssist(graph GraphAssistant, memoryID memind.MemoryId, rootType memind.MemoryInsightType, branches []*memind.MemoryInsight) *RootAssist {
	if graph == nil {
		return rootAssistIdentity(branches)
	}
	assist := graph.RootAssist(memoryID, rootType, branches)
	if assist == nil {
		return rootAssistIdentity(branches)
	}
	if len(assist.OrderedBranchInsights) != len(branches) {
		assist.OrderedBranchInsights = branches
	}
	return assist
}

// normalizeAdditionalContext - 规范化额外上下文
func normalizeAdditionalContext(ctx string) string {
	if ctx == "" {
		return ""
	}
	return ctx
}
