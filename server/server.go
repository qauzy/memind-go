package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	memind "github.com/openmemind/memind-go"
)

type Server struct {
	memory memind.Memory
	mux    *http.ServeMux
	addr   string
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func New(memory memind.Memory, addr string) *Server {
	if addr == "" {
		addr = ":8080"
	}
	s := &Server{
		memory: memory,
		mux:    http.NewServeMux(),
		addr:   addr,
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/open/v1/memory/sync/extract", s.handleExtract)
	s.mux.HandleFunc("/open/v1/memory/sync/add-message", s.handleAddMessage)
	s.mux.HandleFunc("/open/v1/memory/sync/commit", s.handleCommit)
	s.mux.HandleFunc("/open/v1/memory/retrieve", s.handleRetrieve)
	s.mux.HandleFunc("/open/v1/memory/async/extract", s.handleAsyncExtract)
	s.mux.HandleFunc("/open/v1/memory/async/add-message", s.handleAsyncAddMessage)
	s.mux.HandleFunc("/open/v1/memory/async/commit", s.handleAsyncCommit)
}

func (s *Server) Start() error {
	log.Printf("memind server starting on %s", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, memind.HealthResponse{Status: "ok", Service: "memind-go"})
}

func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req struct {
		UserID       string `json:"userId"`
		AgentID      string `json:"agentId"`
		RawContent   struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"rawContent"`
		SourceClient string `json:"sourceClient,omitempty"`
		Language     string `json:"language,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request", Message: err.Error()})
		return
	}

	cfg := memind.DefaultExtractionConfig()
	if req.Language != "" {
		cfg.Language = req.Language
	}

	result, err := s.memory.Extract(memind.ExtractionRequest{
		MemoryID: memind.NewMemoryId(req.UserID, req.AgentID),
		Content:  memind.RawContent{Type: req.RawContent.Type, Content: req.RawContent.Content},
		Config:   cfg,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "extraction failed", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, toExtractResponse(result))
}

func (s *Server) handleAddMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req struct {
		UserID       string `json:"userId"`
		AgentID      string `json:"agentId"`
		Message      struct {
			Role         string `json:"role"`
			Content      []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
		SourceClient string `json:"sourceClient,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request", Message: err.Error()})
		return
	}

	role := memind.RoleUser
	if strings.ToUpper(req.Message.Role) == "ASSISTANT" {
		role = memind.RoleAssistant
	}

	var blocks []memind.ContentBlock
	for _, c := range req.Message.Content {
		blocks = append(blocks, memind.ContentBlock{Type: c.Type, Text: c.Text})
	}

	memoryID := memind.NewMemoryId(req.UserID, req.AgentID)
	now := time.Now()
	msg := memind.Message{
		Role:    role,
		Content: blocks,
		Timestamp: &now,
	}

	result, err := s.memory.AddMessage(memoryID, msg, memind.DefaultExtractionConfig())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "add message failed", Message: err.Error()})
		return
	}

	resp := memind.AddMessageResponse{
		Triggered: result.Status == memind.ExtractionSuccess && (len(result.Items.NewItems) > 0 || len(result.Insights.Insights) > 0),
	}
	if resp.Triggered {
		resp.Result = result
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req struct {
		UserID       string `json:"userId"`
		AgentID      string `json:"agentId"`
		SourceClient string `json:"sourceClient,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request", Message: err.Error()})
		return
	}

	result, err := s.memory.Commit(memind.NewMemoryId(req.UserID, req.AgentID), memind.DefaultExtractionConfig())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "commit failed", Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, toExtractResponse(result))
}

func (s *Server) handleRetrieve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	var req struct {
		UserID   string  `json:"userId"`
		AgentID  string  `json:"agentId"`
		Query    string  `json:"query"`
		Strategy *string `json:"strategy,omitempty"`
		Trace    *bool   `json:"trace,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request", Message: err.Error()})
		return
	}

	strategy := memind.StrategySimple
	if req.Strategy != nil {
		strategy = memind.Strategy(*req.Strategy)
	}

	result, err := s.memory.Retrieve(memind.RetrievalRequest{
		MemoryID: memind.NewMemoryId(req.UserID, req.AgentID),
		Query:    req.Query,
		Config:   memind.SimpleRetrievalConfig(),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "retrieval failed", Message: err.Error()})
		return
	}

	_ = strategy
	writeJSON(w, http.StatusOK, toRetrieveResponse(result))
}

func (s *Server) handleAsyncExtract(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusAccepted, memind.OperationAccepted{
		OperationID: fmt.Sprintf("op-%d", time.Now().UnixNano()),
		Status:      "accepted",
		Mode:        "async",
	})
}

func (s *Server) handleAsyncAddMessage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusAccepted, memind.OperationAccepted{
		OperationID: fmt.Sprintf("op-%d", time.Now().UnixNano()),
		Status:      "accepted",
		Mode:        "async",
	})
}

func (s *Server) handleAsyncCommit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusAccepted, memind.OperationAccepted{
		OperationID: fmt.Sprintf("op-%d", time.Now().UnixNano()),
		Status:      "accepted",
		Mode:        "async",
	})
}

func toExtractResponse(result *memind.ExtractionResult) map[string]any {
	var itemIDs []int64
	for _, item := range result.Items.NewItems {
		itemIDs = append(itemIDs, item.ID)
	}
	var insightIDs []int64
	for _, ins := range result.Insights.Insights {
		insightIDs = append(insightIDs, ins.ID)
	}
	var rawDataIDs []string
	for _, rd := range result.RawData.RawDataList {
		rawDataIDs = append(rawDataIDs, rd.ID)
	}

	resp := map[string]any{
		"status":         string(result.Status),
		"rawDataIds":     rawDataIDs,
		"itemIds":        itemIDs,
		"insightIds":     insightIDs,
		"insightPending": result.InsightPending,
	}
	if result.ErrorMessage != "" {
		resp["errorMessage"] = result.ErrorMessage
	}
	return resp
}

func toRetrieveResponse(result *memind.RetrievalResult) map[string]any {
	items := make([]map[string]any, len(result.Items))
	for i, item := range result.Items {
		items[i] = map[string]any{
			"id":          item.SourceID,
			"text":        item.Text,
			"vectorScore": item.VectorScore,
			"finalScore":  item.FinalScore,
		}
	}

	insights := make([]map[string]any, len(result.Insights))
	for i, ins := range result.Insights {
		insights[i] = map[string]any{
			"id":   ins.ID,
			"text": ins.Text,
			"tier": string(ins.Tier),
		}
	}

	rawData := make([]map[string]any, len(result.RawData))
	for i, rd := range result.RawData {
		rawData[i] = map[string]any{
			"rawDataId": rd.RawDataID,
			"caption":   rd.Caption,
			"maxScore":  rd.MaxScore,
		}
	}

	return map[string]any{
		"items":    items,
		"insights": insights,
		"rawData":  rawData,
		"strategy": result.Strategy,
		"query":    result.Query,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
