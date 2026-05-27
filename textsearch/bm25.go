package textsearch

import (
	"math"
	"strings"
	"sync"

	memind "github.com/openmemind/memind-go"
)

const (
	k1 = 1.5
	b  = 0.75
)

type indexEntry struct {
	docID    string
	text     string
	avgLen   float64
	totalDocs int
}

type bm25Store struct {
	docs   map[string]indexEntry
	terms  map[string]map[string]int
}

type InMemoryBM25Search struct {
	mu       sync.RWMutex
	indices  map[string]map[SearchTarget]*bm25Store
}

var _ MemoryTextSearch = (*InMemoryBM25Search)(nil)

func NewInMemoryBM25Search() *InMemoryBM25Search {
	return &InMemoryBM25Search{
		indices: make(map[string]map[SearchTarget]*bm25Store),
	}
}

func (s *InMemoryBM25Search) key(memID memind.MemoryId) string {
	return memID.Identifier()
}

func (s *InMemoryBM25Search) getStore(memoryID memind.MemoryId, target SearchTarget) *bm25Store {
	k := s.key(memoryID)
	if s.indices[k] == nil {
		s.indices[k] = make(map[SearchTarget]*bm25Store)
	}
	if s.indices[k][target] == nil {
		s.indices[k][target] = &bm25Store{
			docs:  make(map[string]indexEntry),
			terms: make(map[string]map[string]int),
		}
	}
	return s.indices[k][target]
}

func (s *InMemoryBM25Search) Search(memoryID memind.MemoryId, query string, topK int, target SearchTarget) ([]Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.getStore(memoryID, target)
	if len(st.docs) == 0 {
		return nil, nil
	}
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil, nil
	}
	totalDocs := len(st.docs)
	var avgLen float64
	for _, doc := range st.docs {
		avgLen += doc.avgLen
	}
	if totalDocs > 0 {
		avgLen /= float64(totalDocs)
	}
	var results []Result
	for docID, doc := range st.docs {
		score := 0.0
		for _, term := range queryTerms {
			df := len(st.terms[term])
			if df == 0 {
				continue
			}
			idf := math.Log(1 + (float64(totalDocs-df)+0.5)/(float64(df)+0.5))
			tf := 0
			if docTerms, ok := st.terms[term]; ok {
				tf = docTerms[docID]
			}
			docLen := doc.avgLen
			score += idf * (float64(tf) * (k1 + 1)) / (float64(tf) + k1*(1-b+b*docLen/avgLen))
		}
		if score > 0 {
			results = append(results, Result{
				DocumentID: docID,
				Text:       doc.text,
				Score:      score,
			})
		}
	}
	sortByScoreDescBM25(results)
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

func (s *InMemoryBM25Search) Index(memoryID memind.MemoryId, documentID string, text string, target SearchTarget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.getStore(memoryID, target)
	terms := tokenize(text)
	st.docs[documentID] = indexEntry{
		docID:    documentID,
		text:     text,
		avgLen:   float64(len(terms)),
		totalDocs: len(st.docs) + 1,
	}
	for _, term := range terms {
		if st.terms[term] == nil {
			st.terms[term] = make(map[string]int)
		}
		st.terms[term][documentID]++
	}
	for _, entry := range st.docs {
		entry.totalDocs = len(st.docs)
	}
	return nil
}

func (s *InMemoryBM25Search) IndexBatch(memoryID memind.MemoryId, documents map[string]string, target SearchTarget) error {
	for id, text := range documents {
		if err := s.Index(memoryID, id, text, target); err != nil {
			return err
		}
	}
	return nil
}

func (s *InMemoryBM25Search) Remove(memoryID memind.MemoryId, documentID string, target SearchTarget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.getStore(memoryID, target)
	delete(st.docs, documentID)
	for term, docMap := range st.terms {
		delete(docMap, documentID)
		if len(docMap) == 0 {
			delete(st.terms, term)
		}
	}
	return nil
}

func (s *InMemoryBM25Search) Invalidate(memoryID memind.MemoryId) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.indices, s.key(memoryID))
	return nil
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.Fields(text)
	result := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.Trim(w, ".,!?;:\"'()[]{}<>【】「」『』《》，。！？；：、")
		if w != "" {
			result = append(result, w)
		}
	}
	return result
}

func sortByScoreDescBM25(results []Result) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
