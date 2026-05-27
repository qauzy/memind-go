<p align="center">
  <strong>能思考的记忆，能进化的上下文。</strong>
</p>

<p align="center">
  <a href="#overview">概览</a> ·
  <a href="#quick-start">快速开始</a> ·
  <a href="#usage">使用</a> ·
  <a href="#architecture">架构</a> ·
  <a href="#api">API</a>
</p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-orange" alt="License"></a>
  <a href="./README.md"><img src="https://img.shields.io/badge/English-Click-yellow" alt="English"></a>
  <a href="./README_zh.md"><img src="https://img.shields.io/badge/简体中文-点击查看-orange" alt="简体中文"></a>
</p>

<p align="center">
  <a href="#"><img src="https://img.shields.io/badge/Go-1.22-blue" alt="Go 1.22"></a>
  <a href="#"><img src="https://img.shields.io/badge/dependencies-zero-brightgreen" alt="Zero Dependencies"></a>
  <a href="#"><img src="https://img.shields.io/badge/status-alpha-yellow" alt="Status Alpha"></a>
</p>

---

**memind-go** 是 [memind](https://github.com/openmemind/memind) 层级认知记忆与上下文引擎的 Go 原生移植版。它用地道的 Go 重新实现了核心引擎——包括 Insight Tree、双检索策略和提取管线——且零外部依赖。

memind 不把记忆看作一堆彼此孤立的事实，而是持续从对话中提取、组织并演化知识，最终沉淀为结构化的 **Insight Tree**。三个层级各有所见，每一层都能看见上一层无法直接得出的模式。

---

<a id="overview"></a>

## 概览

### Memind 是什么？

Memind 是一个面向 AI Agent 的层级认知记忆与上下文引擎。它要解决 Agent 记忆里最常见的两个问题：

- **存储扁平、缺少结构**——记忆只是零散的事实，没有更高层级的组织
- **知识不会进化**——记忆只会累积，但不会沉淀为更深层的理解

### Insight Tree

| 层级 | 输入 | 产出 |
|------|------|------|
| 🍃 **Leaf** | 分组后的记忆条目 | 单个语义组内的洞察 |
| 🌿 **Branch** | 多个 Leaf | 同一维度内的跨组模式 |
| 🌳 **Root** | 多个 Branch | 低层级无法看见的跨维度洞察 |

### 双作用域记忆

| 作用域 | 类别 | 作用 |
|-------|------|------|
| **USER** | Profile, Behavior, Event | 用户身份、偏好、关系与经历 |
| **AGENT** | Tool, Directive, Playbook, Resolution | 工具使用经验、持久指令、可复用工作流 |

### 双检索策略

| 策略 | 机制 | 适用场景 |
|------|------|----------|
| **Simple** | 向量搜索 + BM25 关键词匹配，通过 RRF（倒数排序融合）合并，带自适应截断 | 低延迟、低成本场景 |
| **Deep** | LLM 辅助的查询扩展、充分性检查和重排序 | 需要推理的复杂查询 |

### 为什么用 Go？

原版 memind 基于 Java（Spring Boot + Project Reactor）。这个 Go 移植版带来了不同的优势：

- **零依赖**——单个二进制文件，无需 JVM，无需外部向量数据库，无需数据库
- **可嵌入**——作为库导入即可使用，不需要运行服务器
- **简单并发模型**——同步 Go API，无响应式流
- **LLM 可选**——核心管线在无 AI 提供商时也能运行；需要 Deep 检索或洞察生成时再接入 LLM

---

<a id="quick-start"></a>

## 快速开始

### 前置条件

- Go 1.22+

### 启动服务器

```bash
git clone https://github.com/openmemind/memind-go
cd memind-go

# 使用内存存储启动
go run ./cmd/memind/ -addr :8080
```

```bash
# 也可通过环境变量配置地址
MEMIND_ADDR=:9090 go run ./cmd/memind/
```

### 作为库使用

```go
package main

import (
    "fmt"
    "github.com/openmemind/memind-go/engine"
    "github.com/openmemind/memind-go/store"
)

func main() {
    mem := engine.Builder().
        Store(store.NewInMemoryStore()).
        Build()
    defer mem.Close()

    memID := struct{ UserID, AgentID string }{UserID: "alice"}

    // 添加一条对话消息（会进入缓冲区）
    mem.AddMessage(memID, Message{Role: "USER", Content: []ContentBlock{
        {Type: "text", Text: "你好，我是 Alice，一名数据科学家。"},
    }}, DefaultExtractionConfig())

    // 提交缓冲区 → 提取 → 存储
    result, _ := mem.Commit(memID, DefaultExtractionConfig())
    fmt.Printf("提取了 %d 条记忆项\n", len(result.Items.NewItems))

    // 检索相关记忆
    ret, _ := mem.Retrieve(RetrievalRequest{
        MemoryID: memID,
        Query:    "告诉我关于 Alice 的信息",
    })
    fmt.Printf("找到 %d 条结果\n", len(ret.Items))
}
```

---

<a id="usage"></a>

## 使用指南

### 核心 API

`Memory` 接口是主要入口：

```go
// 直接提取（不使用缓冲区）
mem.Extract(ExtractionRequest{
    MemoryID: memID,
    Content:  RawContent{Type: "text", Content: "Alice 是一名数据科学家。"},
})

// 缓冲消息流程
mem.AddMessage(memID, message, config)   // 添加到待处理缓冲区
mem.Commit(memID, config)                 // 刷新缓冲区 → 提取 → 存储

// 检索
mem.Retrieve(RetrievalRequest{
    MemoryID: memID,
    Query:    "查询文本",
})

// 上下文窗口（消息 + 记忆）
mem.GetContext(ContextRequest{
    MemoryID:  memID,
    MaxTokens: 4096,
})
```

### 配置

```go
// 使用自定义组件的 Builder
mem := engine.Builder().
    Store(store.NewInMemoryStore()).
    Vector(vector.NewInMemoryVectorStore()).
    TextSearch(textsearch.NewInMemoryBM25Search()).
    Options(memind.DefaultBuildOptions()).
    Build()

// 为特定槽位注册 LLM 客户端
mem = engine.Builder().
    ChatClientForSlot(llm.SlotQueryExpander, myLLMClient).
    Build()
```

---

<a id="architecture"></a>

## 架构

```
┌──────────────────────────────────────────────────┐
│  Memory (接口)                                   │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐ │
│  │ Extractor  │  │ Retriever  │  │   Buffer   │ │
│  └─────┬──────┘  └─────┬──────┘  └─────┬──────┘ │
│        │               │               │        │
│  ┌─────┴──────┐  ┌─────┴──────┐  ┌─────┴──────┐ │
│  │ RawData →  │  │ Simple     │  │ Pending    │ │
│  │ Item →     │  │ Deep       │  │ Recent     │ │
│  │ Insight    │  │ (策略模式)  │  │ Insight    │ │
│  └────────────┘  └────────────┘  └────────────┘ │
│                                                  │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐ │
│  │ MemoryStore│  │ MemoryVec  │  │ TextSearch │ │
│  │ (内存)     │  │ (内存)     │  │ (BM25)     │ │
│  └────────────┘  └────────────┘  └────────────┘ │
│                                                  │
│  ┌────────────┐  ┌────────────┐                  │
│  │  LLM       │  │  Tracing   │                  │
│  │  (可选)    │  │  (NoOp)    │                  │
│  └────────────┘  └────────────┘                  │
└──────────────────────────────────────────────────┘
```

### 包职责

| 包 | 职责 |
|-----|-------|
| `root` | 类型、接口、配置——从不导入子包 |
| `engine/` | `Builder()` + `Memory` 实现——组装所有组件 |
| `store/` | `MemoryStore` 接口 + `InMemoryStore` |
| `vector/` | `MemoryVector` 接口 + 哈希嵌入引擎（128维，余弦相似度） |
| `textsearch/` | `MemoryTextSearch` 接口 + BM25 全文搜索 |
| `extraction/` | 管线：RawData → MemoryItem → Insight |
| `retrieval/` | Simple（向量 + BM25 + RRF）和 Deep（Simple + LLM 重排序） |
| `insight/` | `TreeBuilder`——Leaf → Branch → Root 递进 |
| `buffer/` | PendingConversation、RecentConversation、Insight 缓冲区 |
| `llm/` | `StructuredChatClient` 接口 + 槽位注册表（默认 NoOp） |
| `server/` | 标准库 HTTP 服务器，提供 REST 路由 |
| `cmd/memind/` | CLI 二进制入口 |

### 关键设计决策

- **禁止循环导入**：根包从不导入子包，所有组装逻辑在 `engine/` 中
- **LLM 可选**：`StructuredChatClient` 默认为 NoOp；通过 `Builder().ChatClientForSlot(slot, client)` 注册
- **默认内存存储**：无需数据库，无需外部向量数据库
- **内置向量搜索**：基于哈希的 128 维嵌入 + 余弦相似度

---

<a id="api"></a>

## HTTP API

启动服务器后，以下 REST 端点可用：

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/health` | 健康检查 |
| `POST` | `/open/v1/memory/sync/extract` | 直接提取 |
| `POST` | `/open/v1/memory/sync/add-message` | 缓冲消息 |
| `POST` | `/open/v1/memory/sync/commit` | 刷新缓冲区 → 提取 → 存储 |
| `POST` | `/open/v1/memory/retrieve` | 搜索记忆 |
| `POST` | `/open/v1/memory/async/extract` | 异步提取（即发即忘） |
| `POST` | `/open/v1/memory/async/add-message` | 异步添加消息 |
| `POST` | `/open/v1/memory/async/commit` | 异步提交 |

### 示例请求

```bash
# 提取
curl -X POST http://localhost:8080/open/v1/memory/sync/extract \
  -H 'Content-Type: application/json' \
  -d '{"userId":"u1","agentId":"a1","rawContent":{"type":"text","content":"你好世界"}}'

# 检索
curl -X POST http://localhost:8080/open/v1/memory/retrieve \
  -H 'Content-Type: application/json' \
  -d '{"userId":"u1","agentId":"a1","query":"你好","strategy":"SIMPLE"}'
```

---

<a id="development"></a>

## 开发

```bash
go build ./...          # 编译所有包
go vet ./...            # 静态分析
go test ./... -count=1  # 运行所有测试（不缓存）
```

---

## 许可证

Apache 2.0。详见 [LICENSE](./LICENSE)。
