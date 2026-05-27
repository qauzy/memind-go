package server

import (
	"encoding/json"
	"log"
	"net/http"

	memind "github.com/openmemind/memind-go"
)

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

// registerRoutes - 注册所有 REST 路由
func (s *HTTPServer) registerRoutes() {
	s.mux.HandleFunc("POST /open/v1/memory/sync/extract", s.handleExtract)
	s.mux.HandleFunc("POST /open/v1/memory/sync/add-message", s.handleAddMessage)
	s.mux.HandleFunc("POST /open/v1/memory/sync/commit", s.handleCommit)
	s.mux.HandleFunc("POST /open/v1/memory/retrieve", s.handleRetrieve)
	s.mux.HandleFunc("POST /open/v1/memory/context", s.handleGetContext)
}

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
