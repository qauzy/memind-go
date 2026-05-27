# memind-go — AGENTS.md

## Entrypoint & Build

- CLI: `cmd/memind/main.go` — builds `engine.Builder().Store(store.NewInMemoryStore()).Build()` → starts HTTP server.
- Single module `github.com/openmemind/memind-go`, Go 1.22, zero runtime deps.
- No codegen, migrations, or build steps beyond `go build`.

## Commands

```bash
go build ./...          # compile all packages
go vet ./...            # static analysis
go test ./... -count=1  # all tests (no caching)
go run ./cmd/memind/ -addr :8080   # start server
```

## Package layout

| Package | Role |
|---------|------|
| `memind` (root) | Types, interfaces, config — **never import subpackages** |
| `engine/` | `Builder()` + `Memory` impl — wires all subpackages |
| `store/` | `MemoryStore` interface + `InMemoryStore` |
| `vector/` | `MemoryVector` interface + built-in hash-embedding engine |
| `textsearch/` | `MemoryTextSearch` interface + BM25 |
| `extraction/` | RawData → Item → Insight pipeline |
| `retrieval/` | Simple (vec+BM25+RRF) / Deep (Simple+LLM rerank) strategies |
| `insight/` | `TreeBuilder` — Leaf→Branch→Root progression |
| `buffer/` | PendingConversation / RecentConversation / Insight buffers |
| `llm/` | `StructuredChatClient` interface + slot registry |
| `server/` | stdlib HTTP server (REST routes at `/open/v1/memory/*`) |
| `cmd/memind/` | CLI binary entrypoint |

## Architecture constraints

- **Circular imports**: root `memind` package can never import subpackages. All wiring lives in `engine/`.
- **LLM is optional**: `llm.StructuredChatClient` defaults to NoOp. Register per-slot with `Builder().ChatClientForSlot(slot, client)`.
- **All storage defaults to InMemory**: `store.NewInMemoryStore()`, `vector.NewInMemoryVectorStore()`, `textsearch.NewInMemoryBM25Search()`. No DB needed.
- **Vector search is built-in**: hash-based 128-dim embedding + cosine similarity. No external vector DB.

## Testing

- External test package `memind_test` at root: shows canonical usage pattern.
- Pattern: `engine.Builder().Store(store.NewInMemoryStore()).Build()` → `defer mem.Close()`.
- Flow test: `AddMessage` → `Commit` → `Retrieve` → verify non-empty.

## Key API flow

```go
mem := engine.Builder().Build()  // InMemory defaults
mem.AddMessage(memID, msg, config)
mem.Commit(memID, config)
mem.Retrieve(RetrievalRequest{MemoryID: memID, Query: "..."})
mem.Extract(ExtractionRequest{...})  // direct, no buffer
```

## Server routes

- `POST /open/v1/memory/sync/extract` — direct extraction
- `POST /open/v1/memory/sync/add-message` — buffer message
- `POST /open/v1/memory/sync/commit` — flush buffer → extract → store
- `POST /open/v1/memory/retrieve` — search
- Async variants at `/open/v1/memory/async/*` — fire-and-forget
