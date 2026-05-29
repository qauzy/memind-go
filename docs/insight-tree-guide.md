# 洞察树入门指南 — 从小白到精通

> 本文用**比喻 + 递进**的方式讲解。每章可以独立阅读，建议按顺序。

---

## 第一章：一句话说清「洞察树」是什么

**洞察树 = 把零散记忆整理成三层知识结构的系统。**

```
原始消息（零散）                   洞察树（结构）
"我喜欢吃披萨"          ──→     LEAF "食物偏好: 喜欢披萨"
"我讨厌下雨天"          ──→     LEAF "天气偏好: 讨厌下雨"
"我每天早上跑步"        ──→     LEAF "运动习惯: 晨跑"
                                    │
                          BRANCH "偏好: 食物+天气"
                          BRANCH "行为: 运动+..."
                                    │
                          ROOT "用户画像: 偏好+行为综合分析"
```

**记忆就像一堆散落的照片，洞察树是帮你把照片按时间、人物、地点整理进相册的人。**

---

## 第二章：先搞懂「为什么需要三层」

### 想象你在整理书房

| 层次 | 对应现实 | 洞察树 |
|------|----------|--------|
| **书桌上的便利贴** | 单条信息 | **LEAF**（叶子） |
| **文件夹里的分类资料** | 同类汇总 | **BRANCH**（树枝） |
| **书架上的百科全书** | 全局总结 | **ROOT**（树根） |

### 为什么不是两层？也不是四层？

- 一层（只有 LEAF）：信息太多，找起来慢
- 三层（LEAF → BRANCH → ROOT）：**性价比最高**
  - LEAF 保留细节
  - BRANCH 做主题聚合
  - ROOT 做跨主题洞察
- 四层以上：太复杂，维护成本高

> 这就像写笔记：你有原文（LEAF）、有摘要（BRANCH）、有读后感（ROOT）。

---

## 第三章：10 个分类 — 洞察树在记什么

洞察树把人相关的信息分成 **10 个主题**，每个主题就是一个 `InsightType`：

### 用户相关（USER 空间）

| 类型 | 大白话 | 例子 |
|------|--------|------|
| **identity**（身份） | 你是谁 | "我是程序员"、"我会说英语" |
| **preferences**（偏好） | 你喜欢什么 | "喜欢咖啡"、"讨厌周一" |
| **relationships**（关系） | 你认识谁 | "我老板是张三"、"好朋友是李四" |
| **experiences**（经历） | 你经历过什么 | "去年去了日本"、"在学吉他" |
| **behavior**（行为） | 你经常做什么 | "每天跑步"、"周末打游戏" |

### Agent 相关（AGENT 空间）

| 类型 | 大白话 | 例子 |
|------|--------|------|
| **directives**（指令） | 你要 AI 怎么做 | "回复要简洁" |
| **playbooks**（流程） | 事情怎么做 | "写周报的格式" |
| **resolutions**（解决方案） | 问题怎么修 | "服务器重启步骤" |

### 顶层合成（ROOT）

| 类型 | 大白话 | 作用 |
|------|--------|------|
| **profile**（画像） | 综合了解你这个人 | 把上面所有类型整合成完整画像 |
| **interaction**（交互） | AI 该怎么和你说话 | 语气、长度、风格 |

---

## 第四章：一条消息的旅程

**从你说了一句「我喜欢吃披萨」开始，到 ROOT 洞察「用户画像」更新，中间发生了什么？**

### 第 1 步：提取 — 把话变成数据

你说：「我喜欢吃披萨」

系统做了四件事：
1. **存原文** → `MemoryRawData`
2. **提条目** → `MemoryItem`（内容="我喜欢吃披萨"，分类=BEHAVIOR）
3. **按主题分组** → 同类型条目由 LLM 按语义聚类（InsightGroupPrompts），例如多条 preferences 条目会被分到"食物偏好"、"天气偏好"等子组
4. **每组生洞察** → `MemoryInsight`（Tier=LEAF，类型=preferences，名称=分组名）— 每组内的多条条目一起合成，而非每条单独生成

### 第 2 步：挂树 — 把叶子挂到树枝上

这条 LEAF 的类型是 `preferences`，系统会：

1. 找到或创建 `preferences` 类型的 **BRANCH**
2. 把这条 LEAF 的 `ParentInsightID` 指向 BRANCH
3. 把 BRANCH 的 `ChildInsightIDs` 加上这条 LEAF 的 ID

**结果**：BRANCH「preferences」现在知道它下面有一条 LEAF「喜欢披萨」。

### 第 3 步：等待 — 积攒够了再总结

系统不会每条消息都重新总结，那样太费钱了（LLM 调用要花钱）。

它用**记数器**：preferences 类型每多一条 LEAF，计数器 +1。

- 计数器 < 3：只挂树，不总结
- 计数器 >= 3：触发总结

### 第 4 步：总结 — 把同类信息压缩

当 preferences 积累了 3 条 LEAF：

```
LEAF 1: "我喜欢吃披萨"
LEAF 2: "我喜欢喝咖啡"
LEAF 3: "我不喜欢海鲜"
        │
        ▼
BRANCH preferences 的 Points:
  "用户偏好: 喜欢意大利菜和咖啡，避免海鲜"
```

这个总结可能由 AI 生成（更智能），也可以简单拼接（没 AI 时）。

### 第 5 步：升级 — 从 BRANCH 到 ROOT

preferences BRANCH 更新后，系统会检查所有 ROOT：

- **profile**（用户画像）的计数器 +1
- 如果 profile 的 >= 2，触发 ROOT 重总结

ROOT 总结会**跨类型分析**：

```
BRANCH identity: "我是程序员，会英语"
BRANCH preferences: "喜欢披萨和咖啡，不喜欢海鲜"
BRANCH experiences: "去年去了日本"
        │
        ▼
ROOT profile 的 Points:
  "用户是一位会英语的程序员，偏好西式饮食，
   有国际旅行经历，整体画像一致（CONVERGENCE）"
```

---

## 第五章：关键机制 — 像泡泡一样冒泡

**洞察树的更新不是一蹴而就的，而是像泡泡一样慢慢往上冒。**

```
时间线 →
──────────┬──────────┬──────────┬──────────┬──────────┬─────────
LEAF 到达 │ LEAF 到达 │ LEAF 到达 │          │ LEAF 到达 │
          │          │          │          │          │
计数器：1          2          3（触发）    重置0       1
          │          │          │          │          │
BRANCH    │          │     ────重摘要────    │          │
          │          │          │          │          │
ROOT 计数：                    1（+1）      2（触发）

                              └── goroutine ──→ ROOT 重摘要
```

**关键规则**：
- LEAF 到 BRANCH：阈值 = 3（默认，可配）
- BRANCH 到 ROOT：阈值 = 2（默认，可配）
- ROOT 重摘要在**后台 goroutine** 异步执行，不阻塞主流程

---

## 第六章：三种总结方式 — 从智能到简单

系统有三种方式生成 BRANCH 摘要，**智能降级**：

```
┌─ 方式一：增量操作 ──────────────────────────┐
│  效果：AI 只告诉你「加什么、改什么、删什么」    │
│  成本：低（只需传变化部分）                    │
│  依赖：需要 AI                              │
│  例子："ADD 一条'喜欢咖啡'，DELETE '喜欢茶'"    │
└────────────────────────────────────────────┘
        │ 失败（AI 不支持、解析出错）
        ▼
┌─ 方式二：全量重写 ───────────────────────────┐
│  效果：AI 完整重新生成所有内容                 │
│  成本：高（全部重传）                         │
│  依赖：需要 AI                              │
│  例子：完全新写 3 条 SUMMARY                 │
└────────────────────────────────────────────┘
        │ 失败（AI 不可用、返回空）
        ▼
┌─ 方式三：机械拼接 ──────────────────────────┐
│  效果：把子节点的内容挨个拼起来                 │
│  成本：免费                                  │
│  依赖：不需要 AI                             │
│  例子：直接复制所有 LEAF 的原文到 BRANCH       │
└────────────────────────────────────────────┘
```

**ROOT 只有方式二和方式三**（没有增量操作），因为 ROOT 每次都需要全局视角。

---

## 第七章：三个辅助角色

### 1. ID 管家（PointIdentityManager）

每个洞察点（InsightPoint）有一个唯一 ID。管家负责：
- **缺 ID 时自动生成**：用内容算 SHA256 哈希
- **复用旧 ID**：内容相同就用旧 ID，方便追踪变化
- **纠正 AI 错误**：AI 说要 UPDATE 一个不存在的 ID，管家自动转成 ADD

### 2. 证据传递员（EvidenceNormalizer）

负责把「证据链」从底层传递到顶层：

```
Item #1 "喜欢披萨" ← sourceItemID
        │
LEAF preferences
  Point: "喜欢披萨" ← sourceItemIDs=["1"]
        │ sourceRefs
        ▼
BRANCH preferences
  Point: "偏好意式" ← sourceItemIDs=["1","2"]
                     ← sourceRefs=[{insightID=LEAF, pointID="..."}]
        │ sourceRefs
        ▼
ROOT profile
  Point: "饮食偏好" ← sourceItemIDs=["1","2","3"]
                     ← sourceRefs=[{insightID=BRANCH, pointID="..."}]
```

这样你随时可以回答：「ROOT 的这条结论，最初来自哪几条原始消息？」

### 3. 泡泡计数器（BubbleTracker）

就是一个**计数器**，记录每个类型「有多少条 LEAF 是上次总结之后新增的」。

```
memoryId::preferences → 3    ← 达到阈值，该总结了
memoryId::profile     → 2    ← 达到阈值，该总结了
```

为什么叫「泡泡」？因为像气泡一样积累到一定程度就冒上来（触发操作）。

---

## 第八章：并发 — 多用户同时使用怎么办

系统设计了**三级并发防护**：

### 第一级：Strip Lock（条形锁）

16 把锁，每个用户（memoryId）分配一把：

```
用户 A（memoryId = "alice"）→ hash("alice") % 16 = 锁 #3
用户 B（memoryId = "bob"）  → hash("bob") % 16   = 锁 #7
用户 C（memoryId = "charlie"）→ hash("charlie") % 16 = 锁 #3（和 Alice 同一把）
```

同一把锁的用户串行操作 ROOT，不同锁的用户可以并行。

### 第二级：异步 ROOT

ROOT 重摘要在**后台**运行，不阻塞主流程：

```go
go func() {
    resummarizeRoot(...)  // 异步执行
}()
```

主线程可以继续处理下一条消息。

### 第三级：等待机制（Drain）

当需要确保所有 ROOT 都更新完毕时（比如关机前），可以：

```go
reorganizer.DrainRootTasks(memoryID, 5*time.Second)
```

等所有后台任务完成（最多等 5 秒），超时就放弃。

---

## 第九章：怎么用代码触发一次完整流程

### 最小示例

```go
// 1. 构建引擎
mem := engine.Builder().Build()
defer mem.Close()

// 2. 发送消息
mem.AddMessage(memoryID, message, config)

// 3. 提交（触发提取 + 树构建）
mem.Commit(memoryID, config)
```

### 拆解执行过程

```go
// 引擎内部实际做的是：
// 1. AddMessage 把消息放入缓冲区
// 2. Commit 触发：
extractor.Extract(request)
    // 2a. extractRawData — 存原文
    // 2b. extractItems — 提条目
    // 2c. extractInsights — 生成 LEAF

// 3. 引擎遍历新建的洞察类型
for _, t := range result.Items.Types {
    reorganizer.OnLeafsUpdated(memoryID, t.Name, t, leafs, config, language)
    //    ↑ 这就是本文讲的所有流程
}
```

### 完整流程回溯

```
AddMessage → 缓冲区 → Commit → Extract()
                                    │
                                    ├─ RawData → Item (分类)
                                    │
                                    └─ → 按类型收集条目
                                         │
                                         ├─ [有 LLM] groupItemsForType — 语义分组（InsightGroupPrompts）
                                         │     每个分组内 generateLeafInsights — 多条目合成 LEAF（InsightLeafPrompts）
                                         │     SUMMARY="整合多源事实"，REASONING="推导隐含结论"
                                         │
                                         └─ [无 LLM] 每一条目生成单点摘要
                                              │
                                              ▼
                                         LEAF 洞察
                                              │
                                              ▼
                                         OnLeafsUpdated()
                                              │
                                              ├─ Bubble 计数
                                              ├─ 创建/链接 BRANCH
                                              ├─ 链接 ROOT
                                              ├─ [阈值达标] BRANCH 重摘要（LLM 三层回退）
                                              └─ [阈值达标] ROOT 异步重摘要（goroutine）
```

---

## 第十章：常见问题

### Q：没有 AI 的时候能用吗？

能。所有 LLM 回退都有机械拼接兜底。只是没有 AI 时 BRANCH 和 ROOT 的摘要质量会差一些（相当于直接把原文堆在一起）。

### Q：阈值设多少合适？

| 场景 | Branch 阈值 | Root 阈值 | 原因 |
|------|------------|-----------|------|
| 对话密集 | 5-10 | 3-5 | 消息多，太频繁浪费钱 |
| 对话稀疏 | 2-3 | 2 | 消息少，来了就总结 |
| 调试/测试 | 1 | 1 | 每条都触发，方便观察 |

### Q：LEAF 和 BRANCH 和 ROOT 存在哪里？

都存在 `store.MemoryStore.Insights()` 中。目前支持：
- **内存**（`InMemoryStore`）— 重启丢失
- **SQLite**（`NewSQLiteStore`）— 持久化到本地文件
- **MySQL**（`NewMySQLStore`）— 持久化到远程数据库

### Q：怎么查看当前的洞察树？

```go
// 通过 store 直接查
store.Insights().GetInsightsByTier(memoryID, TierLeaf)   // 查 LEAF
store.Insights().GetInsightsByTier(memoryID, TierBranch) // 查 BRANCH
store.Insights().GetInsightsByTier(memoryID, TierRoot)   // 查 ROOT

// 按类型查
store.Insights().GetBranchByType(memoryID, "preferences") // 查某个类型的 BRANCH
store.Insights().GetRootByType(memoryID, "profile")       // 查某个类型的 ROOT
```

---

## 路线图：从入门到精通

```
第 1 步：【概念】读本文第 1-3 章
  → 明白三层结构和 10 个分类

第 2 步：【流程】读第 4 章
  → 理解一条消息的完整旅程

第 3 步：【机制】读第 5-7 章
  → 理解泡泡、ID、证据链

第 4 步：【并发】读第 8 章
  → 理解多用户处理

第 5 步：【动手】跑测试
  → go test ./... -count=1 -v
  → 在 TestExtractAndRetrieve 打断点跟踪

第 6 步：【读代码】按调用链
  → engine/memory.go:58 → Extract()
  → extraction/extractor.go:54 → Extract() → extractInsights()
  → （内部：groupItemsForType at :595 → generateLeafInsights at :650）
  → insight/reorganizer.go:63 → OnLeafsUpdated()
  → insight/reorganizer.go:256 → resummarizeBranch()
  → insight/reorganizer.go:457 → resummarizeRoot()

第 7 步：【改代码】试着改阈值
  → config.go:254 → DefaultInsightTreeConfig()
  → 把 BranchBubbleThreshold 改成 1
  → 观察每条消息都触发总结的效果

第 8 步：【深入】读原版 Java
  → /opt/code/memind/memind-core/.../InsightTreeReorganizer.java
  → 对比 Go 版理解移植思路
```

---

---

## 附录：分类到底怎么算的（代码溯源）

### 核心结论
/opt/code/memind-go/extraction/extractor.go:43
分类走**两路策略**：

| 路径 | 怎么分 | 效果 |
|------|--------|------|
| **LLM 语义（首选）** | LLM 根据决策表 + 正反例判断 | 精准：「披萨→BEHAVIOR」「周五下雨→EVENT」 |
| **哈希分桶（回退）** | `simpleHash(caption) % len(categories)` | 均匀但无语义 |

- 有 LLM 时（配置了 `SlotItemExtraction`）：用 Java 移植的 `MemoryItemUnifiedPrompts`，LLM 同时给出 category 和 insightTypes
- 无 LLM 时：哈希分桶，对 `"我喜欢吃披萨"`，`simpleHash` 首字节 `'a'=97`，`97 % 3 = 1` → `UserCategories()[1] = BEHAVIOR`

### 代码链路


提取入口
  └─ [/extraction/extractor.go:54](/extraction/extractor.go#L54)  `Extract()` → extractRawData → extractItems → extractInsights

原文封装
  └─ [/extraction/extractor.go:159](/extraction/extractor.go#L159)  `MemoryRawData{ Caption: "我喜欢吃披萨" }`

分类（两路）：
  │
  ├─ [有 LLM] [/extraction/extractor.go:221](/extraction/extractor.go#L221)  `extractItemsWithLLM()`
  │     └─ [/extraction/item_extraction_prompts.go:10](/extraction/item_extraction_prompts.go#L10)  MemoryItemUnifiedPrompts（决策表 + 正反例）
  │     └─ 输出：{content, category=BEHAVIOR, insightTypes=[preferences]}
  │
  └─ [无 LLM] [/extraction/extractor.go:353](/extraction/extractor.go#L353)  `extractItemsHash()`
        └─ [/models.go:42](/models.go#L42)  `UserCategories() = [PROFILE, BEHAVIOR, EVENT]`
        └─ [/extraction/extractor.go:383](/extraction/extractor.go#L383)  `simpleHash(caption)[0] % len(categories)`

Item 写入
  └─ [/extraction/extractor.go:305](/extraction/extractor.go#L305)  `MemoryItem{ Category: BEHAVIOR, Metadata: {insightTypes: [preferences]} }`

匹配洞察类型
  └─ [/extraction/extractor.go:556](/extraction/extractor.go#L556)  `resolveItemInsightTypes()` — 优先读 Metadata.insightTypes（LLM 语义），
  └─ 回退 typeMatchesCategory(item.Category, insightType) 逐项匹配

生成 LEAF（分组合成流程）
  └─ [/extraction/extractor.go:448](/extraction/extractor.go#L448)  `extractInsights()` → 按类型收集条目
       │
       ├─ [有 LLM] [/extraction/extractor.go:595](/extraction/extractor.go#L595)  `groupItemsForType()` — InsightGroupPrompts 语义分组
       │     └─ [/extraction/extractor.go:650](/extraction/extractor.go#L650)  `generateLeafInsights()` — InsightLeafPrompts 多条目合成
       │           输出：SUMMARY（整合多源）/ REASONING（推导隐含）
       │
       └─ [无 LLM] 每条目生成单点摘要
            └─ `MemoryInsight{ Type: "preferences", Name: 分组名, Tier: LEAF }`

ROOT 更新
  └─ [/engine/memory.go:71](/engine/memory.go#L71)  `Extract()` → `OnLeafsUpdated()`
  └─ [/config.go:254](/config.go#L254)  阈值: BranchBubbleThreshold=3, RootBubbleThreshold=2


### 相比原版 Java

Go 版已从 Java 移植了：
- **语义分类**：`MemoryItemUnifiedPrompts.java` → `item_extraction_prompts.go`（决策表、正反例、LLM 判断 category/insightTypes）
- **语义分组**：`InsightGroupPrompts.java` → `insight_group_prompts.go`（多条目语义聚类，命名空间稳定性）
- **多条目合成**：`InsightLeafPrompts.java` → `insight_leaf_prompts.go`（每组内 SUMMARY/REASONING 合成，拒绝单条目重复）

哈希路径仅作为 LLM 不可用时的回退。

---

## 术语对照表

| 英文 | 中文 | 大白话 |
|------|------|--------|
| LEAF | 叶子节点 | 单条洞察 |
| BRANCH | 树枝节点 | 同主题汇总 |
| ROOT | 根节点 | 全局分析 |
| InsightType | 洞察类型 | 分类主题 |
| Tier | 层级 | LEAF/BRANCH/ROOT |
| Point | 洞察点 | 一段具体的洞察文本 |
| Bubble | 泡泡 | 脏计数器 |
| Threshold | 阈值 | 触发条件 |
| Fallback | 回退 | 备选方案 |
| Strip Lock | 条形锁 | 按用户分配的锁 |
| Goroutine | Go 协程 | 轻量级后台任务 |
