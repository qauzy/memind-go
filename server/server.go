package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"

	memind "github.com/openmemind/memind-go"
)

var operationID atomic.Int64

// HTTPServer - 基于 stdlib 的 HTTP REST 服务器
type HTTPServer struct {
	memory memind.Memory
	addr   string
	mux    *http.ServeMux
}

// New - 创建 HTTP 服务器实例，注册所有路由
func New(memory memind.Memory, addr string) *HTTPServer {
	s := &HTTPServer{
		memory: memory,
		addr:   addr,
		mux:    http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Start - 启动 HTTP 服务器并开始监听
func (s *HTTPServer) Start() error {
	log.Printf("Memind server listening on %s", s.addr)
	return http.ListenAndServe(s.addr, s)
}

// ServeHTTP - 实现 http.Handler 接口
func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// registerRoutes - 注册所有同步和异步 REST 路由
func (s *HTTPServer) registerRoutes() {
	// 同步路由
	s.mux.HandleFunc("POST /open/v1/memory/sync/extract", s.handleExtract)
	s.mux.HandleFunc("POST /open/v1/memory/sync/add-message", s.handleAddMessage)
	s.mux.HandleFunc("POST /open/v1/memory/sync/commit", s.handleCommit)
	s.mux.HandleFunc("POST /open/v1/memory/retrieve", s.handleRetrieve)
	s.mux.HandleFunc("POST /open/v1/memory/context", s.handleGetContext)
	// 异步路由（fire-and-forget）
	s.mux.HandleFunc("POST /open/v1/memory/async/extract", s.handleAsyncExtract)
	s.mux.HandleFunc("POST /open/v1/memory/async/add-message", s.handleAsyncAddMessage)
	s.mux.HandleFunc("POST /open/v1/memory/async/commit", s.handleAsyncCommit)
}

// nextOperationID - 生成全局唯一操作 ID
func nextOperationID() string {
	return fmt.Sprintf("op-%d", operationID.Add(1))
}

// writeAccepted - 返回 202 Accepted 响应
func writeAccepted(w http.ResponseWriter, opID string) {
	writeJSON(w, http.StatusAccepted, memind.OperationAccepted{
		OperationID: opID,
		Status:      "accepted",
		Mode:        "async",
	})
}

// ---------- 同步处理器 ----------

// handleExtract - POST /open/v1/memory/sync/extract 直接提取
func (s *HTTPServer) handleExtract(w http.ResponseWriter, r *http.Request) {
	var req memind.ExtractionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.memory.Extract(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleAddMessage - POST /open/v1/memory/sync/add-message 添加消息到缓冲区
func (s *HTTPServer) handleAddMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemoryID memind.MemoryId         `json:"memoryId"`
		Message  memind.Message          `json:"message"`
		Config   memind.ExtractionConfig `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.memory.AddMessage(req.MemoryID, req.Message, req.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleCommit - POST /open/v1/memory/sync/commit 提交缓冲区消息
func (s *HTTPServer) handleCommit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemoryID memind.MemoryId         `json:"memoryId"`
		Config   memind.ExtractionConfig `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.memory.Commit(req.MemoryID, req.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleRetrieve - POST /open/v1/memory/retrieve 检索记忆
func (s *HTTPServer) handleRetrieve(w http.ResponseWriter, r *http.Request) {
	var req memind.RetrievalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.memory.Retrieve(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleGetContext - POST /open/v1/memory/context 获取上下文窗口
func (s *HTTPServer) handleGetContext(w http.ResponseWriter, r *http.Request) {
	var req memind.ContextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.memory.GetContext(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ---------- 异步处理器 ----------

// handleAsyncExtract - POST /open/v1/memory/async/extract（fire-and-forget）
func (s *HTTPServer) handleAsyncExtract(w http.ResponseWriter, r *http.Request) {
	var req memind.ExtractionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	opID := nextOperationID()
	go func() {
		if _, err := s.memory.Extract(req); err != nil {
			log.Printf("[async %s] extract error: %v", opID, err)
		}
	}()
	writeAccepted(w, opID)
}

// handleAsyncAddMessage - POST /open/v1/memory/async/add-message（fire-and-forget）
func (s *HTTPServer) handleAsyncAddMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemoryID memind.MemoryId         `json:"memoryId"`
		Message  memind.Message          `json:"message"`
		Config   memind.ExtractionConfig `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	opID := nextOperationID()
	go func() {
		if _, err := s.memory.AddMessage(req.MemoryID, req.Message, req.Config); err != nil {
			log.Printf("[async %s] add-message error: %v", opID, err)
		}
	}()
	writeAccepted(w, opID)
}

// handleAsyncCommit - POST /open/v1/memory/async/commit（fire-and-forget）
func (s *HTTPServer) handleAsyncCommit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MemoryID memind.MemoryId         `json:"memoryId"`
		Config   memind.ExtractionConfig `json:"config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	opID := nextOperationID()
	go func() {
		if _, err := s.memory.Commit(req.MemoryID, req.Config); err != nil {
			log.Printf("[async %s] commit error: %v", opID, err)
		}
	}()
	writeAccepted(w, opID)
}

// ---------- 工具函数 ----------

// writeJSON - 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError - 写入 JSON 错误响应
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
