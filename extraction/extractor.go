package extraction

import (
	"encoding/json"
	"fmt"
	"time"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/buffer"
	"github.com/openmemind/memind-go/llm"
	"github.com/openmemind/memind-go/store"
	tsearch "github.com/openmemind/memind-go/textsearch"
	"github.com/openmemind/memind-go/vector"
)

// MemoryExtractor - 提取管线接口
type MemoryExtractor interface {
	Extract(req memind.ExtractionRequest) (*memind.ExtractionResult, error)
	AddMessage(memoryID memind.MemoryId, msg memind.Message, config memind.ExtractionConfig) (*memind.ExtractionResult, error)
}

// DefaultExtractor - 默认提取器，按 RawData → Item → Insight 三阶段执行
type DefaultExtractor struct {
	memStore   store.MemoryStore
	buf        buffer.MemoryBuffer
	vector     vector.MemoryVector
	textSearch tsearch.MemoryTextSearch
	llm        *llm.ChatClientRegistry
	opts       memind.ExtractionOptions
}

// NewExtractor - 创建默认提取器实例
func NewExtractor(
	memStore store.MemoryStore,
	buf buffer.MemoryBuffer,
	vec vector.MemoryVector,
	ts tsearch.MemoryTextSearch,
	llm *llm.ChatClientRegistry,
	opts memind.ExtractionOptions,
) *DefaultExtractor {
	return &DefaultExtractor{
		memStore:   memStore,
		buf:        buf,
		vector:     vec,
		textSearch: ts,
		llm:        llm,
		opts:       opts,
	}
}

// Extract - 执行完整提取流程：原始内容 → 原始数据 → 条目 → 洞察
func (e *DefaultExtractor) Extract(req memind.ExtractionRequest) (*memind.ExtractionResult, error) {
	start := time.Now()
	cfg := req.Config
	if cfg == (memind.ExtractionConfig{}) {
		cfg = memind.DefaultExtractionConfig()
	}

	rawDataResult, err := e.extractRawData(req.MemoryID, req.Content, req.Metadata, cfg)
	if err != nil {
		return nil, &memind.ExtractionError{Stage: "rawdata", Message: "raw data extraction failed", Err: err}
	}

	itemResult, err := e.extractItems(req.MemoryID, rawDataResult, cfg, "")
	if err != nil {
		return nil, &memind.ExtractionError{Stage: "item", Message: "item extraction failed", Err: err}
	}

	insightResult, err := e.extractInsights(req.MemoryID, itemResult, cfg)
	if err != nil {
		return nil, &memind.ExtractionError{Stage: "insight", Message: "insight extraction failed", Err: err}
	}

	status := memind.ExtractionSuccess
	duration := time.Since(start)

	return &memind.ExtractionResult{
		MemoryID: req.MemoryID,
		RawData:  memind.RawDataResult{RawDataList: rawDataResult.RawDataList, Existed: rawDataResult.Existed},
		Items:    memind.MemoryItemResult{NewItems: itemResult.NewItems, Types: itemResult.Types},
		Insights: memind.InsightResult{Insights: insightResult.Insights},
		Status:   status,
		Duration: duration,
	}, nil
}

// AddMessage - 添加消息到缓冲区，达到批处理阈值时自动触发提取
func (e *DefaultExtractor) AddMessage(memoryID memind.MemoryId, msg memind.Message, config memind.ExtractionConfig) (*memind.ExtractionResult, error) {
	if err := e.buf.PendingConversation().Add(memoryID, msg); err != nil {
		return nil, err
	}
	if err := e.buf.RecentConversation().Add(memoryID, msg); err != nil {
		return nil, err
	}

	pending, _ := e.buf.PendingConversation().Get(memoryID)
	if len(pending) >= e.opts.Common.MaxMessageBatchSize {
		return e.commitMessages(memoryID, config)
	}
	return &memind.ExtractionResult{
		MemoryID: memoryID,
		Status:   memind.ExtractionSuccess,
	}, nil
}

// commitMessages - 将待提交消息拼接为对话文本并执行提取
func (e *DefaultExtractor) commitMessages(memoryID memind.MemoryId, config memind.ExtractionConfig) (*memind.ExtractionResult, error) {
	msgs, err := e.buf.PendingConversation().Get(memoryID)
	if err != nil || len(msgs) == 0 {
		return &memind.ExtractionResult{MemoryID: memoryID, Status: memind.ExtractionSuccess}, nil
	}
	defer e.buf.PendingConversation().Clear(memoryID)

	var text string
	for i, msg := range msgs {
		role := "user"
		if msg.Role == memind.RoleAssistant {
			role = "assistant"
		}
		var content string
		for _, block := range msg.Content {
			content += block.Text + " "
		}
		text += fmt.Sprintf("[%s] %s\n", role, content)
		if i < len(msgs)-1 {
			text += "\n"
		}
	}

	rawContent := memind.RawContent{
		Type:    "ConversationContent",
		Content: text,
	}
	return e.Extract(memind.ExtractionRequest{
		MemoryID: memoryID,
		Content:  rawContent,
		Config:   config,
	})
}

// extractRawData - 第一阶段：将原始内容封装为 MemoryRawData 并存储
func (e *DefaultExtractor) extractRawData(memoryID memind.MemoryId, content memind.RawContent, metadata map[string]any, cfg memind.ExtractionConfig) (*RawDataExtractResult, error) {
	if !e.opts.RawData.Enabled {
		existing, _ := e.memStore.RawData().ListRawData(memoryID)
		if len(existing) > 0 {
			return &RawDataExtractResult{RawDataList: existing, Existed: true}, nil
		}
	}

	contentID := fmt.Sprintf("content-%d", time.Now().UnixNano())
	now := time.Now()
	rd := &memind.MemoryRawData{
		ID:          fmt.Sprintf("rd-%d", time.Now().UnixNano()),
		MemoryID:    memoryID.Identifier(),
		ContentType: content.Type,
		ContentID:   contentID,
		Caption:     content.Content,
		Metadata:    metadata,
		CreatedAt:   now,
	}

	if err := e.memStore.RawData().UpsertRawData(memoryID, []*memind.MemoryRawData{rd}); err != nil {
		return nil, err
	}

	if e.vector != nil {
		vecID, err := e.vector.Store(memoryID, content.Content, nil)
		if err == nil {
			rd.CaptionVectorID = vecID
		}
	}

	return &RawDataExtractResult{
		RawDataList: []*memind.MemoryRawData{rd},
	}, nil
}

// extractItems - 第二阶段：从原始数据中提取结构化 MemoryItem
func (e *DefaultExtractor) extractItems(memoryID memind.MemoryId, rawResult *RawDataExtractResult, cfg memind.ExtractionConfig, language string) (*ItemExtractResult, error) {
	// 如果 Item 提取被禁用，直接返回空结果
	if !e.opts.Item.Enabled {
		return &ItemExtractResult{}, nil
	}

	var itemTypes []memind.MemoryInsightType // 本轮新条目关联的洞察类型列表
	var newItems []*memind.MemoryItem        // 本轮实际新增的条目
	now := time.Now()

	// ---- 遍历每个原始数据，构造条目 ----
	for _, rd := range rawResult.RawDataList {
		// 对 caption 计算哈希，用于内容级去重
		hash := simpleHash(rd.Caption)
		existing, _ := e.memStore.Items().GetItemByHash(memoryID, hash)
		if existing != nil {
			continue // 相同内容已存在，跳过
		}

		// 根据 scope 选择分类列表：USER 用 UserCategories，AGENT 用 AgentCategories
		scope := cfg.Scope
		categories := memind.UserCategories()
		if scope == memind.ScopeAgent {
			categories = memind.AgentCategories()
		}

		// 用 hash 首字节对分类数取模，均匀分布条目到各类
		category := categories[0]
		if len(categories) > 1 {
			idx := 0
			for _, c := range hash {
				idx += int(c)
				break
			}
			category = categories[idx%len(categories)]
		}

		// 构造 MemoryItem，初始类型固定为 FACT
		item := &memind.MemoryItem{
			MemoryID:    memoryID.Identifier(),
			Content:     rd.Caption,
			Scope:       scope,
			Category:    category,
			ContentType: rd.ContentType,
			RawDataID:   rd.ID,
			ContentHash: hash,
			CreatedAt:   now,
			ObservedAt:  &now,
			Type:        memind.ItemTypeFact,
		}
		newItems = append(newItems, item)

		// 从 store 加载所有洞察类型，筛选与当前 scope 匹配的
		insightTypes, _ := e.memStore.Insights().ListInsightTypes()
		for _, it := range insightTypes {
			if it.Scope == scope {
				itemTypes = append(itemTypes, *it)
			}
		}
	}

	// ---- 若有新条目，执行持久化和索引构建 ----
	if len(newItems) > 0 {
		// ① 持久化到 store，自动分配 item.ID
		if err := e.memStore.Items().UpsertItems(memoryID, newItems); err != nil {
			return nil, err
		}

		for _, item := range newItems {
			// ② 建立 BM25 全文索引，docID = "item-{id}"
			docID := fmt.Sprintf("item-%d", item.ID)
			e.textSearch.Index(memoryID, docID, item.Content, tsearch.TargetItem)

			// ③ 生成向量嵌入并存入向量索引，vectorID 写回 item
			if e.vector != nil {
				vecID, _ := e.vector.Store(memoryID, item.Content, map[string]any{"type": "item", "item_id": item.ID})
				item.VectorID = vecID
			}

			// ④ 将 (itemID, insightTypeName) 推入 InsightBuffer
			//    第三阶段 extractInsights 会消费这些记录来生成洞察
			if e.buf != nil {
				for _, it := range itemTypes {
					e.buf.InsightBuffer().Add(memoryID, item.ID, it.Name)
				}
			}
		}
	}

	// 去重洞察类型后返回
	uniqTypes := dedupTypes(itemTypes)
	return &ItemExtractResult{
		NewItems: newItems,
		Types:    uniqTypes,
	}, nil
}

// extractInsights - 第三阶段：基于提取的条目生成洞察
//
// 对每个 (newItem × insightType) 组合执行：
//  1. 调用 LLM（SlotInsightGenerator）从条目内容中提取结构化 InsightPoint
//  2. 若无 LLM 或调用失败，回退为将条目内容包为单个 SUMMARY 点
//  3. 构造 MemoryInsight（Tier=Leaf），持久化到 store
//  4. 建立 BM25 全文索引（docID = "insight-{id}"）
//  5. 对 PointsContent 生成向量嵌入，存入向量索引
//  6. 收集所有新洞察返回
func (e *DefaultExtractor) extractInsights(memoryID memind.MemoryId, itemResult *ItemExtractResult, cfg memind.ExtractionConfig) (*InsightExtractResult, error) {
	if !cfg.EnableInsight || !e.opts.Insight.Enabled || len(itemResult.NewItems) == 0 {
		return &InsightExtractResult{}, nil
	}

	llmClient := e.llm.Resolve(llm.SlotInsightGenerator)
	now := time.Now()
	var insights []*memind.MemoryInsight

	// 为每个 item 在每个匹配的洞察类型下生成洞察
	for _, item := range itemResult.NewItems {
		for _, t := range itemResult.Types {
			// 检查 item 的 category 是否匹配洞察类型的 categories
			if !typeMatchesCategory(t, item.Category) {
				continue
			}

			// 调用 LLM 或回退，生成洞察点列表
			points := e.generateInsightPoints(llmClient, item, t, cfg.Language)

			if len(points) == 0 {
				continue
			}

			ins := &memind.MemoryInsight{
				MemoryID:  memoryID.Identifier(),
				Type:      t.Name,
				Scope:     t.Scope,
				Name:      t.Name,
				Points:    points,
				CreatedAt: now,
				UpdatedAt: now,
				Tier:      memind.TierLeaf,
				Version:   1,
			}

			// 持久化到 store（自动分配 ins.ID）
			if err := e.memStore.Insights().UpsertInsights(memoryID, []*memind.MemoryInsight{ins}); err != nil {
				return nil, fmt.Errorf("upsert insight: %w", err)
			}

			// BM25 全文索引
			if e.textSearch != nil {
				docID := fmt.Sprintf("insight-%d", ins.ID)
				_ = e.textSearch.Index(memoryID, docID, ins.PointsContent(), tsearch.TargetInsight)
			}

			// 向量索引
			if e.vector != nil {
				vecID, _ := e.vector.Store(memoryID, ins.PointsContent(), map[string]any{"type": "insight", "insight_id": ins.ID})
				ins.SummaryEmbedding = nil
				_ = vecID
			}

			insights = append(insights, ins)
		}
	}

	return &InsightExtractResult{Insights: insights}, nil
}

// generateInsightPoints - 调用 LLM 从条目内容中提取洞察点，无 LLM 时回退为摘要
func (e *DefaultExtractor) generateInsightPoints(client llm.StructuredChatClient, item *memind.MemoryItem, typeDef memind.MemoryInsightType, language string) []memind.InsightPoint {
	// 先尝试 LLM 提取
	if _, ok := client.(*llm.NoOpChatClient); !ok {
		resp, err := client.Call([]llm.ChatMessage{
			{Role: llm.RoleSystem, Content: insightSystemPrompt},
			{Role: llm.RoleUser, Content: fmt.Sprintf(insightUserPrompt,
				typeDef.Name, typeDef.Description, item.Content, item.ID)},
		})
		if err == nil && resp != "" {
			var points []memind.InsightPoint
			if err := json.Unmarshal([]byte(resp), &points); err == nil && len(points) > 0 {
				return points
			}
		}
	}

	// 回退：将条目内容包为单个 SUMMARY 点
	return []memind.InsightPoint{
		{
			PointID:       fmt.Sprintf("sp-%d", time.Now().UnixNano()),
			Type:          memind.PointTypeSummary,
			Content:       item.Content,
			SourceItemIDs: []string{fmt.Sprintf("%d", item.ID)},
		},
	}
}

// typeMatchesCategory - 检查 item 的 category 是否在洞察类型的 categories 列表中
func typeMatchesCategory(t memind.MemoryInsightType, cat memind.MemoryCategory) bool {
	for _, c := range t.Categories {
		if string(cat) == c {
			return true
		}
	}
	return false
}

// insightSystemPrompt - 洞察提取的系统提示词
const insightSystemPrompt = `You are an insight extraction system. Extract structured insights concisely.
Return ONLY a JSON array of objects, each with:
- "pointId": a unique string identifier
- "type": "SUMMARY" or "REASONING"
- "content": the insight text in the original language
- "sourceItemIds": array of source item ID strings`

// insightUserPrompt - 洞察提取的用户提示词模板
// 参数依次为：洞察类型名、类型描述、条目内容、条目 ID
const insightUserPrompt = `Extract structured insight points about "%s" from the following information.

Type: %s
Content: %s

Return a JSON array of insight points referencing source item "%d".`

// simpleHash - 基于字符串的简单哈希函数，用于去重
func simpleHash(s string) string {
	var h int64 = 0
	for _, c := range s {
		h = h*31 + int64(c)
	}
	return fmt.Sprintf("%x", h)
}

// dedupTypes - 按名称去重洞察类型列表
func dedupTypes(types []memind.MemoryInsightType) []memind.MemoryInsightType {
	seen := make(map[string]bool)
	var result []memind.MemoryInsightType
	for _, t := range types {
		if !seen[t.Name] {
			seen[t.Name] = true
			result = append(result, t)
		}
	}
	return result
}
