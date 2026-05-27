package buffer

import (
	"sync"

	memind "github.com/openmemind/memind-go"
)

// PendingConversation - 待提交对话缓冲区，暂存用户/助手消息直到 Commit 被调用
type PendingConversation struct {
	mu       sync.RWMutex
	messages map[memind.MemoryId][]memind.Message
}

// NewPendingConversation - 创建待提交缓冲区
func NewPendingConversation() *PendingConversation {
	return &PendingConversation{
		messages: make(map[memind.MemoryId][]memind.Message),
	}
}

// Add - 向指定 memory 追加消息
func (p *PendingConversation) Add(memoryID memind.MemoryId, msg memind.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages[memoryID] = append(p.messages[memoryID], msg)
	return nil
}

// Get - 获取指定 memory 的所有待提交消息
func (p *PendingConversation) Get(memoryID memind.MemoryId) ([]memind.Message, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	msgs, ok := p.messages[memoryID]
	if !ok {
		return nil, nil
	}
	return msgs, nil
}

// Clear - 清空指定 memory 的待提交消息
func (p *PendingConversation) Clear(memoryID memind.MemoryId) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.messages, memoryID)
}

// Size - 获取指定 memory 的待提交消息数
func (p *PendingConversation) Size(memoryID memind.MemoryId) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.messages[memoryID])
}

// RecentConversation - 近期对话缓冲区，保存最近 N 条消息用于上下文窗口
type RecentConversation struct {
	mu       sync.RWMutex
	messages map[memind.MemoryId][]memind.Message
	maxSize  int
}

// NewRecentConversation - 创建近期对话缓冲区
func NewRecentConversation(maxSize int) *RecentConversation {
	return &RecentConversation{
		messages: make(map[memind.MemoryId][]memind.Message),
		maxSize:  maxSize,
	}
}

// Add - 追加消息，超出 maxSize 时丢弃最早的消息
func (r *RecentConversation) Add(memoryID memind.MemoryId, msg memind.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages[memoryID] = append(r.messages[memoryID], msg)
	if len(r.messages[memoryID]) > r.maxSize {
		excess := len(r.messages[memoryID]) - r.maxSize
		r.messages[memoryID] = r.messages[memoryID][excess:]
	}
	return nil
}

// GetRecent - 获取最近 N 条消息
func (r *RecentConversation) GetRecent(memoryID memind.MemoryId, n int) ([]memind.Message, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	msgs, ok := r.messages[memoryID]
	if !ok {
		return nil, false
	}
	if n <= 0 || n >= len(msgs) {
		return msgs, true
	}
	return msgs[len(msgs)-n:], true
}

// Clear - 清空指定 memory 的近期消息
func (r *RecentConversation) Clear(memoryID memind.MemoryId) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.messages, memoryID)
}

// pendingInsightItem - 待生成的洞察项（item ID + 洞察类型）
type pendingInsightItem struct {
	ItemID      int64
	InsightType string
}

// InsightBuffer - 洞察结果缓冲区，暂存待生成的洞察项
type InsightBuffer struct {
	mu       sync.RWMutex
	items    map[memind.MemoryId][]pendingInsightItem
	insights map[memind.MemoryId][]*memind.MemoryInsight
}

// NewInsightBuffer - 创建洞察缓冲区
func NewInsightBuffer() *InsightBuffer {
	return &InsightBuffer{
		items:    make(map[memind.MemoryId][]pendingInsightItem),
		insights: make(map[memind.MemoryId][]*memind.MemoryInsight),
	}
}

// Add - 添加待生成的洞察项（itemID + insightTypeName）到缓冲区
func (b *InsightBuffer) Add(memoryID memind.MemoryId, itemID int64, insightType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.items[memoryID] = append(b.items[memoryID], pendingInsightItem{
		ItemID:      itemID,
		InsightType: insightType,
	})
	return nil
}

// Get - 获取并清空指定 memory 的洞察列表
func (b *InsightBuffer) Get(memoryID memind.MemoryId) []*memind.MemoryInsight {
	b.mu.Lock()
	defer b.mu.Unlock()
	insights := b.insights[memoryID]
	delete(b.insights, memoryID)
	return insights
}

// MemoryBuffer - 缓冲区聚合接口，被提取器使用
type MemoryBuffer interface {
	PendingConversation() *PendingConversation
	RecentConversation() *RecentConversation
	InsightBuffer() *InsightBuffer
	AddMessage(memoryID memind.MemoryId, msg memind.Message)
	ConversationExists(memoryID memind.MemoryId) bool
	GetPendingMessageCount(memoryID memind.MemoryId) int
}

// InMemoryBuffer - 内存版缓冲区集合，包含待提交对话、近期对话和洞察缓冲区
type InMemoryBuffer struct {
	pending    *PendingConversation
	recent     *RecentConversation
	insightBuf *InsightBuffer
}

// NewInMemoryBuffer - 创建完整的缓冲区集合
func NewInMemoryBuffer() *InMemoryBuffer {
	return &InMemoryBuffer{
		pending:    NewPendingConversation(),
		recent:     NewRecentConversation(100),
		insightBuf: NewInsightBuffer(),
	}
}

func (b *InMemoryBuffer) PendingConversation() *PendingConversation { return b.pending }
func (b *InMemoryBuffer) RecentConversation() *RecentConversation   { return b.recent }
func (b *InMemoryBuffer) InsightBuffer() *InsightBuffer             { return b.insightBuf }

// AddMessage - 添加消息到待提交对话和近期对话缓冲区
func (b *InMemoryBuffer) AddMessage(memoryID memind.MemoryId, msg memind.Message) {
	b.pending.Add(memoryID, msg)
	b.recent.Add(memoryID, msg)
}

// ConversationExists - 检查指定 memory 是否有最近的会话活动
func (b *InMemoryBuffer) ConversationExists(memoryID memind.MemoryId) bool {
	msgs, ok := b.recent.GetRecent(memoryID, 1)
	return ok && len(msgs) > 0
}

// GetPendingMessageCount - 获取待提交消息数量
func (b *InMemoryBuffer) GetPendingMessageCount(memoryID memind.MemoryId) int {
	return b.pending.Size(memoryID)
}
