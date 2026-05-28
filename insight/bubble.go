package insight

import "sync"

// BubbleTracker - 脏计数跟踪器，用于延迟重摘要
// Modified: 2026-05-28 - 原版 Java BubbleTracker 的 Go 移植
type BubbleTracker struct {
	mu     sync.RWMutex
	counts map[string]int
}

// NewBubbleTracker - 创建脏计数跟踪器
func NewBubbleTracker() *BubbleTracker {
	return &BubbleTracker{counts: make(map[string]int)}
}

// IncrementAndGet - 增加指定键的脏计数并返回新值
func (bt *BubbleTracker) IncrementAndGet(key string, delta int) int {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.counts[key] += delta
	return bt.counts[key]
}

// Get - 读取指定键的脏计数
func (bt *BubbleTracker) Get(key string) int {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.counts[key]
}

// Reset - 重置指定键的脏计数
func (bt *BubbleTracker) Reset(key string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	delete(bt.counts, key)
}

// ResetAll - 重置所有脏计数
func (bt *BubbleTracker) ResetAll() {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.counts = make(map[string]int)
}
