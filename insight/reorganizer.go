package insight

import (
	"fmt"
	"log"
	"sync"
	"time"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/llm"
	"github.com/openmemind/memind-go/store"
	"github.com/openmemind/memind-go/vector"
)

// TreeReorganizer - 洞察树重组织器，管理 Leaf → Branch → Root 三层结构
// Modified: 2026-05-28 - 原版 Java InsightTreeReorganizer 的 Go 移植
type TreeReorganizer struct {
	store     store.MemoryStore
	generator InsightGenerator
	vector    vector.MemoryVector
	bubble    *BubbleTracker
	idMgr     *PointIdentityManager
	evidence  *EvidenceNormalizer
	graph     GraphAssistant

	rootLocks    []sync.Mutex
	lockStripes  int
	pendingRoots sync.Map
}

// NewTreeReorganizer - 创建洞察树重组织器
func NewTreeReorganizer(
	memStore store.MemoryStore,
	gen InsightGenerator,
	vec vector.MemoryVector,
	bubble *BubbleTracker,
	graph GraphAssistant,
) *TreeReorganizer {
	stripes := 16
	locks := make([]sync.Mutex, stripes)
	return &TreeReorganizer{
		store:       memStore,
		generator:   gen,
		vector:      vec,
		bubble:      bubble,
		idMgr:       NewPointIdentityManager(),
		evidence:    NewEvidenceNormalizer(),
		graph:       graph,
		rootLocks:   locks,
		lockStripes: stripes,
	}
}

// SetLLM - 设置 LLM 客户端（向后兼容，若未通过构造函数传入生成器则创建）
func (r *TreeReorganizer) SetLLM(llmReg *llm.ChatClientRegistry) {
	if r.generator == nil {
		r.generator = NewLlmInsightGenerator(llmReg)
	}
}

// OnLeafsUpdated - 批量 LEAF 更新后的入口，执行 Branch → Root 整链重组织
// Modified: 2026-05-28 - 原版 Java onLeafsUpdated() 的 Go 移植
func (r *TreeReorganizer) OnLeafsUpdated(
	memoryID memind.MemoryId,
	insightTypeName string,
	insightType memind.MemoryInsightType,
	builtLeafs []*memind.MemoryInsight,
	config memind.InsightTreeConfig,
	language string,
) error {
	if len(builtLeafs) == 0 {
		return nil
	}

	dirtyKey := branchBubbleKey(memoryID, insightTypeName)
	branchDirtyCount := r.bubble.IncrementAndGet(dirtyKey, len(builtLeafs))

	allLeafs, err := r.store.Insights().GetInsightsByType(memoryID, insightTypeName)
	if err != nil {
		return fmt.Errorf("query leafs: %w", err)
	}
	var leafList []*memind.MemoryInsight
	for _, ins := range allLeafs {
		if ins.Tier == memind.TierLeaf {
			leafList = append(leafList, ins)
		}
	}

	branch, err := r.getOrCreateBranch(memoryID, insightTypeName, insightType)
	if err != nil {
		return fmt.Errorf("get/create branch: %w", err)
	}

	linkedBranch := r.batchLinkLeafsToBranch(memoryID, leafList, branch)

	rootCtx := r.queryRootContext(memoryID)
	rootLock := r.rootLock(memoryID)
	rootLock.Lock()
	r.linkBranchToRoot(memoryID, linkedBranch, rootCtx)
	rootLock.Unlock()

	if branchDirtyCount < config.BranchBubbleThreshold {
		return nil
	}

	updatedBranch := r.resummarizeBranch(memoryID, insightTypeName, insightType, linkedBranch, leafList, config, language)
	if updatedBranch == nil {
		return nil
	}

	r.bubble.Reset(dirtyKey)

	rootLock.Lock()
	r.bubbleAndMaybeResummarizeRoots(memoryID, updatedBranch, rootCtx, config, language)
	rootLock.Unlock()

	return nil
}

// ForceResummarizeBranchIfEmpty - 强制重摘要空 Branch（flush 场景）
func (r *TreeReorganizer) ForceResummarizeBranchIfEmpty(
	memoryID memind.MemoryId,
	insightType memind.MemoryInsightType,
	language string,
) {
	branch, _ := r.store.Insights().GetBranchByType(memoryID, insightType.Name)
	if branch == nil {
		return
	}
	if len(branch.Points) > 0 {
		return
	}

	allInsights, _ := r.store.Insights().GetInsightsByType(memoryID, insightType.Name)
	var leafList []*memind.MemoryInsight
	for _, ins := range allInsights {
		if ins.Tier == memind.TierLeaf {
			leafList = append(leafList, ins)
		}
	}
	if len(leafList) == 0 {
		return
	}

	config := resolveTreeConfig(insightType)
	updated := r.resummarizeBranch(memoryID, insightType.Name, insightType, branch, leafList, config, language)
	if updated == nil {
		return
	}

	rootCtx := r.queryRootContext(memoryID)
	rootLock := r.rootLock(memoryID)
	rootLock.Lock()
	defer rootLock.Unlock()
	r.bubbleAndMaybeResummarizeRoots(memoryID, updated, rootCtx, config, language)
}

// DrainRootTasks - 等待所有异步 Root 重摘要完成（flush 时调用）
func (r *TreeReorganizer) DrainRootTasks(memoryID memind.MemoryId, timeout time.Duration) {
	val, ok := r.pendingRoots.Load(memoryID.Identifier())
	if !ok {
		return
	}
	wg, ok := val.(*sync.WaitGroup)
	if !ok {
		return
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		log.Printf("drainRootTasks timed out [memoryId=%s]", memoryID.Identifier())
	}
}

// ===== Internal =====

// rootLock - 根据 memoryId 分配 strip lock
func (r *TreeReorganizer) rootLock(memoryID memind.MemoryId) *sync.Mutex {
	idx := hashString(memoryID.Identifier()) % r.lockStripes
	return &r.rootLocks[idx]
}

// getOrCreateBranch - 获取或创建指定类型的 Branch
func (r *TreeReorganizer) getOrCreateBranch(memoryID memind.MemoryId, typeName string, insightType memind.MemoryInsightType) (*memind.MemoryInsight, error) {
	existing, err := r.store.Insights().GetBranchByType(memoryID, typeName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	now := time.Now()
	scope := insightType.Scope
	if scope == "" {
		scope = memind.ScopeUser
	}
	branch := &memind.MemoryInsight{
		MemoryID:  memoryID.Identifier(),
		Type:      typeName,
		Scope:     scope,
		Name:      "branch-" + typeName,
		Points:    []memind.InsightPoint{},
		CreatedAt: now,
		UpdatedAt: now,
		Tier:      memind.TierBranch,
		Version:   1,
	}
	if err := r.store.Insights().UpsertInsights(memoryID, []*memind.MemoryInsight{branch}); err != nil {
		return nil, err
	}
	log.Printf("Creating BRANCH [type=%s]", typeName)
	return branch, nil
}

// batchLinkLeafsToBranch - 批量设置 Leaf → Branch 父子链接
func (r *TreeReorganizer) batchLinkLeafsToBranch(memoryID memind.MemoryId, leafList []*memind.MemoryInsight, branch *memind.MemoryInsight) *memind.MemoryInsight {
	childSet := make(map[int64]bool)
	for _, id := range branch.ChildInsightIDs {
		childSet[id] = true
	}

	var updatedLeafs []*memind.MemoryInsight
	for _, leaf := range leafList {
		if leaf.ParentInsightID == nil || *leaf.ParentInsightID != branch.ID {
			leaf.ParentInsightID = &branch.ID
			updatedLeafs = append(updatedLeafs, leaf)
		}
		childSet[leaf.ID] = true
	}

	var childIDs []int64
	for id := range childSet {
		childIDs = append(childIDs, id)
	}

	if len(updatedLeafs) > 0 {
		updatedBranch := copyInsight(branch)
		updatedBranch.ChildInsightIDs = childIDs
		var toSave []*memind.MemoryInsight
		toSave = append(toSave, updatedBranch)
		toSave = append(toSave, updatedLeafs...)
		_ = r.store.Insights().UpsertInsights(memoryID, toSave)
		return updatedBranch
	}

	return branch
}

// resummarizeBranch - 重摘要 Branch，优先使用 LLM 增量操作，回退全量重写
func (r *TreeReorganizer) resummarizeBranch(
	memoryID memind.MemoryId,
	typeName string,
	insightType memind.MemoryInsightType,
	branch *memind.MemoryInsight,
	leafList []*memind.MemoryInsight,
	config memind.InsightTreeConfig,
	language string,
) *memind.MemoryInsight {
	if len(leafList) == 0 {
		return nil
	}

	log.Printf("Re-summarize BRANCH [type=%s, id=%d, leaves=%d]", typeName, branch.ID, len(leafList))

	branch = r.normalizeInsightIfNeeded(memoryID, branch)
	existingPoints := branch.Points
	if existingPoints == nil {
		existingPoints = []memind.InsightPoint{}
	}

	assist := resolveBranchAssist(r.graph, memoryID, insightType, leafList)
	orderedLeaves := assist.OrderedLeafInsights
	additionalCtx := normalizeAdditionalContext(assist.AdditionalContext)

	opsResp, err := r.generator.GenerateBranchPointOps(insightType, existingPoints, orderedLeaves, config.BranchBubbleThreshold, additionalCtx, language)
	if err == nil && opsResp != nil && len(opsResp.Operations) > 0 {
		normalizedOps := r.idMgr.NormalizeGeneratedOperations(existingPoints, opsResp.Operations)
		resolved := resolvePointOps(existingPoints, normalizedOps)
		if !resolved.fallbackRequired && !resolved.noop {
			points := r.evidence.NormalizeBranchPoints(resolved.points, orderedLeaves)
			return r.embedAndSaveIfChanged(memoryID, branch, points, memind.TierBranch)
		}
	}

	fullResp, err := r.generator.GenerateBranchSummary(insightType, existingPoints, orderedLeaves, config.BranchBubbleThreshold, additionalCtx, language)
	if err == nil && fullResp != nil && len(fullResp.Points) > 0 {
		points := r.evidence.NormalizeBranchPoints(
			r.idMgr.ReusePointIDsForFullRewrite(existingPoints, fullResp.Points),
			orderedLeaves,
		)
		return r.embedAndSaveIfChanged(memoryID, branch, points, memind.TierBranch)
	}

	return r.fallbackBranchConcat(memoryID, branch, orderedLeaves)
}

// fallbackBranchConcat - 无 LLM 时回退为简单拼接
func (r *TreeReorganizer) fallbackBranchConcat(memoryID memind.MemoryId, branch *memind.MemoryInsight, leafList []*memind.MemoryInsight) *memind.MemoryInsight {
	var points []memind.InsightPoint
	for _, leaf := range leafList {
		for _, p := range leaf.Points {
			points = append(points, memind.InsightPoint{
				PointID:       fmt.Sprintf("fb-%d-%s", leaf.ID, p.PointID),
				Type:          p.Type,
				Content:       p.Content,
				SourceItemIDs: p.SourceItemIDs,
				SourceRefs: []memind.InsightPointRef{
					{InsightID: leaf.ID, PointID: p.PointID},
				},
				Metadata: p.Metadata,
			})
		}
	}
	return r.embedAndSave(memoryID, branch, points, memind.TierBranch)
}

// queryRootContext - 查询 Root 上下文
type rootContext struct {
	allBranches []*memind.MemoryInsight
	rootTypes   []memind.MemoryInsightType
}

func (r *TreeReorganizer) queryRootContext(memoryID memind.MemoryId) *rootContext {
	allBranches, _ := r.store.Insights().GetInsightsByTier(memoryID, memind.TierBranch)
	allTypes, _ := r.store.Insights().ListInsightTypes()
	var rootTypes []memind.MemoryInsightType
	for _, t := range allTypes {
		if t.AnalysisMode == memind.AnalysisModeRoot {
			rootTypes = append(rootTypes, *t)
		}
	}
	return &rootContext{allBranches: allBranches, rootTypes: rootTypes}
}

// linkBranchToRoot - 确保 Branch 链接到所有 Root
func (r *TreeReorganizer) linkBranchToRoot(memoryID memind.MemoryId, branch *memind.MemoryInsight, ctx *rootContext) {
	for _, rootType := range ctx.rootTypes {
		config := resolveTreeConfig(rootType)
		if len(ctx.allBranches) < config.MinBranchesForRoot {
			continue
		}
		root := r.ensureRoot(memoryID, rootType, ctx.allBranches)
		r.linkBranchToSingleRoot(memoryID, branch, root)
	}
}

// pendingRootResummarize - 待异步执行的 Root 重摘要描述
type pendingRootResummarize struct {
	rootType memind.MemoryInsightType
	config   memind.InsightTreeConfig
	rootKey  string
}

// bubbleAndMaybeResummarizeRoots - 脏计数达到阈值后触发 Root 重摘要
func (r *TreeReorganizer) bubbleAndMaybeResummarizeRoots(memoryID memind.MemoryId, branch *memind.MemoryInsight, ctx *rootContext, config memind.InsightTreeConfig, language string) {
	var pendingList []pendingRootResummarize
	for _, rootType := range ctx.rootTypes {
		cfg := resolveTreeConfig(rootType)
		if len(ctx.allBranches) < cfg.MinBranchesForRoot {
			continue
		}
		root := r.ensureRoot(memoryID, rootType, ctx.allBranches)
		r.linkBranchToSingleRoot(memoryID, branch, root)

		rootKey := rootBubbleKey(memoryID, rootType.Name)
		rootDirtyCount := r.bubble.IncrementAndGet(rootKey, 1)

		if rootDirtyCount < cfg.RootBubbleThreshold {
			continue
		}
		pendingList = append(pendingList, pendingRootResummarize{rootType: rootType, config: cfg, rootKey: rootKey})
	}

	for _, p := range pendingList {
		p := p
		wg := r.getOrCreateWaitGroup(memoryID)
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.resummarizeRootAndReset(memoryID, p, language)
		}()
	}
}

// resummarizeRootAndReset - 执行 Root 重摘要并重置脏计数
func (r *TreeReorganizer) resummarizeRootAndReset(memoryID memind.MemoryId, p pendingRootResummarize, language string) {
	root, err := r.store.Insights().GetRootByType(memoryID, p.rootType.Name)
	if err != nil || root == nil {
		return
	}
	allBranches, err := r.store.Insights().GetInsightsByTier(memoryID, memind.TierBranch)
	if err != nil {
		return
	}
	r.resummarizeRoot(memoryID, p.rootType, root, allBranches, p.config, language)
	r.bubble.Reset(p.rootKey)
}

// ensureRoot - 获取或创建指定类型的 Root
func (r *TreeReorganizer) ensureRoot(memoryID memind.MemoryId, rootType memind.MemoryInsightType, allBranches []*memind.MemoryInsight) *memind.MemoryInsight {
	existing, _ := r.store.Insights().GetRootByType(memoryID, rootType.Name)
	if existing != nil {
		return existing
	}

	now := time.Now()
	scope := rootType.Scope
	if scope == "" {
		scope = inferScope(allBranches)
	}
	var childIDs []int64
	for _, b := range allBranches {
		childIDs = append(childIDs, b.ID)
	}
	root := &memind.MemoryInsight{
		MemoryID:        memoryID.Identifier(),
		Type:            rootType.Name,
		Scope:           scope,
		Name:            "root-" + rootType.Name,
		Points:          []memind.InsightPoint{},
		CreatedAt:       now,
		UpdatedAt:       now,
		Tier:            memind.TierRoot,
		ChildInsightIDs: childIDs,
		Version:         1,
	}
	_ = r.store.Insights().UpsertInsights(memoryID, []*memind.MemoryInsight{root})
	log.Printf("Creating ROOT [type=%s, branches=%d]", rootType.Name, len(allBranches))
	return root
}

// linkBranchToSingleRoot - 将单个 Branch 链接到 Root
func (r *TreeReorganizer) linkBranchToSingleRoot(memoryID memind.MemoryId, branch *memind.MemoryInsight, root *memind.MemoryInsight) {
	latestRoot, err := r.store.Insights().GetInsight(memoryID, root.ID)
	if err != nil || latestRoot == nil {
		latestRoot = root
	}
	for _, id := range latestRoot.ChildInsightIDs {
		if id == branch.ID {
			return
		}
	}
	updatedRoot := copyInsight(latestRoot)
	updatedRoot.ChildInsightIDs = append(updatedRoot.ChildInsightIDs, branch.ID)
	updatedRoot.UpdatedAt = time.Now()
	_ = r.store.Insights().UpsertInsights(memoryID, []*memind.MemoryInsight{updatedRoot})
	log.Printf("Linking BRANCH [id=%d] -> ROOT [type=%s]", branch.ID, root.Type)
}

// resummarizeRoot - 执行 Root 深度合成
func (r *TreeReorganizer) resummarizeRoot(
	memoryID memind.MemoryId,
	rootType memind.MemoryInsightType,
	root *memind.MemoryInsight,
	allBranches []*memind.MemoryInsight,
	config memind.InsightTreeConfig,
	language string,
) *memind.MemoryInsight {
	if len(allBranches) == 0 {
		return root
	}
	log.Printf("Re-summarize ROOT [type=%s, id=%d, branches=%d]", rootType.Name, root.ID, len(allBranches))

	root = r.normalizeInsightIfNeeded(memoryID, root)
	existingPoints := root.Points
	if existingPoints == nil {
		existingPoints = []memind.InsightPoint{}
	}

	assist := resolveRootAssist(r.graph, memoryID, rootType, allBranches)
	orderedBranches := assist.OrderedBranchInsights
	additionalCtx := normalizeAdditionalContext(assist.AdditionalContext)

	fullResp, err := r.generator.GenerateRootSynthesis(rootType, existingPoints, orderedBranches, config.RootTargetTokens, additionalCtx, language)
	if err != nil || fullResp == nil || len(fullResp.Points) == 0 {
		log.Printf("ROOT re-summarize: LLM result empty [type=%s]", rootType.Name)
		return root
	}

	points := r.evidence.NormalizeRootPoints(
		r.idMgr.ReusePointIDsForFullRewrite(existingPoints, fullResp.Points),
		orderedBranches,
	)
	return r.embedAndSave(memoryID, root, points, memind.TierRoot)
}

// normalizeInsightIfNeeded - 确保洞察的 pointId 已归一化
func (r *TreeReorganizer) normalizeInsightIfNeeded(memoryID memind.MemoryId, ins *memind.MemoryInsight) *memind.MemoryInsight {
	if ins == nil || len(ins.Points) == 0 {
		return ins
	}
	normalized := r.idMgr.NormalizePersistedPoints(ins.Points)
	if pointListsEqual(normalized, ins.Points) {
		return ins
	}
	updated := copyInsight(ins)
	updated.Points = normalized
	updated.UpdatedAt = time.Now()
	_ = r.store.Insights().UpsertInsights(memoryID, []*memind.MemoryInsight{updated})
	return updated
}

// embedAndSave - 保存并计算 embedding
func (r *TreeReorganizer) embedAndSave(memoryID memind.MemoryId, insight *memind.MemoryInsight, points []memind.InsightPoint, tier memind.InsightTier) *memind.MemoryInsight {
	content := pointsContent(points)
	var embedding []float32
	if r.vector != nil {
		if emb, err := r.vector.Store(memoryID, content, nil); err == nil {
			_ = emb
		}
	}

	now := time.Now()
	updated := copyInsight(insight)
	updated.Points = points
	updated.SummaryEmbedding = embedding
	updated.LastReasonedAt = &now
	updated.UpdatedAt = now
	updated.Version = insight.Version + 1
	_ = r.store.Insights().UpsertInsights(memoryID, []*memind.MemoryInsight{updated})
	return updated
}

// embedAndSaveIfChanged - 仅在有变更时保存
func (r *TreeReorganizer) embedAndSaveIfChanged(memoryID memind.MemoryId, insight *memind.MemoryInsight, points []memind.InsightPoint, tier memind.InsightTier) *memind.MemoryInsight {
	if !pointsChanged(insight, points) {
		return insight
	}
	return r.embedAndSave(memoryID, insight, points, tier)
}

// getOrCreateWaitGroup - 获取或创建 memoryId 对应的 WaitGroup
func (r *TreeReorganizer) getOrCreateWaitGroup(memoryID memind.MemoryId) *sync.WaitGroup {
	val, _ := r.pendingRoots.LoadOrStore(memoryID.Identifier(), &sync.WaitGroup{})
	return val.(*sync.WaitGroup)
}

// ===== Utility =====

func branchBubbleKey(memoryID memind.MemoryId, typeName string) string {
	return memoryID.Identifier() + "::" + typeName
}

func rootBubbleKey(memoryID memind.MemoryId, rootTypeName string) string {
	return memoryID.Identifier() + "::root::" + rootTypeName
}

func inferScope(insights []*memind.MemoryInsight) memind.MemoryScope {
	var s memind.MemoryScope
	for i, ins := range insights {
		if i == 0 {
			s = ins.Scope
		} else if ins.Scope != s {
			return memind.ScopeUser
		}
	}
	return s
}

func resolveTreeConfig(t memind.MemoryInsightType) memind.InsightTreeConfig {
	return memind.DefaultInsightTreeConfig()
}

func pointsContent(points []memind.InsightPoint) string {
	var s string
	for i, p := range points {
		if i > 0 {
			s += "\n"
		}
		s += p.Content
	}
	return s
}

func pointsChanged(insight *memind.MemoryInsight, points []memind.InsightPoint) bool {
	if len(insight.Points) != len(points) {
		return true
	}
	for i := range insight.Points {
		if insight.Points[i].Content != points[i].Content {
			return true
		}
	}
	return false
}

func pointListsEqual(a, b []memind.InsightPoint) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].PointID != b[i].PointID || a[i].Content != b[i].Content {
			return false
		}
	}
	return true
}

func copyInsight(ins *memind.MemoryInsight) *memind.MemoryInsight {
	c := *ins
	c.ChildInsightIDs = append([]int64{}, ins.ChildInsightIDs...)
	c.Points = append([]memind.InsightPoint{}, ins.Points...)
	if ins.ParentInsightID != nil {
		v := *ins.ParentInsightID
		c.ParentInsightID = &v
	}
	return &c
}

func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}
