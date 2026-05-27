package insight

import (
	"fmt"
	"time"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/store"
)

type TreeBuilder struct {
	store  store.MemoryStore
	config memind.InsightTreeConfig
}

func NewTreeBuilder(s store.MemoryStore, config memind.InsightTreeConfig) *TreeBuilder {
	return &TreeBuilder{store: s, config: config}
}

func (tb *TreeBuilder) BuildLeaves(memoryID memind.MemoryId, items []*memind.MemoryItem, insightTypes []memind.MemoryInsightType) ([]*memind.MemoryInsight, error) {
	if len(items) == 0 {
		return nil, nil
	}

	allTypes, _ := tb.store.Insights().ListInsightTypes()

	itemByCategory := make(map[string][]*memind.MemoryItem)
	for _, item := range items {
		cat := string(item.Category)
		itemByCategory[cat] = append(itemByCategory[cat], item)
	}

	var leaves []*memind.MemoryInsight
	for _, insType := range allTypes {
		for _, cat := range insType.Categories {
			if catItems, ok := itemByCategory[cat]; ok {
				leaf, err := tb.buildLeaf(memoryID, catItems, *insType)
				if err != nil {
					continue
				}
				if leaf != nil {
					leaves = append(leaves, leaf)
				}
			}
		}
	}

	if len(leaves) > 0 {
		tb.store.Insights().UpsertInsights(memoryID, leaves)
	}
	return leaves, nil
}

func (tb *TreeBuilder) buildLeaf(memoryID memind.MemoryId, items []*memind.MemoryItem, insType memind.MemoryInsightType) (*memind.MemoryInsight, error) {
	if len(items) == 0 {
		return nil, nil
	}

	group := insType.Name
	existing, _ := tb.store.Insights().GetInsight(memoryID, 0)
	_ = existing

	now := time.Now()
	var points []memind.InsightPoint
	for _, item := range items {
		points = append(points, memind.InsightPoint{
			PointID:       item.ContentHash,
			Type:          memind.PointTypeSummary,
			Content:       item.Content,
			SourceItemIDs: []string{fmt.Sprintf("%d", item.ID)},
		})
	}

	leaf := &memind.MemoryInsight{
		MemoryID:   memoryID.Identifier(),
		Type:       insType.Name,
		Scope:      insType.Scope,
		Name:       insType.Name,
		Categories: insType.Categories,
		Points:     points,
		Group:      group,
		Tier:       memind.TierLeaf,
		CreatedAt:  now,
		UpdatedAt:  now,
		Version:    1,
	}

	return leaf, nil
}

func (tb *TreeBuilder) BuildBranches(memoryID memind.MemoryId, leaves []*memind.MemoryInsight) ([]*memind.MemoryInsight, error) {
	if len(leaves) < tb.config.BranchBubbleThreshold {
		return nil, nil
	}

	leavesByType := make(map[string][]*memind.MemoryInsight)
	for _, leaf := range leaves {
		leavesByType[leaf.Type] = append(leavesByType[leaf.Type], leaf)
	}

	var branches []*memind.MemoryInsight
	for typeName, groupLeaves := range leavesByType {
		if len(groupLeaves) < tb.config.BranchBubbleThreshold {
			continue
		}

		insType, _ := tb.store.Insights().GetInsightType(typeName)
		if insType == nil {
			continue
		}

		now := time.Now()
		var points []memind.InsightPoint
		var childIDs []int64
		var allContent string

		for _, leaf := range groupLeaves {
			childIDs = append(childIDs, leaf.ID)
			allContent += leaf.PointsContent() + " "
			for _, p := range leaf.Points {
				points = append(points, p)
			}
		}

		branch := &memind.MemoryInsight{
			MemoryID:        memoryID.Identifier(),
			Type:            typeName,
			Scope:           insType.Scope,
			Name:            typeName,
			Categories:      insType.Categories,
			Points:          points,
			Group:           typeName,
			Tier:            memind.TierBranch,
			ParentInsightID: nil,
			ChildInsightIDs: childIDs,
			CreatedAt:       now,
			UpdatedAt:       now,
			Version:         1,
			LastReasonedAt:  &now,
		}
		branches = append(branches, branch)
	}

	if len(branches) > 0 {
		tb.store.Insights().UpsertInsights(memoryID, branches)
	}
	return branches, nil
}

func (tb *TreeBuilder) BuildRoots(memoryID memind.MemoryId, branches []*memind.MemoryInsight) ([]*memind.MemoryInsight, error) {
	if len(branches) < tb.config.MinBranchesForRoot {
		return nil, nil
	}

	now := time.Now()
	allPoints := make(map[string][]*memind.MemoryInsight)

	for _, branch := range branches {
		scope := string(branch.Scope)
		allPoints[scope] = append(allPoints[scope], branch)
	}

	var roots []*memind.MemoryInsight
	for scope, scopeBranches := range allPoints {
		if len(scopeBranches) < tb.config.MinBranchesForRoot {
			continue
		}

		rootName := "profile"
		memScope := memind.ScopeUser
		if scope == string(memind.ScopeAgent) {
			rootName = "interaction"
			memScope = memind.ScopeAgent
		}

		var points []memind.InsightPoint
		var childIDs []int64
		for _, b := range scopeBranches {
			childIDs = append(childIDs, b.ID)
			for _, p := range b.Points {
				points = append(points, p)
			}
		}

		root := &memind.MemoryInsight{
			MemoryID:        memoryID.Identifier(),
			Type:            rootName,
			Scope:           memScope,
			Name:            rootName,
			Categories:      []string{"PROFILE", "BEHAVIOR", "EVENT"},
			Points:          points,
			Group:           rootName,
			Tier:            memind.TierRoot,
			ChildInsightIDs: childIDs,
			CreatedAt:       now,
			UpdatedAt:       now,
			Version:         1,
			LastReasonedAt:  &now,
		}
		roots = append(roots, root)
	}

	if len(roots) > 0 {
		tb.store.Insights().UpsertInsights(memoryID, roots)
	}
	return roots, nil
}
