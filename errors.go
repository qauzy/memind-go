package memind

import "errors"

// 预定义错误 - memind 全系统通用错误码
var (
	ErrMemoryNotFound      = errors.New("memory not found")
	ErrItemNotFound        = errors.New("memory item not found")
	ErrInsightNotFound     = errors.New("insight not found")
	ErrRawDataNotFound     = errors.New("raw data not found")
	ErrInvalidRequest      = errors.New("invalid request")
	ErrEmptyQuery          = errors.New("empty query")
	ErrStoreNotInitialized = errors.New("store not initialized")
	ErrLLMClientNotSet     = errors.New("LLM client not configured")
	ErrVectorStoreNotSet   = errors.New("vector store not configured")
	ErrTextSearchNotSet    = errors.New("text search not configured")
	ErrStrategyNotFound    = errors.New("retrieval strategy not found")
)

// AdmissionError - 检索准入拒绝错误
type AdmissionError struct {
	Reason string
}

// Error - 实现 error 接口
func (e *AdmissionError) Error() string { return "admission rejected: " + e.Reason }

// ExtractionError - 提取管线阶段错误，携带出错阶段信息
type ExtractionError struct {
	Stage   string
	Message string
	Err     error
}

// Error - 实现 error 接口
func (e *ExtractionError) Error() string {
	if e.Err != nil {
		return "extraction failed at " + e.Stage + ": " + e.Message + ": " + e.Err.Error()
	}
	return "extraction failed at " + e.Stage + ": " + e.Message
}

// Unwrap - 支持 errors.Is / errors.As 链式解包
func (e *ExtractionError) Unwrap() error { return e.Err }
