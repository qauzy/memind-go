package store

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	memind "github.com/openmemind/memind-go"
)

type InMemoryRawDataOps struct {
	mu       sync.RWMutex
	rawData  map[string]map[string]*memind.MemoryRawData
	byContent map[string]map[string][]*memind.MemoryRawData
}

func NewInMemoryRawDataOps() *InMemoryRawDataOps {
	return &InMemoryRawDataOps{
		rawData:   make(map[string]map[string]*memind.MemoryRawData),
		byContent: make(map[string]map[string][]*memind.MemoryRawData),
	}
}

func (s *InMemoryRawDataOps) key(memID memind.MemoryId) string { return memID.Identifier() }

func (s *InMemoryRawDataOps) UpsertRawData(memoryID memind.MemoryId, rawData []*memind.MemoryRawData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if s.rawData[k] == nil {
		s.rawData[k] = make(map[string]*memind.MemoryRawData)
	}
	if s.byContent[k] == nil {
		s.byContent[k] = make(map[string][]*memind.MemoryRawData)
	}
	for _, rd := range rawData {
		if rd.ID == "" {
			rd.ID = fmt.Sprintf("rd-%d", rand.Int63())
		}
		if rd.CreatedAt.IsZero() {
			rd.CreatedAt = time.Now()
		}
		s.rawData[k][rd.ID] = rd
		if rd.ContentID != "" {
			s.byContent[k][rd.ContentID] = append(s.byContent[k][rd.ContentID], rd)
		}
	}
	return nil
}

func (s *InMemoryRawDataOps) GetRawData(memoryID memind.MemoryId, rawDataID string) (*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	if m, ok := s.rawData[k]; ok {
		if rd, ok := m[rawDataID]; ok {
			return rd, nil
		}
	}
	return nil, memind.ErrRawDataNotFound
}

func (s *InMemoryRawDataOps) GetRawDataByContentID(memoryID memind.MemoryId, contentID string) (*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	if m, ok := s.byContent[k]; ok {
		if list, ok := m[contentID]; ok && len(list) > 0 {
			return list[0], nil
		}
	}
	return nil, memind.ErrRawDataNotFound
}

func (s *InMemoryRawDataOps) ListRawDataByContentID(memoryID memind.MemoryId, contentID string) ([]*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	if m, ok := s.byContent[k]; ok {
		if list, ok := m[contentID]; ok {
			return list, nil
		}
	}
	return nil, nil
}

func (s *InMemoryRawDataOps) ListRawData(memoryID memind.MemoryId) ([]*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	var result []*memind.MemoryRawData
	if m, ok := s.rawData[k]; ok {
		for _, rd := range m {
			result = append(result, rd)
		}
	}
	return result, nil
}

func (s *InMemoryRawDataOps) PollRawDataWithoutVector(memoryID memind.MemoryId, limit int, minAge time.Duration) ([]*memind.MemoryRawData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	var result []*memind.MemoryRawData
	cutoff := time.Now().Add(-minAge)
	if m, ok := s.rawData[k]; ok {
		for _, rd := range m {
			if rd.CaptionVectorID == "" && rd.CreatedAt.Before(cutoff) {
				result = append(result, rd)
				if len(result) >= limit {
					break
				}
			}
		}
	}
	return result, nil
}

func (s *InMemoryRawDataOps) UpdateRawDataVectorIDs(memoryID memind.MemoryId, vectorIDs map[string]string, metadataPatch map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if m, ok := s.rawData[k]; ok {
		for id, vecID := range vectorIDs {
			if rd, ok := m[id]; ok {
				rd.CaptionVectorID = vecID
				for k, v := range metadataPatch {
					if rd.Metadata == nil {
						rd.Metadata = make(map[string]any)
					}
					rd.Metadata[k] = v
				}
			}
		}
	}
	return nil
}

func (s *InMemoryRawDataOps) DeleteRawData(memoryID memind.MemoryId, rawDataID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if m, ok := s.rawData[k]; ok {
		if rd, ok := m[rawDataID]; ok {
			delete(m, rawDataID)
			if rd.ContentID != "" {
				if cm, ok := s.byContent[k]; ok {
					list := cm[rd.ContentID]
					for i, item := range list {
						if item.ID == rawDataID {
							cm[rd.ContentID] = append(list[:i], list[i+1:]...)
							break
						}
					}
				}
			}
		}
	}
	return nil
}

type InMemoryItemOps struct {
	mu     sync.RWMutex
	items  map[string]map[int64]*memind.MemoryItem
	byHash map[string]map[string]*memind.MemoryItem
	seq    map[string]int64
}

func NewInMemoryItemOps() *InMemoryItemOps {
	return &InMemoryItemOps{
		items:  make(map[string]map[int64]*memind.MemoryItem),
		byHash: make(map[string]map[string]*memind.MemoryItem),
		seq:    make(map[string]int64),
	}
}

func (s *InMemoryItemOps) key(memID memind.MemoryId) string { return memID.Identifier() }

func (s *InMemoryItemOps) UpsertItems(memoryID memind.MemoryId, items []*memind.MemoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if s.items[k] == nil {
		s.items[k] = make(map[int64]*memind.MemoryItem)
	}
	if s.byHash[k] == nil {
		s.byHash[k] = make(map[string]*memind.MemoryItem)
	}
	for _, item := range items {
		if item.ID == 0 {
			s.seq[k]++
			item.ID = s.seq[k]
		}
		if item.CreatedAt.IsZero() {
			item.CreatedAt = time.Now()
		}
		s.items[k][item.ID] = item
		if item.ContentHash != "" {
			s.byHash[k][item.ContentHash] = item
		}
	}
	return nil
}

func (s *InMemoryItemOps) GetItem(memoryID memind.MemoryId, itemID int64) (*memind.MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	if m, ok := s.items[k]; ok {
		if item, ok := m[itemID]; ok {
			return item, nil
		}
	}
	return nil, memind.ErrItemNotFound
}

func (s *InMemoryItemOps) ListItems(memoryID memind.MemoryId) ([]*memind.MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	var result []*memind.MemoryItem
	if m, ok := s.items[k]; ok {
		for _, item := range m {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *InMemoryItemOps) DeleteItems(memoryID memind.MemoryId, itemIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if m, ok := s.items[k]; ok {
		idSet := make(map[int64]bool)
		for _, id := range itemIDs {
			idSet[id] = true
		}
		for _, id := range itemIDs {
			if item, ok := m[id]; ok {
				delete(m, id)
				if item.ContentHash != "" {
					if hm, ok := s.byHash[k]; ok {
						delete(hm, item.ContentHash)
					}
				}
			}
		}
	}
	return nil
}

func (s *InMemoryItemOps) GetItemByHash(memoryID memind.MemoryId, hash string) (*memind.MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	if m, ok := s.byHash[k]; ok {
		if item, ok := m[hash]; ok {
			return item, nil
		}
	}
	return nil, memind.ErrItemNotFound
}

type InMemoryInsightOps struct {
	mu          sync.RWMutex
	insights    map[string]map[int64]*memind.MemoryInsight
	types       map[string]*memind.MemoryInsightType
	seq         map[string]int64
}

func NewInMemoryInsightOps() *InMemoryInsightOps {
	ops := &InMemoryInsightOps{
		insights: make(map[string]map[int64]*memind.MemoryInsight),
		types:    make(map[string]*memind.MemoryInsightType),
		seq:      make(map[string]int64),
	}
	for _, t := range defaultInsightTypes() {
		ops.types[t.Name] = t
	}
	return ops
}

func (s *InMemoryInsightOps) key(memID memind.MemoryId) string { return memID.Identifier() }

func (s *InMemoryInsightOps) UpsertInsightTypes(types []*memind.MemoryInsightType) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range types {
		s.types[t.Name] = t
	}
	return nil
}

func (s *InMemoryInsightOps) GetInsightType(name string) (*memind.MemoryInsightType, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if t, ok := s.types[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("insight type not found: %s", name)
}

func (s *InMemoryInsightOps) ListInsightTypes() ([]*memind.MemoryInsightType, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*memind.MemoryInsightType
	for _, t := range s.types {
		result = append(result, t)
	}
	return result, nil
}

func (s *InMemoryInsightOps) UpsertInsights(memoryID memind.MemoryId, insights []*memind.MemoryInsight) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if s.insights[k] == nil {
		s.insights[k] = make(map[int64]*memind.MemoryInsight)
	}
	for _, ins := range insights {
		if ins.ID == 0 {
			s.seq[k]++
			ins.ID = s.seq[k]
		}
		if ins.CreatedAt.IsZero() {
			ins.CreatedAt = time.Now()
		}
		ins.UpdatedAt = time.Now()
		s.insights[k][ins.ID] = ins
	}
	return nil
}

func (s *InMemoryInsightOps) GetInsight(memoryID memind.MemoryId, insightID int64) (*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	if m, ok := s.insights[k]; ok {
		if ins, ok := m[insightID]; ok {
			return ins, nil
		}
	}
	return nil, memind.ErrInsightNotFound
}

func (s *InMemoryInsightOps) ListInsights(memoryID memind.MemoryId) ([]*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	var result []*memind.MemoryInsight
	if m, ok := s.insights[k]; ok {
		for _, ins := range m {
			result = append(result, ins)
		}
	}
	return result, nil
}

func (s *InMemoryInsightOps) GetInsightsByType(memoryID memind.MemoryId, insightType string) ([]*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	var result []*memind.MemoryInsight
	if m, ok := s.insights[k]; ok {
		for _, ins := range m {
			if ins.Type == insightType {
				result = append(result, ins)
			}
		}
	}
	return result, nil
}

func (s *InMemoryInsightOps) GetInsightsByTier(memoryID memind.MemoryId, tier memind.InsightTier) ([]*memind.MemoryInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	var result []*memind.MemoryInsight
	if m, ok := s.insights[k]; ok {
		for _, ins := range m {
			if ins.Tier == tier {
				result = append(result, ins)
			}
		}
	}
	return result, nil
}

func (s *InMemoryInsightOps) DeleteInsights(memoryID memind.MemoryId, insightIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if m, ok := s.insights[k]; ok {
		for _, id := range insightIDs {
			delete(m, id)
		}
	}
	return nil
}

type InMemoryStore struct {
	RawDataOps  *InMemoryRawDataOps
	ItemOps     *InMemoryItemOps
	InsightOps  *InMemoryInsightOps
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		RawDataOps:  NewInMemoryRawDataOps(),
		ItemOps:     NewInMemoryItemOps(),
		InsightOps:  NewInMemoryInsightOps(),
	}
}

func (s *InMemoryStore) RawData() RawDataOperations   { return s.RawDataOps }
func (s *InMemoryStore) Items() ItemOperations         { return s.ItemOps }
func (s *InMemoryStore) Insights() InsightOperations   { return s.InsightOps }

func defaultInsightTypes() []*memind.MemoryInsightType {
	now := time.Now()
	return []*memind.MemoryInsightType{
		{
			ID: 1, Name: "identity", Scope: memind.ScopeUser,
			Categories: []string{"PROFILE"}, TargetTokens: 300,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 2, Name: "preferences", Scope: memind.ScopeUser,
			Categories: []string{"BEHAVIOR"}, TargetTokens: 300,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 3, Name: "relationships", Scope: memind.ScopeUser,
			Categories: []string{"BEHAVIOR", "EVENT"}, TargetTokens: 300,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 4, Name: "experiences", Scope: memind.ScopeUser,
			Categories: []string{"EVENT"}, TargetTokens: 400,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 5, Name: "behavior", Scope: memind.ScopeUser,
			Categories: []string{"BEHAVIOR"}, TargetTokens: 300,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 6, Name: "directives", Scope: memind.ScopeAgent,
			Categories: []string{"DIRECTIVE"}, TargetTokens: 400,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 7, Name: "playbooks", Scope: memind.ScopeAgent,
			Categories: []string{"PLAYBOOK"}, TargetTokens: 500,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 8, Name: "resolutions", Scope: memind.ScopeAgent,
			Categories: []string{"RESOLUTION"}, TargetTokens: 400,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 9, Name: "profile", Scope: memind.ScopeUser,
			Categories: []string{"PROFILE", "BEHAVIOR", "EVENT"}, TargetTokens: 800,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: 10, Name: "interaction", Scope: memind.ScopeAgent,
			Categories: []string{"TOOL", "DIRECTIVE", "PLAYBOOK", "RESOLUTION"}, TargetTokens: 800,
			LastUpdatedAt: now, CreatedAt: now, UpdatedAt: now,
		},
	}
}
