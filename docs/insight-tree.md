# 洞察树（Insight Tree）运作逻辑详解

## 一、整体架构

洞察树是**三层固定结构**，按 `InsightType` 组织：

```
                    ┌─────────────┐
                    │   ROOT      │  ← 跨维度深度合成（profile, interaction）
                    └──────┬──────┘
                           │ childInsightIDs
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ BRANCH   │ │ BRANCH   │ │ BRANCH   │  ← 每个 InsightType 一个（identity, preferences...）
        └─────┬────┘ └─────┬────┘ └─────┬────┘
               │            │            │ parentInsightID
     ┌─────────┼──┐  ┌─────┼──┐  ┌──────┼──┐
     ▼        ▼  ▼  ▼     ▼  ▼  ▼      ▼  ▼
   LEAF      LEAF        LEAF          LEAF    ← 每个语义组一个
```

### 内置 InsightType（共 10 个）

| 类型 | 模式 | 作用域 | 分类 | Token |
|------|------|--------|------|-------|
| identity | BRANCH | USER | PROFILE | 300 |
| preferences | BRANCH | USER | BEHAVIOR | 300 |
| relationships | BRANCH | USER | BEHAVIOR, EVENT | 300 |
| experiences | BRANCH | USER | EVENT | 400 |
| behavior | BRANCH | USER | BEHAVIOR | 300 |
| directives | BRANCH | AGENT | DIRECTIVE | 400 |
| playbooks | BRANCH | AGENT | PLAYBOOK | 500 |
| resolutions | BRANCH | AGENT | RESOLUTION | 400 |
| **profile** | **ROOT** | USER | 全分类 | 800 |
| **interaction** | **ROOT** | AGENT | 全分类 | 800 |

### 三层含义

| 层级 | 含义 | 生成方式 |
|------|------|----------|
| **LEAF** | 从原始条目中提取的细粒度洞察点 | 提取器调用 LLM 生成，无 LLM 时回退为单条 SUMMARY |
| **BRANCH** | 同类型下所有 LEAF 的聚合摘要 | LLM 增量操作或全量重写，无 LLM 时机械拼接 |
| **ROOT** | 跨类型的全局深度分析 | LLM 深度合成（收敛/张力/轨迹/因果四维度分析） |

---

## 二、完整流程：从消息到 ROOT

### 步骤 1：引擎入口

```
engine/memory.go: Extract()
  → extractor.Extract()   // 生成 RawData → Item → LEAF
  → 遍历 result.Items.Types
    → reorganizer.OnLeafsUpdated(memoryID, typeName, type, leafs, config, language)
```

`Extract()` 先调用提取管线生成 LEAF 洞察，然后**按 InsightType 分组**，为每个有新建 LEAF 的类型调用 `OnLeafsUpdated()`。

### 步骤 2：LEAF 生成（提取器内）

`extraction/extractor.go: extractInsights()` 为每个 `(newItem × matchingInsightType)` 组合创建一个 `Tier=LEAF` 的 `MemoryInsight`，包含 LLM 提取的 `InsightPoint` 列表。输出通过 `InsightResult.ByType` 按类型分组。

### 步骤 3：OnLeafsUpdated—核心入口

`insight/reorganizer.go: OnLeafsUpdated()` 是整条链路的入口，分 **5 个子阶段**执行。

---

## 三、5 个子阶段详解

### 阶段 A：Bubble 脏计数

```go
dirtyKey := branchBubbleKey(memoryID, insightTypeName)
branchDirtyCount := r.bubble.IncrementAndGet(dirtyKey, len(builtLeafs))
```

**目的**：避免每次 LEAF 变化都触发昂贵的 LLM 重摘要。`BubbleTracker` 是一个并发安全的 `map[string]int`。每次有新 LEAF 写入时，对应类型的脏计数累加。

只有 `branchDirtyCount >= config.BranchBubbleThreshold`（默认 3）时才执行 BRANCH 重摘要。否则直接返回——树结构（父子链接）已更新，但 BRANCH 的摘要文本保持不变。

### 阶段 B：BRANCH 保证存在 + LEAF 链接

```go
branch, err := r.getOrCreateBranch(memoryID, insightTypeName, insightType)
linkedBranch := r.batchLinkLeafsToBranch(memoryID, leafList, branch)
```

- `getOrCreateBranch`：查询 store 中是否存在该类型的 BRANCH 节点；不存在则创建一个空的 `Tier: TierBranch` 的 `MemoryInsight`。
- `batchLinkLeafsToBranch`：将所有同类型 LEAF 的 `ParentInsightID` 指向 BRANCH，并将 BRANCH 的 `ChildInsightIDs` 更新为所有 LEAF 的 ID 集合。

### 阶段 C：BRANCH → ROOT 链接

```go
rootCtx := r.queryRootContext(memoryID)
rootLock := r.rootLock(memoryID)
rootLock.Lock()
r.linkBranchToRoot(memoryID, linkedBranch, rootCtx)
rootLock.Unlock()
```

- **Strip lock**：16 条互斥锁，按 `memoryId` 哈希取模分配，避免不同 memoryId 之间的竞争。
- `queryRootContext`：一次查询所有 BRANCH 节点和所有 `AnalysisMode=ROOT` 的类型。
- `linkBranchToRoot`：遍历所有 ROOT 类型，检查 BRANCH 数量是否达到 `MinBranchesForRoot`（默认 2）。达到则创建 ROOT（如果不存在），然后将当前 BRANCH ID 追加到 ROOT 的 `ChildInsightIDs`。

### 阶段 D：BRANCH 重摘要（LLM 核心）

`resummarizeBranch` 实现**三层回退策略**：

#### 第 1 层：增量操作（优先）

```
generator.go: GenerateBranchPointOps()
  → LLM 返回 {"operations": [{"op": "ADD|UPDATE|DELETE", ...}]}
  → IdentityManager.NormalizeGeneratedOperations()  — 对齐 pointId
  → operation.go: resolvePointOps()                 — 解析操作
  → EvidenceNormalizer.NormalizeBranchPoints()      — 传播 sourceItemIds
  → embedAndSaveIfChanged()                         — 持久化
```

调用 LLM 生成 `ADD/UPDATE/DELETE` 操作。`resolvePointOps()` 对三种操作的处理：

| 操作 | 行为 |
|------|------|
| ADD | 插入新 point，pointId 为空时标记 fallback |
| UPDATE | 更新 content/type/metadata，pointId 不存在时标记 fallback |
| DELETE | 移除 point，不存在则跳过 |

**ADD / UPDATE / DELETE 解析规则**（`insight/operation.go`）：

```go
case OpAdd:
    pointMap[op.PointID] = newPoint    // 插入
case OpUpdate:
    existingPoint.Content = op.Content  // 更新字段
case OpDelete:
    delete(pointMap, op.PointID)        // 删除
```

任何异常（如 UPDATE 不存在的 pointId）触发 `fallbackRequired`。

#### 第 2 层：全量重写（回退）

```
generator.go: GenerateBranchSummary()
  → LLM 返回 {"points": [...]}
  → IdentityManager.ReusePointIDsForFullRewrite()  — 复用现有 pointId
  → EvidenceNormalizer.NormalizeBranchPoints()
  → embedAndSaveIfChanged()
```

当增量操作失败或 LLM 不支持操作模式时，请求 LLM 重新生成完整的 points 列表。`ReusePointIDsForFullRewrite` 按 content hash 匹配现有 pointId，避免相同内容的 point 每次重写都生成新 ID。

#### 第 3 层：机械拼接（最终回退）

```go
func (r *TreeReorganizer) fallbackBranchConcat(...) *memind.MemoryInsight {
    // 遍历所有子 Leaf，逐个 point 拼接到 Branch
    for _, leaf := range leafList {
        for _, p := range leaf.Points {
            points = append(points, InsightPoint{
                PointID:  "fb-{leafID}-{pointID}",
                Content:  p.Content,
                SourceRefs: []InsightPointRef{{InsightID: leaf.ID, PointID: p.PointID}},
            })
        }
    }
}
```

当 LLM 不可用（`NoOpChatClient`）或 LLM 返回空结果时，机械拼接所有子 LEAF 的 points。

### 阶段 E：ROOT 泡泡 + 异步重摘要

```go
rootLock.Lock()
r.bubbleAndMaybeResummarizeRoots(memoryID, updatedBranch, rootCtx, config, language)
rootLock.Unlock()
```

`bubbleAndMaybeResummarizeRoots`：

1. 对每个 ROOT 类型，递增 Root 级别脏计数
2. 若达到 `RootBubbleThreshold`（默认 2），将重摘要任务加入 pending 列表
3. 在 goroutine 中异步执行：

```go
for _, p := range pendingList {
    p := p
    wg := r.getOrCreateWaitGroup(memoryID)
    wg.Add(1)
    go func() {
        defer wg.Done()
        r.resummarizeRootAndReset(memoryID, p, language)
    }()
}
```

`resummarizeRoot` 调用 `generator.GenerateRootSynthesis()`，prompt 要求 LLM 做**四维度深度分析**：

| 维度 | 含义 |
|------|------|
| CONVERGENCE | 不同 BRANCH 间的一致与强化 |
| TENSION | 冲突与权衡 |
| TRAJECTORY | 时间趋势与方向变化 |
| CAUSATION | 跨维度因果关系 |

Branch 支持增量操作，但 ROOT **不支持**——每次都是全量重写，因为 ROOT 的每次更新都需要重新审视所有分支的全局关系。

---

## 四、辅助机制

### BubbleTracker（脏计数延迟）

```go
type BubbleTracker struct {
    mu     sync.RWMutex
    counts map[string]int
}
```

**键的格式**：
- Branch: `"{memoryId}::{typeName}"`（如 `"user1::identity"`）
- Root: `"{memoryId}::root::{rootTypeName}"`（如 `"user1::root::profile"`）

**流程**：`IncrementAndGet` → 比较阈值 → 达到则执行操作 → `Reset`。

**配置**（`config.go`）：
```go
BranchBubbleThreshold: 3   // LEAF 更新 3 次 → BRANCH 重摘要
RootBubbleThreshold:   2   // BRANCH 更新 2 次 → ROOT 重摘要
MinBranchesForRoot:    2   // 至少 2 个 BRANCH 才创建 ROOT
RootTargetTokens:      800 // ROOT LLM 输出的 token 预算
```

### PointIdentityManager（ID 管理）

```go
type PointIdentityManager struct{}
```

三个职责：

| 方法 | 功能 |
|------|------|
| `NormalizePersistedPoints` | 缺 pointId 的 points 补 SHA256(content) 哈希 |
| `NormalizeGeneratedOperations` | LLM 返回的 UPDATE/DELETE 的 pointId 若不存在，自动转为 ADD |
| `ReusePointIDsForFullRewrite` | 按 content hash 匹配现有 pointId，避免生成新 ID |

### EvidenceNormalizer（证据传播）

```go
type EvidenceNormalizer struct{}
```

Branch/Root 写入前自动处理：

1. 展平子节点的所有 points
2. 按 content 匹配设置 `sourcePointRefs`（Branch 指向 Leaf 的 point，Root 指向 Branch 的 point）
3. 从子节点收集去重的 `sourceItemIDs`
4. 设置 `metadata["tier"]` = "BRANCH" / "ROOT"

### GraphAssistant（可选增强）

```go
type GraphAssistant interface {
    BranchAssist(memoryID, insightType, leafInsights) *BranchAssist
    RootAssist(memoryID, rootType, branchInsights) *RootAssist
}
```

可选的排序/上下文增强接口。`NoOpGraphAssistant` 返回 nil，reorganizer 使用原始顺序。`resolveBranchAssist` 和 `resolveRootAssist` 统一处理：若 GraphAssistant 返回的排序列表长度不匹配，回退到原始顺序。

---

## 五、并发模型

| 组件 | 策略 |
|------|------|
| BubbleTracker | `sync.RWMutex`—每次 IncrementAndGet 加写锁 |
| ROOT 操作 | 16 条 `sync.Mutex` strip lock（按 memoryId 哈希分配） |
| 异步 ROOT | `sync.WaitGroup` + goroutine，track 在 `sync.Map` 中 |
| Point 操作 | 纯函数，无锁 |
| DrainRootTasks | channel + select + timeout，flush 时调用 |

Strip lock 索引：
```go
func (r *TreeReorganizer) rootLock(memoryID memind.MemoryId) *sync.Mutex {
    idx := hashString(memoryID.Identifier()) % r.lockStripes
    return &r.rootLocks[idx]
}
```

`hashString` 使用与 Java `String.hashCode()` 相同的多项式 `h = h*31 + c`。

---

## 六、数据模型

### MemoryInsight（三层节点通用）

```go
type MemoryInsight struct {
    ID               int64              // Store 分配
    MemoryID         string             // 归属
    Type             string             // InsightType 名称
    Scope            MemoryScope        // USER / AGENT
    Name             string
    Categories       []string
    Points           []InsightPoint     // 实际内容
    Group            string             // 语义组名称
    LastReasonedAt   *time.Time
    SummaryEmbedding []float32          // PointsContent 的向量
    CreatedAt        time.Time
    UpdatedAt        time.Time
    Tier             InsightTier        // LEAF / BRANCH / ROOT
    ParentInsightID  *int64             // 父节点 ID（Root 为 nil）
    ChildInsightIDs  []int64            // 子节点 ID 列表（Leaf 为空）
    Version          int                // 每次更新 +1
}
```

### InsightPoint

```go
type InsightPoint struct {
    PointID       string            // 唯一标识
    Type          PointType         // SUMMARY / REASONING
    Content       string            // 文本内容
    SourceItemIDs []string          // 原始 MemoryItem ID
    SourceRefs    []InsightPointRef // 子节点引用链
    Metadata      map[string]string // 标签
}
```

### InsightPointRef

```go
type InsightPointRef struct {
    InsightID int64   // 来源洞察 ID
    PointID   string  // 来源 Point ID
}
```

---

## 七、与旧版（tree.go）的关键差异

| 维度 | 旧版（已删除） | 新版 |
|------|---------------|------|
| 算法 | 机械拼接 + 阈值计数 | LLM 增量操作 / 全量重写 / 机械拼接三层回退 |
| BRANCH 摘要 | `mergePoints()` 拼接子点 | `resummarizeBranch()` LLM 聚合 |
| ROOT 合成 | `mergePoints()` 拼接 | `resummarizeRoot()` 四维度深度分析 |
| 脏计数 | 简单的 `MaxChildren` 阈值 | `BubbleTracker` 持久化计数器 |
| 并发 | 无 | Strip lock + 异步 goroutine |
| building架构 | `Promote()` 全局扫描 | `OnLeafsUpdated()` 按类型精确触发 |
| 证据传播 | 无 | `EvidenceNormalizer` 自动传播 |
| Point ID | 时间戳生成 | SHA256 content hash + 复用 |
| 数据模型 | 同 | 新增 `AnalysisMode` 字段 |

---

## 八、Flow 图

```
消息进入
    │
    ▼
extraction.Extract()
    │
    ├─ extractRawData()   → MemoryRawData
    ├─ extractItems()     → MemoryItem + 解析 InsightType
    └─ extractInsights()  → LEAF MemoryInsight (Tier=LEAF)
                                │
                                ▼
engine.Extract()
    │
    └─ 遍历 result.Items.Types
         │
         ▼
    reorganizer.OnLeafsUpdated(memoryID, typeName, insightType, leafs, config, lang)
         │
         ├─ (A) BubbleTracker.IncrementAndGet()
         │
         ├─ (B) getOrCreateBranch() + batchLinkLeafsToBranch()
         │
         ├─ (C) linkBranchToRoot() [strip lock]
         │
         ├─ [branchDirtyCount < threshold? → return]
         │
         ├─ (D) resummarizeBranch()
         │    ├── LLM GenerateBranchPointOps()     → 增量操作
         │    ├── fallback: LLM GenerateBranchSummary() → 全量重写
         │    └── fallback: fallbackBranchConcat()  → 机械拼接
         │
         └─ (E) bubbleAndMaybeResummarizeRoots()
              │
              └─ [rootDirtyCount >= threshold?]
                   └─ goroutine → resummarizeRoot() → LLM 四维度深度合成
```

---

## 九、关键文件索引

| 文件 | 内容 |
|------|------|
| `insight/reorganizer.go` | 核心重组织器（OnLeafsUpdated, resummarizeBranch, resummarizeRoot） |
| `insight/generator.go` | InsightGenerator 接口 + LlmInsightGenerator + NoOpInsightGenerator |
| `insight/operation.go` | Point 操作解析器（ADD/UPDATE/DELETE） |
| `insight/identity.go` | PointIdentityManager（ID 管理复用） |
| `insight/evidence.go` | EvidenceNormalizer（证据传播） |
| `insight/bubble.go` | BubbleTracker（脏计数延迟） |
| `insight/graph.go` | GraphAssistant 接口 + NoOpGraphAssistant |
| `insight/prompts.go` | LLM 提示词模板（Branch Ops / Branch Full / Root Synthesis） |
| `engine/memory.go` | 引擎入口，Extract() 调用链 |
| `store/inmemory.go` | 默认 InsightType 定义 |
| `config.go` | InsightTreeConfig 配置 |
| `models.go` | MemoryInsight / InsightPoint / InsightType 数据模型 |
