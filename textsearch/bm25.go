package textsearch

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	memind "github.com/openmemind/memind-go"
)

// bm25Index - BM25 索引，按 MemoryID 和 SearchTarget 组织
type bm25Index struct {
	documents map[string]int            // documentID → doc length (word count)
	terms     map[string]map[string]int // term → documentID → frequency
	docText   map[string]string         // documentID → original text
	avgDocLen float64
	numDocs   int
}

// InMemoryBM25Search - 内存版 BM25 全文搜索引擎
type InMemoryBM25Search struct {
	mu    sync.RWMutex
	store map[memind.MemoryId]map[SearchTarget]*bm25Index
}

// NewInMemoryBM25Search - 创建 BM25 搜索实例
func NewInMemoryBM25Search() *InMemoryBM25Search {
	return &InMemoryBM25Search{
		store: make(map[memind.MemoryId]map[SearchTarget]*bm25Index),
	}
}

const (
	k1 = 1.5
	b  = 0.75
)

// Search - 对指定 memory 和 target 执行 BM25 搜索
func (s *InMemoryBM25Search) Search(memoryID memind.MemoryId, query string, topK int, target SearchTarget) ([]Result, error) {
	s.mu.RLock()
	idx := s.getOrCreateIndex(memoryID, target)
	s.mu.RUnlock()

	terms := tokenize(query)
	scores := make(map[string]float64)

	for _, term := range terms {
		df := len(idx.terms[term])
		for docID, freq := range idx.terms[term] {
			idf := math.Log(1 + (float64(idx.numDocs)-float64(df)+0.5)/(float64(df)+0.5))
			docLen := idx.documents[docID]
			tf := float64(freq) * (k1 + 1) / (float64(freq) + k1*(1-b+b*float64(docLen)/idx.avgDocLen))
			scores[docID] += idf * tf
		}
	}

	results := make([]Result, 0, len(scores))
	for docID, score := range scores {
		results = append(results, Result{DocumentID: docID, Text: idx.docText[docID], Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

// Index - 索引单个文档
func (s *InMemoryBM25Search) Index(memoryID memind.MemoryId, documentID string, text string, target SearchTarget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.getOrCreateIndex(memoryID, target)
	s.addDocument(idx, documentID, text)
	return nil
}

// IndexBatch - 批量索引文档
func (s *InMemoryBM25Search) IndexBatch(memoryID memind.MemoryId, documents map[string]string, target SearchTarget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.getOrCreateIndex(memoryID, target)
	for docID, text := range documents {
		s.addDocument(idx, docID, text)
	}
	return nil
}

// Remove - 从索引中移除文档
func (s *InMemoryBM25Search) Remove(memoryID memind.MemoryId, documentID string, target SearchTarget) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.getOrCreateIndex(memoryID, target)
	if _, ok := idx.documents[documentID]; !ok {
		return nil
	}

	docLen := idx.documents[documentID]
	delete(idx.documents, documentID)
	delete(idx.docText, documentID)
	idx.numDocs--

	for term, docFreq := range idx.terms {
		if _, ok := docFreq[documentID]; ok {
			delete(docFreq, documentID)
			if len(docFreq) == 0 {
				delete(idx.terms, term)
			}
		}
	}

	if idx.numDocs > 0 {
		idx.avgDocLen = (idx.avgDocLen*float64(idx.numDocs+1) - float64(docLen)) / float64(idx.numDocs)
		if idx.avgDocLen < 0 {
			idx.avgDocLen = 0
		}
	} else {
		idx.avgDocLen = 0
	}
	return nil
}

// Invalidate - 清空指定 memory 的所有索引
func (s *InMemoryBM25Search) Invalidate(memoryID memind.MemoryId) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, memoryID)
	return nil
}

// ClearAll - 清空所有索引
func (s *InMemoryBM25Search) ClearAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = make(map[memind.MemoryId]map[SearchTarget]*bm25Index)
	return nil
}

// getOrCreateIndex - 获取或创建指定 memory+target 的 BM25 索引
func (s *InMemoryBM25Search) getOrCreateIndex(memoryID memind.MemoryId, target SearchTarget) *bm25Index {
	if s.store[memoryID] == nil {
		s.store[memoryID] = make(map[SearchTarget]*bm25Index)
	}
	if s.store[memoryID][target] == nil {
		s.store[memoryID][target] = &bm25Index{
			documents: make(map[string]int),
			terms:     make(map[string]map[string]int),
			docText:   make(map[string]string),
		}
	}
	return s.store[memoryID][target]
}

// addDocument - 向 BM25 索引中添加文档并更新统计
func (s *InMemoryBM25Search) addDocument(idx *bm25Index, docID, text string) {
	// 移除旧文档
	if oldLen, ok := idx.documents[docID]; ok {
		for term, docFreq := range idx.terms {
			if _, exists := docFreq[docID]; exists {
				delete(docFreq, docID)
				if len(docFreq) == 0 {
					delete(idx.terms, term)
				}
			}
		}
		idx.numDocs--
		idx.avgDocLen = (idx.avgDocLen*float64(idx.numDocs+1) - float64(oldLen)) / float64(max(1, idx.numDocs))
	}

	terms := tokenize(text)
	termFreq := make(map[string]int)
	for _, term := range terms {
		termFreq[term]++
	}

	docLen := len(terms)
	idx.documents[docID] = docLen
	idx.docText[docID] = text
	idx.numDocs++

	for term, freq := range termFreq {
		if idx.terms[term] == nil {
			idx.terms[term] = make(map[string]int)
		}
		idx.terms[term][docID] = freq
	}

	idx.avgDocLen = (idx.avgDocLen*float64(idx.numDocs-1) + float64(docLen)) / float64(idx.numDocs)
}

// tokenize - 分词：小写 + 按空白切分 + CJK 字符拆分为单字
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	for _, part := range strings.Fields(text) {
		cleaned := strings.Trim(part, ".,!?;:\"'()[]{}\u3001\u3002\uff01\uff1f\uff1b\uff1a\u201c\u201d")
		if cleaned == "" {
			continue
		}
		// 若包含 CJK 字符，拆分为单字
		if hasCJK(cleaned) {
			runes := []rune(cleaned)
			for _, r := range runes {
				if r > 0x2E80 && (r < 0x3000 || r > 0x303F) && r != 0xFF0C && r != 0x3001 && r != 0x3002 {
					tokens = append(tokens, string(r))
				}
			}
		} else {
			tokens = append(tokens, cleaned)
		}
	}
	return tokens
}

// hasCJK - 检查字符串是否包含中日韩统一表意文字 (CJK Unified Ideographs)
func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
