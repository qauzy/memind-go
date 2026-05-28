package insight

import (
	"crypto/sha256"
	"fmt"
	"sort"

	memind "github.com/openmemind/memind-go"
)

// PointIdentityManager - 洞察点 ID 管理与复用
// Modified: 2026-05-28 - 原版 Java InsightPointIdentityManager 的 Go 移植
type PointIdentityManager struct{}

// NewPointIdentityManager - 创建 PointIdentityManager
func NewPointIdentityManager() *PointIdentityManager {
	return &PointIdentityManager{}
}

// NormalizePersistedPoints - 确保持久化的 points 都有合法 pointId，缺失的补 sha256(content)
func (m *PointIdentityManager) NormalizePersistedPoints(points []memind.InsightPoint) []memind.InsightPoint {
	changed := false
	result := make([]memind.InsightPoint, len(points))
	for i, p := range points {
		if p.PointID == "" {
			p.PointID = pointContentHash(p)
			changed = true
		}
		result[i] = p
	}
	if !changed {
		return points
	}
	return result
}

// NormalizeGeneratedOperations - 归一化 LLM 生成的操作，将 PointID 与现有 points 对齐
func (m *PointIdentityManager) NormalizeGeneratedOperations(existing []memind.InsightPoint, ops []memind.InsightPointOp) []memind.InsightPointOp {
	existingMap := make(map[string]memind.InsightPoint)
	for _, p := range existing {
		existingMap[p.PointID] = p
	}

	result := make([]memind.InsightPointOp, len(ops))
	for i, op := range ops {
		// 对 ADD 操作生成新的 pointId
		if op.Op == memind.OpAdd && op.PointID == "" {
			op.PointID = fmt.Sprintf("new-%d", i)
		}
		// 对 UPDATE/DELETE 操作验证 pointId 是否有效
		if (op.Op == memind.OpUpdate || op.Op == memind.OpDelete) && op.PointID != "" {
			if _, exists := existingMap[op.PointID]; !exists {
				// pointId 在现有 points 中不存在，转为 ADD
				op.Op = memind.OpAdd
			}
		}
		result[i] = op
	}
	return result
}

// ReusePointIDsForFullRewrite - 在全量重写时尽可能复用现有 pointId
func (m *PointIdentityManager) ReusePointIDsForFullRewrite(existing []memind.InsightPoint, generated []memind.InsightPoint) []memind.InsightPoint {
	existingByContent := make(map[string]memind.InsightPoint)
	for _, p := range existing {
		key := pointContentHash(p)
		existingByContent[key] = p
	}

	result := make([]memind.InsightPoint, len(generated))
	for i, p := range generated {
		key := pointContentHash(p)
		if ep, ok := existingByContent[key]; ok && ep.PointID != "" {
			p.PointID = ep.PointID
		} else {
			p.PointID = fmt.Sprintf("fp-%s", key[:12])
		}
		result[i] = p
	}
	return result
}

// pointContentHash - 基于 point type + content 生成哈希标识
func pointContentHash(p memind.InsightPoint) string {
	h := sha256.New()
	h.Write([]byte(string(p.Type) + ":" + p.Content))
	return fmt.Sprintf("%x", h.Sum(nil)[:8])
}

// StableInsightID - 生成稳定的洞察 ID
func StableInsightID(memoryID, tier, key string) string {
	h := sha256.New()
	h.Write([]byte(memoryID + ":" + tier + ":" + key))
	return fmt.Sprintf("insight-%x", h.Sum(nil)[:12])
}

// SortInsightsByID - 按 ID 排序洞察列表，确保一致性
func SortInsightsByID(insights []*memind.MemoryInsight) {
	sort.Slice(insights, func(i, j int) bool {
		return insights[i].ID < insights[j].ID
	})
}
