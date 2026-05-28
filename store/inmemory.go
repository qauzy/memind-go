package store

import (
	"sync"
	"time"

	memind "github.com/openmemind/memind-go"
)

// inMemRawDataOps - 内存版原始数据存储
type inMemRawDataOps struct {
	mu   sync.RWMutex
	data map[memind.MemoryId]map[string]*memind.MemoryRawData
}

func newInMemRawDataOps() *inMemRawDataOps {
	return &inMemRawDataOps{
		data: make(map[memind.MemoryId]map[string]*memind.MemoryRawData),
	}
}

func (s *inMemRawDataOps) UpsertRawData(memoryID memind.MemoryId, rawData []*memind.MemoryRawData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data[memoryID] == nil {
		s.data[memoryID] = make(map[string]*memind.MemoryRawData)
	}
	for _, rd := range rawData {
		s.data[memoryID][rd.ID] = rd
	}
	return nil
}

func (s *inMemRawDataOps) GetRawData(memoryID memind.MemoryId, rawDataID string) (*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data[memoryID] == nil {
		return nil, nil
	}
	return s.data[memoryID][rawDataID], nil
}

func (s *inMemRawDataOps) GetRawDataByContentID(memoryID memind.MemoryId, contentID string) (*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.data[memoryID] == nil {
		return nil, nil
	}
	for _, rd := range s.data[memoryID] {
		if rd.ContentID == contentID {
			return rd, nil
		}
	}
	return nil, nil
}

func (s *inMemRawDataOps) ListRawDataByContentID(memoryID memind.MemoryId, contentID string) ([]*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*memind.MemoryRawData
	if s.data[memoryID] == nil {
		return result, nil
	}
	for _, rd := range s.data[memoryID] {
		if rd.ContentID == contentID {
			result = append(result, rd)
		}
	}
	return result, nil
}

func (s *inMemRawDataOps) ListRawData(memoryID memind.MemoryId) ([]*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*memind.MemoryRawData
	if s.data[memoryID] == nil {
		return result, nil
	}
	for _, rd := range s.data[memoryID] {
		result = append(result, rd)
	}
	return result, nil
}

func (s *inMemRawDataOps) PollRawDataWithoutVector(memoryID memind.MemoryId, limit int, minAge time.Duration) ([]*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*memind.MemoryRawData
	if s.data[memoryID] == nil {
		return result, nil
	}
	cutoff := time.Now().Add(-minAge)
	for _, rd := range s.data[memoryID] {
		if rd.CaptionVectorID == "" && rd.CreatedAt.Before(cutoff) {
			result = append(result, rd)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (s *inMemRawDataOps) UpdateRawDataVectorIDs(memoryID memind.MemoryId, vectorIDs map[string]string, metadataPatch map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, vectorID := range vectorIDs {
		if rd, ok := s.data[memoryID][id]; ok {
			rd.CaptionVectorID = vectorID
			if metadataPatch != nil {
				if rd.Metadata == nil {
					rd.Metadata = make(map[string]any)
				}
				for k, v := range metadataPatch {
					rd.Metadata[k] = v
				}
			}
		}
	}
	return nil
}

func (s *inMemRawDataOps) DeleteRawData(memoryID memind.MemoryId, rawDataID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data[memoryID] != nil {
		delete(s.data[memoryID], rawDataID)
	}
	return nil
}

// inMemItemOps - 内存版记忆条目存储
type inMemItemOps struct {
	mu     sync.RWMutex
	items  map[memind.MemoryId]map[int64]*memind.MemoryItem
	nextID map[memind.MemoryId]int64
}

func newInMemItemOps() *inMemItemOps {
	return &inMemItemOps{
		items:  make(map[memind.MemoryId]map[int64]*memind.MemoryItem),
		nextID: make(map[memind.MemoryId]int64),
	}
}

func (s *inMemItemOps) UpsertItems(memoryID memind.MemoryId, items []*memind.MemoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items[memoryID] == nil {
		s.items[memoryID] = make(map[int64]*memind.MemoryItem)
	}
	for _, item := range items {
		if item.ID == 0 {
			s.nextID[memoryID]++
			item.ID = s.nextID[memoryID]
		}
		s.items[memoryID][item.ID] = item
	}
	return nil
}

func (s *inMemItemOps) GetItem(memoryID memind.MemoryId, itemID int64) (*memind.MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.items[memoryID] == nil {
		return nil, nil
	}
	return s.items[memoryID][itemID], nil
}

func (s *inMemItemOps) ListItems(memoryID memind.MemoryId) ([]*memind.MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*memind.MemoryItem
	if s.items[memoryID] == nil {
		return result, nil
	}
	for _, item := range s.items[memoryID] {
		result = append(result, item)
	}
	return result, nil
}

func (s *inMemItemOps) DeleteItems(memoryID memind.MemoryId, itemIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.items[memoryID] == nil {
		return nil
	}
	for _, id := range itemIDs {
		delete(s.items[memoryID], id)
	}
	return nil
}

func (s *inMemItemOps) GetItemByHash(memoryID memind.MemoryId, hash string) (*memind.MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.items[memoryID] == nil {
		return nil, nil
	}
	for _, item := range s.items[memoryID] {
		if item.ContentHash == hash {
			return item, nil
		}
	}
	return nil, nil
}

// inMemInsightOps - 内存版洞察存储
type inMemInsightOps struct {
	mu       sync.RWMutex
	types    map[string]*memind.MemoryInsightType
	insights map[memind.MemoryId]map[int64]*memind.MemoryInsight
	nextID   map[memind.MemoryId]int64
}

func newInMemInsightOps() *inMemInsightOps {
	ops := &inMemInsightOps{
		types:    make(map[string]*memind.MemoryInsightType),
		insights: make(map[memind.MemoryId]map[int64]*memind.MemoryInsight),
		nextID:   make(map[memind.MemoryId]int64),
	}
	// 插入默认洞察类型（与 SQL 存储保持一致）
	ops.UpsertInsightTypes(defaultInsightTypes())
	return ops
}

// defaultInsightTypes - 返回默认洞察类型列表
func defaultInsightTypes() []*memind.MemoryInsightType {
	now := time.Now()
	types := []*memind.MemoryInsightType{
		{Name: "identity", Scope: memind.ScopeUser, Categories: []string{"PROFILE"}, TargetTokens: 300, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "preferences", Scope: memind.ScopeUser, Categories: []string{"PROFILE"}, TargetTokens: 300, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "relationships", Scope: memind.ScopeUser, Categories: []string{"PROFILE"}, TargetTokens: 300, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "experiences", Scope: memind.ScopeUser, Categories: []string{"EVENT"}, TargetTokens: 400, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "behavior", Scope: memind.ScopeUser, Categories: []string{"BEHAVIOR"}, TargetTokens: 300, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "directives", Scope: memind.ScopeAgent, Categories: []string{"DIRECTIVE"}, TargetTokens: 400, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "playbooks", Scope: memind.ScopeAgent, Categories: []string{"PLAYBOOK"}, TargetTokens: 500, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "resolutions", Scope: memind.ScopeAgent, Categories: []string{"RESOLUTION"}, TargetTokens: 400, AnalysisMode: memind.AnalysisModeBranch, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "profile", Scope: memind.ScopeUser, Categories: []string{"PROFILE", "BEHAVIOR", "EVENT"}, TargetTokens: 800, AnalysisMode: memind.AnalysisModeRoot, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
		{Name: "interaction", Scope: memind.ScopeAgent, Categories: []string{"TOOL", "DIRECTIVE", "PLAYBOOK", "RESOLUTION"}, TargetTokens: 800, AnalysisMode: memind.AnalysisModeRoot, LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now},
	}
	for _, t := range types {
		if t.ID == 0 {
			t.ID = int64(1 + len(types))
		}
	}
	return types
}

func (s *inMemInsightOps) UpsertInsightTypes(types []*memind.MemoryInsightType) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range types {
		s.types[t.Name] = t
	}
	return nil
}

func (s *inMemInsightOps) GetInsightType(name string) (*memind.MemoryInsightType, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.types[name], nil
}

func (s *inMemInsightOps) ListInsightTypes() ([]*memind.MemoryInsightType, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*memind.MemoryInsightType
	for _, t := range s.types {
		result = append(result, t)
	}
	return result, nil
}

func (s *inMemInsightOps) UpsertInsights(memoryID memind.MemoryId, insights []*memind.MemoryInsight) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.insights[memoryID] == nil {
		s.insights[memoryID] = make(map[int64]*memind.MemoryInsight)
	}
	for _, ins := range insights {
		if ins.ID == 0 {
			s.nextID[memoryID]++
			ins.ID = s.nextID[memoryID]
		}
		s.insights[memoryID][ins.ID] = ins
	}
	return nil
}

func (s *inMemInsightOps) GetInsight(memoryID memind.MemoryId, insightID int64) (*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.insights[memoryID] == nil {
		return nil, nil
	}
	return s.insights[memoryID][insightID], nil
}

func (s *inMemInsightOps) ListInsights(memoryID memind.MemoryId) ([]*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*memind.MemoryInsight
	if s.insights[memoryID] == nil {
		return result, nil
	}
	for _, ins := range s.insights[memoryID] {
		result = append(result, ins)
	}
	return result, nil
}

func (s *inMemInsightOps) GetInsightsByType(memoryID memind.MemoryId, insightType string) ([]*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*memind.MemoryInsight
	if s.insights[memoryID] == nil {
		return result, nil
	}
	for _, ins := range s.insights[memoryID] {
		if string(ins.Type) == insightType {
			result = append(result, ins)
		}
	}
	return result, nil
}

func (s *inMemInsightOps) GetInsightsByTier(memoryID memind.MemoryId, tier memind.InsightTier) ([]*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*memind.MemoryInsight
	if s.insights[memoryID] == nil {
		return result, nil
	}
	for _, ins := range s.insights[memoryID] {
		if ins.Tier == tier {
			result = append(result, ins)
		}
	}
	return result, nil
}

func (s *inMemInsightOps) GetBranchByType(memoryID memind.MemoryId, typeName string) (*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.insights[memoryID] == nil {
		return nil, nil
	}
	for _, ins := range s.insights[memoryID] {
		if ins.Type == typeName && ins.Tier == memind.TierBranch {
			return ins, nil
		}
	}
	return nil, nil
}

func (s *inMemInsightOps) GetRootByType(memoryID memind.MemoryId, rootTypeName string) (*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.insights[memoryID] == nil {
		return nil, nil
	}
	for _, ins := range s.insights[memoryID] {
		if ins.Type == rootTypeName && ins.Tier == memind.TierRoot {
			return ins, nil
		}
	}
	return nil, nil
}

func (s *inMemInsightOps) DeleteInsights(memoryID memind.MemoryId, insightIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.insights[memoryID] == nil {
		return nil
	}
	for _, id := range insightIDs {
		delete(s.insights[memoryID], id)
	}
	return nil
}

// InMemoryStore - 内存版 MemoryStore 实现，所有数据存储在 map 中
type InMemoryStore struct {
	rawDataOps *inMemRawDataOps
	itemOps    *inMemItemOps
	insightOps *inMemInsightOps
}

// NewInMemoryStore - 创建内存版存储
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		rawDataOps: newInMemRawDataOps(),
		itemOps:    newInMemItemOps(),
		insightOps: newInMemInsightOps(),
	}
}

func (s *InMemoryStore) RawData() RawDataOperations  { return s.rawDataOps }
func (s *InMemoryStore) Items() ItemOperations       { return s.itemOps }
func (s *InMemoryStore) Insights() InsightOperations { return s.insightOps }

// compile-time interface check
var _ MemoryStore = (*InMemoryStore)(nil)
