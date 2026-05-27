package memind

import "errors"

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

type AdmissionError struct {
	Reason string
}

func (e *AdmissionError) Error() string { return "admission rejected: " + e.Reason }

type ExtractionError struct {
	Stage   string
	Message string
	Err     error
}

func (e *ExtractionError) Error() string {
	if e.Err != nil {
		return "extraction failed at " + e.Stage + ": " + e.Message + ": " + e.Err.Error()
	}
	return "extraction failed at " + e.Stage + ": " + e.Message
}

func (e *ExtractionError) Unwrap() error { return e.Err }
