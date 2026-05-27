package tracing

import "time"

type Span struct {
	Name       string                 `json:"name"`
	StartTime  time.Time              `json:"startTime"`
	EndTime    time.Time              `json:"endTime"`
	Attributes map[string]any         `json:"attributes,omitempty"`
	Status     string                 `json:"status"`
	Children   []*Span                `json:"children,omitempty"`
}

type TraceContext struct {
	TraceID string `json:"traceId"`
	Spans   []*Span `json:"spans"`
}

type MemoryObserver interface {
	Observe(spanName string, attrs map[string]any, fn func() error) error
}

type NoOpObserver struct{}

func (o *NoOpObserver) Observe(spanName string, attrs map[string]any, fn func() error) error {
	return fn()
}

type SimpleObserver struct {
	traces []*TraceContext
}

func NewSimpleObserver() *SimpleObserver {
	return &SimpleObserver{}
}

func (o *SimpleObserver) Observe(spanName string, attrs map[string]any, fn func() error) error {
	start := time.Now()
	err := fn()
	if err == nil {
		return nil
	}
	_ = start
	return err
}

func (o *SimpleObserver) Traces() []*TraceContext {
	return o.traces
}
