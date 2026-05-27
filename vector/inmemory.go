package vector

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"

	memind "github.com/openmemind/memind-go"
)

type InMemoryVectorStore struct {
	mu       sync.RWMutex
	vectors  map[string]map[string]*storedVector
}

type storedVector struct {
	vectorID string
	text     string
	embedding []float32
	metadata map[string]any
}

func NewInMemoryVectorStore() *InMemoryVectorStore {
	return &InMemoryVectorStore{
		vectors: make(map[string]map[string]*storedVector),
	}
}

func (s *InMemoryVectorStore) key(memID memind.MemoryId) string {
	return memID.Identifier()
}

func (s *InMemoryVectorStore) Store(memoryID memind.MemoryId, text string, metadata map[string]any) (string, error) {
	embed, err := s.Embed(text)
	if err != nil {
		return "", err
	}
	vectorID := fmt.Sprintf("vec-%d", rand.Int63())
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if s.vectors[k] == nil {
		s.vectors[k] = make(map[string]*storedVector)
	}
	s.vectors[k][vectorID] = &storedVector{
		vectorID:  vectorID,
		text:      text,
		embedding: embed,
		metadata:  metadata,
	}
	return vectorID, nil
}

func (s *InMemoryVectorStore) StoreWithID(memoryID memind.MemoryId, vectorID string, text string, metadata map[string]any) error {
	embed, err := s.Embed(text)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if s.vectors[k] == nil {
		s.vectors[k] = make(map[string]*storedVector)
	}
	s.vectors[k][vectorID] = &storedVector{
		vectorID:  vectorID,
		text:      text,
		embedding: embed,
		metadata:  metadata,
	}
	return nil
}

func (s *InMemoryVectorStore) StoreBatch(memoryID memind.MemoryId, texts []string, metadatas []map[string]any) ([]string, error) {
	embeds, err := s.EmbedAll(texts)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if s.vectors[k] == nil {
		s.vectors[k] = make(map[string]*storedVector)
	}
	ids := make([]string, len(texts))
	for i, text := range texts {
		vectorID := fmt.Sprintf("vec-%d", rand.Int63())
		var meta map[string]any
		if i < len(metadatas) {
			meta = metadatas[i]
		}
		s.vectors[k][vectorID] = &storedVector{
			vectorID:  vectorID,
			text:      text,
			embedding: embeds[i],
			metadata:  meta,
		}
		ids[i] = vectorID
	}
	return ids, nil
}

func (s *InMemoryVectorStore) Delete(memoryID memind.MemoryId, vectorID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if m, ok := s.vectors[k]; ok {
		delete(m, vectorID)
	}
	return nil
}

func (s *InMemoryVectorStore) DeleteBatch(memoryID memind.MemoryId, vectorIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := s.key(memoryID)
	if m, ok := s.vectors[k]; ok {
		for _, id := range vectorIDs {
			delete(m, id)
		}
	}
	return nil
}

func (s *InMemoryVectorStore) Search(memoryID memind.MemoryId, query string, topK int) ([]SearchResult, error) {
	return s.SearchWithFilter(memoryID, query, topK, 0, nil)
}

func (s *InMemoryVectorStore) SearchWithFilter(memoryID memind.MemoryId, query string, topK int, minScore float64, filter map[string]any) ([]SearchResult, error) {
	queryEmbed, err := s.Embed(query)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	k := s.key(memoryID)
	var results []SearchResult
	m, ok := s.vectors[k]
	if !ok {
		return nil, nil
	}
	for _, sv := range m {
		score := cosineSimilarity(queryEmbed, sv.embedding)
		if float64(score) < minScore {
			continue
		}
		if filter != nil {
			match := true
			for fk, fv := range filter {
				if sv.metadata[fk] != fv {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		results = append(results, SearchResult{
			VectorID: sv.vectorID,
			Text:     sv.text,
			Score:    score,
			Metadata: sv.metadata,
		})
	}
	sortByScoreDesc(results)
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (s *InMemoryVectorStore) Embed(text string) ([]float32, error) {
	return embed(text), nil
}

func (s *InMemoryVectorStore) EmbedAll(texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, t := range texts {
		result[i] = embed(t)
	}
	return result, nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

func sortByScoreDesc(results []SearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

func embed(text string) []float32 {
	dim := 128
	vec := make([]float32, dim)
	words := strings.Fields(text)
	wordVec := make([]float32, dim)
	count := 0
	for _, word := range words {
		h := hashString(word)
		for i := 0; i < dim; i++ {
			wordVec[i] = float32(h % 9973)
			h = h*31 + 1
		}
		normalize(wordVec)
		for i := range vec {
			vec[i] += wordVec[i]
		}
		count++
	}
	if count > 0 {
		for i := range vec {
			vec[i] /= float32(count)
		}
	}
	normalize(vec)
	return vec
}

func hashString(s string) int64 {
	var h int64 = 0
	for _, c := range s {
		h = h*31 + int64(c)
	}
	return h
}

func normalize(v []float32) {
	var sumSq float64
	for _, val := range v {
		sumSq += float64(val) * float64(val)
	}
	if sumSq == 0 {
		return
	}
	norm := float32(math.Sqrt(sumSq))
	for i := range v {
		v[i] /= norm
	}
}
