package insight

import (
	memind "github.com/openmemind/memind-go"
)

// resolvedPoints - Point 操作解析结果
// Modified: 2026-05-28 - 原版 Java ResolvedPointOperations 的 Go 移植
type resolvedPoints struct {
	points           []memind.InsightPoint
	noop             bool
	fallbackRequired bool
}

// resolvePointOps - 将操作列表解析为新的 points 列表
// Modified: 2026-05-28 - 原版 Java PointOperationResolver 的 Go 移植
func resolvePointOps(existing []memind.InsightPoint, ops []memind.InsightPointOp) resolvedPoints {
	if len(ops) == 0 {
		return resolvedPoints{points: existing, noop: true}
	}

	pointMap := make(map[string]memind.InsightPoint)
	for _, p := range existing {
		pointMap[p.PointID] = p
	}

	var changed bool
	for _, op := range ops {
		switch op.Op {
		case memind.OpAdd:
			if op.PointID == "" {
				return resolvedPoints{fallbackRequired: true}
			}
			if _, exists := pointMap[op.PointID]; exists {
				continue
			}
			typ := memind.PointTypeSummary
			if op.Type != nil {
				typ = *op.Type
			}
			pointMap[op.PointID] = memind.InsightPoint{
				PointID:       op.PointID,
				Type:          typ,
				Content:       op.Content,
				SourceItemIDs: op.SourceItemIDs,
				Metadata:      op.Metadata,
			}
			changed = true

		case memind.OpUpdate:
			existingPoint, exists := pointMap[op.PointID]
			if !exists {
				return resolvedPoints{fallbackRequired: true}
			}
			if op.Content != "" {
				existingPoint.Content = op.Content
			}
			if op.Type != nil {
				existingPoint.Type = *op.Type
			}
			if op.SourceItemIDs != nil {
				existingPoint.SourceItemIDs = op.SourceItemIDs
			}
			if op.Metadata != nil {
				if existingPoint.Metadata == nil {
					existingPoint.Metadata = make(map[string]string)
				}
				for k, v := range op.Metadata {
					existingPoint.Metadata[k] = v
				}
			}
			pointMap[op.PointID] = existingPoint
			changed = true

		case memind.OpDelete:
			if _, exists := pointMap[op.PointID]; !exists {
				continue
			}
			delete(pointMap, op.PointID)
			changed = true
		}
	}

	if !changed {
		return resolvedPoints{points: existing, noop: true}
	}

	result := make([]memind.InsightPoint, 0, len(pointMap))
	for _, p := range pointMap {
		result = append(result, p)
	}
	return resolvedPoints{points: result}
}
