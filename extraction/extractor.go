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
// 定义两层提取入口：Extract（直接提取）和 AddMessage（缓冲后提取）
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
		Insights: memind.InsightResult{Insights: insightResult.Insights, ByType: insightResult.ByType},
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
//
// 两路策略：
//  1. LLM 语义分类（首选）：调用 SlotItemExtraction 槽位的 LLM，传入完整对话文本，
//     由 LLM 根据语义决定每个条目的 category 和 insightTypes。
//     返回的每个条目是原子事实（非原始文本），具有精准的分类归属。
//  2. 哈希分桶（回退）：无 LLM 时用 simpleHash 首字节对分类数量取模，
//     均匀但不区分语义。条目内容保持原始文本。
//
// LLM 路径产生的条目将 insightTypes 存入 item.Metadata["insightTypes"]，
// extractInsights 后续会消费此字段精确定位应生成的洞察类型。
func (e *DefaultExtractor) extractItems(memoryID memind.MemoryId, rawResult *RawDataExtractResult, cfg memind.ExtractionConfig, language string) (*ItemExtractResult, error) {
	if !e.opts.Item.Enabled {
		return &ItemExtractResult{}, nil
	}

	llmClient := e.llm.Resolve(llm.SlotItemExtraction)
	if _, ok := llmClient.(*llm.NoOpChatClient); !ok {
		return e.extractItemsWithLLM(memoryID, rawResult, cfg, llmClient)
	}
	return e.extractItemsHash(memoryID, rawResult, cfg)
}

// extractItemsWithLLM - LLM 语义分类路径
// Modified: 2026-05-28 - 从 Java MemoryItemUnifiedPrompts / LlmItemExtractionStrategy 移植
func (e *DefaultExtractor) extractItemsWithLLM(memoryID memind.MemoryId, rawResult *RawDataExtractResult, cfg memind.ExtractionConfig, client llm.StructuredChatClient) (*ItemExtractResult, error) {
	scope := cfg.Scope

	// 从所有 RawData 拼接对话文本
	var conversation string
	for i, rd := range rawResult.RawDataList {
		if i > 0 {
			conversation += "\n"
		}
		conversation += rd.Caption
	}

	// 加载 scope 匹配的洞察类型列表
	allTypes, _ := e.memStore.Insights().ListInsightTypes()
	var scopeTypes []memind.MemoryInsightType
	for _, it := range allTypes {
		if it.Scope == scope {
			scopeTypes = append(scopeTypes, *it)
		}
	}

	// 调用 LLM
	var resp llmItemExtractionResponse
	err := client.CallStructured([]llm.ChatMessage{
		{Role: llm.RoleSystem, Content: itemExtractionSystemPrompt},
		{Role: llm.RoleUser, Content: fmt.Sprintf(itemExtractionUserPrompt, conversation)},
	}, &resp)
	if err != nil {
		return nil, fmt.Errorf("llm item extraction: %w", err)
	}

	if len(resp.Items) == 0 {
		return &ItemExtractResult{}, nil
	}

	now := time.Now()
	var newItems []*memind.MemoryItem

	for _, extracted := range resp.Items {
		if extracted.Content == "" {
			continue
		}
		if extracted.Category == "" {
			continue
		}

		hash := simpleHash(extracted.Content)
		existing, _ := e.memStore.Items().GetItemByHash(memoryID, hash)
		if existing != nil {
			continue
		}

		// 校验 LLM 返回的分类是否合法，不合法则跳过
		cat := memind.MemoryCategory(extracted.Category)
		if !isValidCategory(cat, scope) {
			continue
		}

		// 过滤 insightTypes：只保留在 scopeTypes 中存在的类型名
		var validTypes []string
		for _, tName := range extracted.InsightTypes {
			for _, st := range scopeTypes {
				if st.Name == tName {
					validTypes = append(validTypes, tName)
					break
				}
			}
		}
		if len(validTypes) == 0 {
			continue
		}

		item := &memind.MemoryItem{
			MemoryID:    memoryID.Identifier(),
			Content:     extracted.Content,
			Scope:       scope,
			Category:    cat,
			RawDataID:   rawResult.RawDataList[0].ID,
			ContentHash: hash,
			Metadata:    map[string]any{"insightTypes": validTypes},
			CreatedAt:   now,
			ObservedAt:  &now,
			Type:        memind.ItemTypeFact,
		}
		newItems = append(newItems, item)
	}

	if len(newItems) == 0 {
		return &ItemExtractResult{}, nil
	}

	if err := e.memStore.Items().UpsertItems(memoryID, newItems); err != nil {
		return nil, err
	}

	for _, item := range newItems {
		docID := fmt.Sprintf("item-%d", item.ID)
		e.textSearch.Index(memoryID, docID, item.Content, tsearch.TargetItem)

		if e.vector != nil {
			vecID, _ := e.vector.Store(memoryID, item.Content, map[string]any{"type": "item", "item_id": item.ID})
			item.VectorID = vecID
		}

		if e.buf != nil {
			types := item.Metadata["insightTypes"].([]string)
			for _, tName := range types {
				e.buf.InsightBuffer().Add(memoryID, item.ID, tName)
			}
		}
	}

	return &ItemExtractResult{
		NewItems: newItems,
		Types:    scopeTypes,
	}, nil
}

// extractItemsHash - 哈希分桶回退路径
// Modified: 2026-05-28 - 从原 extractItems 提取为独立方法
func (e *DefaultExtractor) extractItemsHash(memoryID memind.MemoryId, rawResult *RawDataExtractResult, cfg memind.ExtractionConfig) (*ItemExtractResult, error) {
	scope := cfg.Scope
	categories := memind.UserCategories()
	if scope == memind.ScopeAgent {
		categories = memind.AgentCategories()
	}

	allTypes, _ := e.memStore.Insights().ListInsightTypes()
	var scopeTypes []memind.MemoryInsightType
	for _, it := range allTypes {
		if it.Scope == scope {
			scopeTypes = append(scopeTypes, *it)
		}
	}

	now := time.Now()
	var newItems []*memind.MemoryItem

	for _, rd := range rawResult.RawDataList {
		hash := simpleHash(rd.Caption)
		existing, _ := e.memStore.Items().GetItemByHash(memoryID, hash)
		if existing != nil {
			continue
		}

		category := categories[0]
		if len(categories) > 1 {
			idx := 0
			for _, c := range hash {
				idx += int(c)
				break
			}
			category = categories[idx%len(categories)]
		}

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
	}

	if len(newItems) == 0 {
		return &ItemExtractResult{}, nil
	}

	if err := e.memStore.Items().UpsertItems(memoryID, newItems); err != nil {
		return nil, err
	}

	for _, item := range newItems {
		docID := fmt.Sprintf("item-%d", item.ID)
		e.textSearch.Index(memoryID, docID, item.Content, tsearch.TargetItem)

		if e.vector != nil {
			vecID, _ := e.vector.Store(memoryID, item.Content, map[string]any{"type": "item", "item_id": item.ID})
			item.VectorID = vecID
		}

		if e.buf != nil {
			for _, it := range scopeTypes {
				e.buf.InsightBuffer().Add(memoryID, item.ID, it.Name)
			}
		}
	}

	return &ItemExtractResult{
		NewItems: newItems,
		Types:    scopeTypes,
	}, nil
}

// extractInsights - 第三阶段：基于提取的条目生成洞察
//
// 分发逻辑：
//   - 若 item.Metadata["insightTypes"] 存在（LLM 路径），仅对该列表中的类型生成 LEAF
//   - 否则（哈希回退路径），由 typeMatchesCategory 按 Category ↔ Categories 匹配决定
//
// 每条 LEAF 的 Type 取洞察类型名，Points 由 generateInsightPoints 生成。
func (e *DefaultExtractor) extractInsights(memoryID memind.MemoryId, itemResult *ItemExtractResult, cfg memind.ExtractionConfig) (*InsightExtractResult, error) {
	if !cfg.EnableInsight || !e.opts.Insight.Enabled || len(itemResult.NewItems) == 0 {
		return &InsightExtractResult{}, nil
	}

	llmClient := e.llm.Resolve(llm.SlotInsightGenerator)
	now := time.Now()
	var insights []*memind.MemoryInsight
	byType := make(map[string][]*memind.MemoryInsight)

	// 建立类型名 → 类型定义的快速查找表
	typeMap := make(map[string]memind.MemoryInsightType)
	for _, t := range itemResult.Types {
		typeMap[t.Name] = t
	}

	for _, item := range itemResult.NewItems {
		// 判断是否有 LLM 分配的 insightTypes
		assignedTypes := e.resolveItemInsightTypes(item, typeMap)

		for _, t := range assignedTypes {
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

			if err := e.memStore.Insights().UpsertInsights(memoryID, []*memind.MemoryInsight{ins}); err != nil {
				return nil, fmt.Errorf("upsert insight: %w", err)
			}

			if e.textSearch != nil {
				docID := fmt.Sprintf("insight-%d", ins.ID)
				_ = e.textSearch.Index(memoryID, docID, ins.PointsContent(), tsearch.TargetInsight)
			}

			if e.vector != nil {
				vecID, _ := e.vector.Store(memoryID, ins.PointsContent(), map[string]any{"type": "insight", "insight_id": ins.ID})
				ins.SummaryEmbedding = nil
				_ = vecID
			}

			insights = append(insights, ins)
			byType[t.Name] = append(byType[t.Name], ins)
		}
	}

	return &InsightExtractResult{Insights: insights, ByType: byType}, nil
}

// resolveItemInsightTypes - 解析条目应生成的洞察类型列表
//
// 优先读取 item.Metadata["insightTypes"]（LLM 语义分类路径），
// 回退到按 Category 匹配 typeMap 中所有类型（哈希分桶路径）。
func (e *DefaultExtractor) resolveItemInsightTypes(item *memind.MemoryItem, typeMap map[string]memind.MemoryInsightType) []memind.MemoryInsightType {
	// LLM 路径：从 Metadata 读取 insightTypes
	if item.Metadata != nil {
		if raw, ok := item.Metadata["insightTypes"]; ok {
			if names, ok := raw.([]string); ok && len(names) > 0 {
				var result []memind.MemoryInsightType
				for _, name := range names {
					if t, found := typeMap[name]; found {
						result = append(result, t)
					}
				}
				return result
			}
			if names, ok := raw.([]any); ok && len(names) > 0 {
				var result []memind.MemoryInsightType
				for _, n := range names {
					name, _ := n.(string)
					if t, found := typeMap[name]; found {
						result = append(result, t)
					}
				}
				return result
			}
		}
	}

	// 哈希回退路径：按 Category 匹配所有类型
	var result []memind.MemoryInsightType
	for _, t := range typeMap {
		if typeMatchesCategory(t, item.Category) {
			result = append(result, t)
		}
	}
	return result
}

// generateInsightPoints - 调用 LLM 从条目内容中提取洞察点，无 LLM 时回退为摘要
func (e *DefaultExtractor) generateInsightPoints(client llm.StructuredChatClient, item *memind.MemoryItem, typeDef memind.MemoryInsightType, language string) []memind.InsightPoint {
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

// isValidCategory - 检查 category 是否在指定 scope 的合法分类集合中
func isValidCategory(cat memind.MemoryCategory, scope memind.MemoryScope) bool {
	var candidates []memind.MemoryCategory
	if scope == memind.ScopeAgent {
		candidates = memind.AgentCategories()
	} else {
		candidates = memind.UserCategories()
	}
	for _, c := range candidates {
		if c == cat {
			return true
		}
	}
	return false
}

const insightSystemPrompt = `You are an insight extraction system. Extract structured insights concisely.
Return ONLY a JSON array of objects, each with:
- "pointId": a unique string identifier
- "type": "SUMMARY" or "REASONING"
- "content": the insight text in the original language
- "sourceItemIds": array of source item ID strings`

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
