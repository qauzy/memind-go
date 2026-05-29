package insight

import (
	memind "github.com/openmemind/memind-go"
)

// EvidenceNormalizer - 证据点引用归一化器
// Modified: 2026-05-28 - 原版 Java InsightPointEvidenceNormalizer 的 Go 移植
type EvidenceNormalizer struct{}

// NewEvidenceNormalizer - 创建 EvidenceNormalizer
func NewEvidenceNormalizer() *EvidenceNormalizer {
	return &EvidenceNormalizer{}
}

// NormalizeBranchPoints - 归一化 Branch 的 points，在 sourcePointRefs 中记录对 Leaf 的引用
func (n *EvidenceNormalizer) NormalizeBranchPoints(points []memind.InsightPoint, leafInsights []*memind.MemoryInsight) []memind.InsightPoint {
	flatLeafPoints := flattenLeafPoints(leafInsights)
	result := make([]memind.InsightPoint, len(points))
	for i, p := range points {
		p.SourceRefs = resolveSourceRefs(p, flatLeafPoints)
		p.SourceItemIDs = collectSourceItemIDs(flatLeafPoints)
		if p.Metadata == nil {
			p.Metadata = make(map[string]string)
		}
		p.Metadata["tier"] = string(memind.TierBranch)
		result[i] = p
	}
	return result
}

// NormalizeRootPoints - 归一化 Root 的 points，在 sourcePointRefs 中记录对 Branch 的引用
func (n *EvidenceNormalizer) NormalizeRootPoints(points []memind.InsightPoint, branchInsights []*memind.MemoryInsight) []memind.InsightPoint {
	flatBranchPoints := flattenInsightPoints(branchInsights)
	result := make([]memind.InsightPoint, len(points))
	for i, p := range points {
		p.SourceRefs = resolveSourceRefs(p, flatBranchPoints)
		p.SourceItemIDs = collectSourceItemIDs(flatBranchPoints)
		if p.Metadata == nil {
			p.Metadata = make(map[string]string)
		}
		p.Metadata["tier"] = string(memind.TierRoot)
		result[i] = p
	}
	return result
}

// flattenLeafPoints - 展平所有 Leaf 的 points，返回带 insightId 的引用
func flattenLeafPoints(leaves []*memind.MemoryInsight) []memind.InsightPoint {
	var result []memind.InsightPoint
	for _, leaf := range leaves {
		for _, p := range leaf.Points {
			ref := memind.InsightPointRef{InsightID: leaf.ID, PointID: p.PointID}
			p.SourceRefs = append(p.SourceRefs, ref)
			result = append(result, p)
		}
	}
	return result
}

// flattenInsightPoints - 通用 insights 展平
func flattenInsightPoints(insights []*memind.MemoryInsight) []memind.InsightPoint {
	var result []memind.InsightPoint
	for _, ins := range insights {
		for _, p := range ins.Points {
			result = append(result, p)
		}
	}
	return result
}

// resolveSourceRefs - 找出与 point content 匹配的子点引用
func resolveSourceRefs(point memind.InsightPoint, childPoints []memind.InsightPoint) []memind.InsightPointRef {
	var refs []memind.InsightPointRef
	for _, cp := range childPoints {
		if cp.PointID == point.PointID || cp.Content == point.Content {
			for _, r := range cp.SourceRefs {
				refs = append(refs, r)
			}
		}
	}
	if len(refs) == 0 && len(childPoints) > 0 {
		refs = childPoints[0].SourceRefs
	}
	return refs
}

// collectSourceItemIDs - 从 points 中收集所有 sourceItemId
func collectSourceItemIDs(points []memind.InsightPoint) []string {
	seen := make(map[string]bool)
	var ids []string
	for _, p := range points {
		for _, id := range p.SourceItemIDs {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}
