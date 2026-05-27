package vector

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	memind "github.com/openmemind/memind-go"
)

// vectorEntry - 内存向量条目
type vectorEntry struct {
	VectorID  string
	Text      string
	Embedding []float32
	CreatedAt time.Time
	Metadata  map[string]any
}

// InMemoryVectorStore - 内存版向量存储，内置基于哈希的嵌入引擎
// 采用 128 维浮点向量，cosine 相似度搜索
type InMemoryVectorStore struct {
	mu      sync.RWMutex
	vectors map[memind.MemoryId]map[string]*vectorEntry
}

// NewInMemoryVectorStore - 创建内存向量存储
func NewInMemoryVectorStore() *InMemoryVectorStore {
	return &InMemoryVectorStore{
		vectors: make(map[memind.MemoryId]map[string]*vectorEntry),
	}
}

// Embed - 对文本计算 128 维哈希嵌入向量
func (s *InMemoryVectorStore) Embed(text string) ([]float32, error) {
	return hashEmbed(text, 128), nil
}

// EmbedAll - 批量计算嵌入向量
func (s *InMemoryVectorStore) EmbedAll(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, t := range texts {
		result[i] = hashEmbed(t, 128)
	}
	return result, nil
}

// Store - 存储文本向量，返回自动生成的向量 ID
func (s *InMemoryVectorStore) Store(memoryID memind.MemoryId, text string, metadata map[string]any) (string, error) {
	embedding, err := s.Embed(text)
	if err != nil {
		return "", err
	}

	vectorID := fmt.Sprintf("vec_%d", time.Now().UnixNano())

	entry := &vectorEntry{
		VectorID:  vectorID,
		Text:      text,
		Embedding: embedding,
		CreatedAt: time.Now(),
		Metadata:  metadata,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vectors[memoryID] == nil {
		s.vectors[memoryID] = make(map[string]*vectorEntry)
	}
	s.vectors[memoryID][vectorID] = entry
	return vectorID, nil
}

// StoreWithID - 使用指定 ID 存储向量
func (s *InMemoryVectorStore) StoreWithID(memoryID memind.MemoryId, vectorID string, text string, metadata map[string]any) error {
	embedding, err := s.Embed(text)
	if err != nil {
		return err
	}

	entry := &vectorEntry{
		VectorID:  vectorID,
		Text:      text,
		Embedding: embedding,
		CreatedAt: time.Now(),
		Metadata:  metadata,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vectors[memoryID] == nil {
		s.vectors[memoryID] = make(map[string]*vectorEntry)
	}
	s.vectors[memoryID][vectorID] = entry
	return nil
}

// StoreBatch - 批量存储文本向量
func (s *InMemoryVectorStore) StoreBatch(memoryID memind.MemoryId, texts []string, metadatas []map[string]any) ([]string, error) {
	ids := make([]string, len(texts))
	for i, text := range texts {
		var meta map[string]any
		if i < len(metadatas) {
			meta = metadatas[i]
		}
		id, err := s.Store(memoryID, text, meta)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

// Delete - 删除指定向量
func (s *InMemoryVectorStore) Delete(memoryID memind.MemoryId, vectorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vectors[memoryID] != nil {
		delete(s.vectors[memoryID], vectorID)
	}
	return nil
}

// DeleteBatch - 批量删除向量
func (s *InMemoryVectorStore) DeleteBatch(memoryID memind.MemoryId, vectorIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vectors[memoryID] != nil {
		for _, id := range vectorIDs {
			delete(s.vectors[memoryID], id)
		}
	}
	return nil
}

// Search - 执行向量相似度搜索，返回 topK 结果
func (s *InMemoryVectorStore) Search(memoryID memind.MemoryId, query string, topK int) ([]SearchResult, error) {
	return s.SearchWithFilter(memoryID, query, topK, 0, nil)
}

// SearchWithFilter - 带过滤条件的向量搜索
func (s *InMemoryVectorStore) SearchWithFilter(memoryID memind.MemoryId, query string, topK int, minScore float64, filter map[string]any) ([]SearchResult, error) {
	queryEmb, err := s.Embed(query)
	if err != nil {
		return nil, err
	}

	s.mu.RLock()
	entries, ok := s.vectors[memoryID]
	s.mu.RUnlock()
	if !ok || len(entries) == 0 {
		return []SearchResult{}, nil
	}

	type scored struct {
		entry *vectorEntry
		score float32
	}
	var results []scored

	for _, e := range entries {
		if filter != nil && !matchFilter(e.Metadata, filter) {
			continue
		}
		score := cosineSimilarity(queryEmb, e.Embedding)
		if minScore > 0 && float64(score) < minScore {
			continue
		}
		results = append(results, scored{e, score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	out := make([]SearchResult, len(results))
	for i, r := range results {
		out[i] = SearchResult{
			VectorID: r.entry.VectorID,
			Text:     r.entry.Text,
			Score:    r.score,
			Metadata: r.entry.Metadata,
		}
	}
	return out, nil
}

// matchFilter - 检查元数据是否满足过滤条件
func matchFilter(meta, filter map[string]any) bool {
	for k, v := range filter {
		if mv, ok := meta[k]; !ok || mv != v {
			return false
		}
	}
	return true
}

// hashEmbed - 基于哈希的文本嵌入算法，输出指定维度的归一化向量
// 对文本的每个 n-gram 计算散列值，映射到向量维度上
func hashEmbed(text string, dims int) []float32 {
	vec := make([]float32, dims)
	text = strings.ToLower(text)
	text = strings.TrimSpace(text)
	if text == "" {
		return vec
	}

	seen := make(map[uint64]bool)
	count := 0

	// 对每个位置计算字符级哈希
	for i := 0; i < len(text); i++ {
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(i))
		hash := sha256.Sum256(append(buf[:], []byte(text)...))
		h := binary.LittleEndian.Uint64(hash[:8])
		idx := h % uint64(dims)
		if !seen[h] {
			seen[h] = true
			vec[idx] += 1.0
			count++
		}
	}

	// L2 归一化
	if count > 0 {
		var norm float64
		for _, v := range vec {
			norm += float64(v * v)
		}
		norm = math.Sqrt(norm)
		if norm > 0 {
			for i := range vec {
				vec[i] = float32(float64(vec[i]) / norm)
			}
		}
	}

	return vec
}

// cosineSimilarity - 计算两个向量的余弦相似度
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}
