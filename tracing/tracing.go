package tracing

import (
	"time"

	memind "github.com/openmemind/memind-go"
)

// MemoryObserver - 可观测性接口，记录内存系统的各类事件
type MemoryObserver interface {
	OnExtractionStart(memoryID memind.MemoryId, contentType string)
	OnExtractionEnd(memoryID memind.MemoryId, duration time.Duration, itemCount int, err error)
	OnRetrievalStart(memoryID memind.MemoryId, strategy string)
	OnRetrievalEnd(memoryID memind.MemoryId, duration time.Duration, resultCount int, err error)
	OnLLMCall(slot string, duration time.Duration, promptTokens int, err error)
	OnItemExtracted(memoryID memind.MemoryId, item *memind.MemoryItem)
	OnInsightGenerated(memoryID memind.MemoryId, insight *memind.MemoryInsight)
	OnError(component string, err error)
}

// NoOpObserver - 空操作实现，默认使用
type NoOpObserver struct{}

func (o *NoOpObserver) OnExtractionStart(memoryID memind.MemoryId, contentType string) {}
func (o *NoOpObserver) OnExtractionEnd(memoryID memind.MemoryId, duration time.Duration, itemCount int, err error) {
}
func (o *NoOpObserver) OnRetrievalStart(memoryID memind.MemoryId, strategy string) {}
func (o *NoOpObserver) OnRetrievalEnd(memoryID memind.MemoryId, duration time.Duration, resultCount int, err error) {
}
func (o *NoOpObserver) OnLLMCall(slot string, duration time.Duration, promptTokens int, err error) {}
func (o *NoOpObserver) OnItemExtracted(memoryID memind.MemoryId, item *memind.MemoryItem)          {}
func (o *NoOpObserver) OnInsightGenerated(memoryID memind.MemoryId, insight *memind.MemoryInsight) {}
func (o *NoOpObserver) OnError(component string, err error)                                        {}
