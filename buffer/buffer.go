package buffer

import (
	"sync"
	"time"

	memind "github.com/openmemind/memind-go"
)

type PendingConversationBuffer interface {
	Add(memoryID memind.MemoryId, msg memind.Message) error
	Get(memoryID memind.MemoryId) ([]memind.Message, error)
	Clear(memoryID memind.MemoryId) error
	Size(memoryID memind.MemoryId) int
}

type RecentConversationBuffer interface {
	Add(memoryID memind.MemoryId, msg memind.Message) error
	GetRecent(memoryID memind.MemoryId, limit int) ([]memind.Message, error)
}

type InsightBuffer interface {
	Add(memoryID memind.MemoryId, itemID int64, group string) error
	GetBuilt(memoryID memind.MemoryId) ([]int64, error)
}

type MemoryBuffer interface {
	PendingConversation() PendingConversationBuffer
	RecentConversation() RecentConversationBuffer
	InsightBuffer() InsightBuffer
}

type InMemoryPendingBuffer struct {
	mu sync.RWMutex
	buf map[string][]memind.Message
}

func NewInMemoryPendingBuffer() *InMemoryPendingBuffer {
	return &InMemoryPendingBuffer{buf: make(map[string][]memind.Message)}
}

func (b *InMemoryPendingBuffer) Add(memoryID memind.MemoryId, msg memind.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := memoryID.Identifier()
	msg.Timestamp = timePtr(time.Now())
	b.buf[k] = append(b.buf[k], msg)
	return nil
}

func (b *InMemoryPendingBuffer) Get(memoryID memind.MemoryId) ([]memind.Message, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	k := memoryID.Identifier()
	result := make([]memind.Message, len(b.buf[k]))
	copy(result, b.buf[k])
	return result, nil
}

func (b *InMemoryPendingBuffer) Clear(memoryID memind.MemoryId) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.buf, memoryID.Identifier())
	return nil
}

func (b *InMemoryPendingBuffer) Size(memoryID memind.MemoryId) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.buf[memoryID.Identifier()])
}

type InMemoryRecentBuffer struct {
	mu     sync.RWMutex
	buf    map[string][]memind.Message
	maxLen int
}

func NewInMemoryRecentBuffer(maxLen int) *InMemoryRecentBuffer {
	if maxLen <= 0 {
		maxLen = 100
	}
	return &InMemoryRecentBuffer{
		buf:    make(map[string][]memind.Message),
		maxLen: maxLen,
	}
}

func (b *InMemoryRecentBuffer) Add(memoryID memind.MemoryId, msg memind.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := memoryID.Identifier()
	b.buf[k] = append(b.buf[k], msg)
	if len(b.buf[k]) > b.maxLen {
		b.buf[k] = b.buf[k][len(b.buf[k])-b.maxLen:]
	}
	return nil
}

func (b *InMemoryRecentBuffer) GetRecent(memoryID memind.MemoryId, limit int) ([]memind.Message, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	k := memoryID.Identifier()
	if limit <= 0 || limit > len(b.buf[k]) {
		limit = len(b.buf[k])
	}
	msgs := b.buf[k]
	result := make([]memind.Message, limit)
	copy(result, msgs[len(msgs)-limit:])
	return result, nil
}

type InMemoryInsightBuf struct {
	mu      sync.RWMutex
	buf     map[string]map[int64]bool
	built   map[string]map[int64]bool
}

func NewInMemoryInsightBuf() *InMemoryInsightBuf {
	return &InMemoryInsightBuf{
		buf:   make(map[string]map[int64]bool),
		built: make(map[string]map[int64]bool),
	}
}

func (b *InMemoryInsightBuf) Add(memoryID memind.MemoryId, itemID int64, group string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	k := memoryID.Identifier() + ":" + group
	if b.buf[k] == nil {
		b.buf[k] = make(map[int64]bool)
	}
	b.buf[k][itemID] = false
	return nil
}

func (b *InMemoryInsightBuf) GetBuilt(memoryID memind.MemoryId) ([]int64, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	var ids []int64
	for _, m := range b.buf {
		for id := range m {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

type InMemoryBuffer struct {
	Pending  *InMemoryPendingBuffer
	Recent   *InMemoryRecentBuffer
	Insight  *InMemoryInsightBuf
}

func NewInMemoryBuffer() *InMemoryBuffer {
	return &InMemoryBuffer{
		Pending: NewInMemoryPendingBuffer(),
		Recent:  NewInMemoryRecentBuffer(100),
		Insight: NewInMemoryInsightBuf(),
	}
}

func (b *InMemoryBuffer) PendingConversation() PendingConversationBuffer { return b.Pending }
func (b *InMemoryBuffer) RecentConversation() RecentConversationBuffer   { return b.Recent }
func (b *InMemoryBuffer) InsightBuffer() InsightBuffer                   { return b.Insight }

func timePtr(t time.Time) *time.Time {
	return &t
}
