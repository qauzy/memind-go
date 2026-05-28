# 洞察树重构任务计划

## 目标
参照原版 Java memind 的 InsightTreeReorganizer，重构 Go 版的洞察树为完整的 LLM 驱动引擎。

## 架构对比

| 原版 Java | Go 版（现状） | Go 版（目标） |
|-----------|---------------|---------------|
| `InsightTreeReorganizer` (907 行) | `tree.go` (204 行) | `reorganizer.go` ~600 行 |
| `InsightGenerator` 接口 + LLM | 内联 LLM 调用 | `InsightGenerator` 接口 + LLM 实现 |
| `BubbleTracker` 脏计数延迟摘要 | 无 | `BubbleTracker` |
| Point 操作 ADD/UPDATE/DELETE | 全量重写 | Point 操作 + fallback 全量重写 |
| `InsightPointEvidenceNormalizer` | 无 | `EvidenceNormalizer` |
| `InsightPointIdentityManager` | 无 | `PointIdentityManager` |
| `InsightGraphAssistant` | 无 | 简化版 optional |
| Strip lock 并发 | 无 | Strip lock |
| 异步 ROOT 重摘要 | 同步 | goroutine 异步 |

## 阶段

### 阶段 1: 模型扩展
- `models.go` — 添加 `InsightAnalysisMode`, `InsightPointOp`, `InsightPointOpsResponse`, `InsightPointGenerateResponse`
- `config.go` — 已有 `InsightTreeConfig`，字段已够
- 修改 `MemoryInsightType` 添加 `InsightAnalysisMode`

### 阶段 2: BubbleTracker
- `insight/bubble.go` — 并发安全的脏计数跟踪器

### 阶段 3: PointIdentityManager
- `insight/identity.go` — pointId 复用 + 归一化

### 阶段 4: EvidenceNormalizer
- `insight/evidence.go` — sourceItemIds 向上传播

### 阶段 5: InsightGenerator 接口 + LLM 实现
- `insight/generator.go` — 接口定义 + LLM prompt 模板
- 支持 BranchPointOps / BranchSummary / RootSynthesis

### 阶段 6: GraphAssistant（简化版）
- `insight/graph.go` — 基于正文关键词的简单排序建议

### 阶段 7: TreeReorganizer
- `insight/reorganizer.go` — 核心重组织器（取代 tree.go）
- `onLeafsUpdated()` → batchLinkLeafsToBranch → resummarizeBranch → bubbleAndMaybeResummarizeRoots
- Strip lock + 异步 goroutine

### 阶段 8: 管线接入 + Store 扩展
- `store/store.go` — 添加 `GetBranchByType`, `GetRootByType`, `GetInsightsByTierAndNotParented`
- `store/inmemory.go` — 实现新接口
- `engine/memory.go` — 用 Reorganizer 替换 TreeBuilder
- `extraction/extractor.go` — 修改 `Extract()` 调用新 Reorganizer

### 阶段 9: 构建验证
- `go vet ./...`
- `go build ./...`
- `go test ./... -count=1`

## 决策记录

1. **不在 Go 中实现 Reactor Mono** — 用 (`T, error`) 返回值 + goroutine 替代
2. **Snowflake ID 替换为 hash-based stable ID** — 避免外部依赖
3. **Embedding 写入不做阻塞** — 失败只 log 不中断流程
4. **Reorganizer 同步入口，异步 ROOT** — `onLeafsUpdated` 同步，ROOT 重摘要在 goroutine
