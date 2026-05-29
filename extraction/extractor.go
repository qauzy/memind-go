package extraction

import (
	"fmt"
	"log"
	"strings"
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
	log.Printf("[extract] begin memoryID=%s scope=%s", req.MemoryID.Identifier(), cfg.Scope)

	rawDataResult, err := e.extractRawData(req.MemoryID, req.Content, req.Metadata, cfg)
	if err != nil {
		return nil, &memind.ExtractionError{Stage: "rawdata", Message: "raw data extraction failed", Err: err}
	}
	log.Printf("[extract] rawData done: %d items", len(rawDataResult.RawDataList))

	itemResult, err := e.extractItems(req.MemoryID, rawDataResult, cfg, cfg.Language)
	if err != nil {
		return nil, &memind.ExtractionError{Stage: "item", Message: "item extraction failed", Err: err}
	}
	log.Printf("[extract] items done: %d items", len(itemResult.NewItems))

	insightResult, err := e.extractInsights(req.MemoryID, itemResult, cfg)
	if err != nil {
		return nil, &memind.ExtractionError{Stage: "insight", Message: "insight extraction failed", Err: err}
	}
	log.Printf("[extract] insights done: %d items", len(insightResult.Insights))

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
		vecID, err := e.vector.Store(memoryID, content.Content, map[string]any{"type": "rawdata"})
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
		log.Printf("[extractItems] item extraction disabled")
		return &ItemExtractResult{}, nil
	}

	llmClient := e.llm.Resolve(llm.SlotItemExtraction)
	if _, ok := llmClient.(*llm.NoOpChatClient); !ok {
		log.Printf("[extractItems] LLM client found (type=%T), trying LLM path", llmClient)
		result, err := e.extractItemsWithLLM(memoryID, rawResult, cfg, llmClient)
		if err != nil {
			log.Printf("[extractItems] LLM path error: %v", err)
			return nil, err
		}
		log.Printf("[extractItems] LLM path returned %d items, falling back to hash if zero", len(result.NewItems))
		if len(result.NewItems) > 0 {
			return result, nil
		}
	} else {
		log.Printf("[extractItems] No LLM client, using hash path directly")
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
	sysPrompt := itemExtractionSystemPrompt + "\n\nIMPORTANT: Output items in the original language as the input conversation. Do NOT translate."
	err := client.CallStructured([]llm.ChatMessage{
		{Role: llm.RoleSystem, Content: sysPrompt},
		{Role: llm.RoleUser, Content: fmt.Sprintf(itemExtractionUserPrompt, conversation)},
	}, &resp)
	if err != nil {
		log.Printf("[extractItemsWithLLM] LLM CallStructured error: %v", err)
		return nil, fmt.Errorf("llm item extraction: %w", err)
	}

	log.Printf("[extractItemsWithLLM] LLM returned %d items", len(resp.Items))
	if len(resp.Items) == 0 {
		return &ItemExtractResult{}, nil
	}

	now := time.Now()
	var newItems []*memind.MemoryItem

	for i, extracted := range resp.Items {
		log.Printf("[extractItemsWithLLM] item[%d]: content=%q category=%q insightTypes=%v", i, extracted.Content, extracted.Category, extracted.InsightTypes)
		if extracted.Content == "" {
			log.Printf("[extractItemsWithLLM] item[%d] skipped: empty content", i)
			continue
		}
		if extracted.Category == "" {
			log.Printf("[extractItemsWithLLM] item[%d] skipped: empty category", i)
			continue
		}

		hash := simpleHash(extracted.Content)
		existing, _ := e.memStore.Items().GetItemByHash(memoryID, hash)
		if existing != nil {
			log.Printf("[extractItemsWithLLM] item[%d] skipped: duplicate hash %s", i, hash)
			continue
		}

		// 校验 LLM 返回的分类是否合法，不合法则跳过
		cat := memind.MemoryCategory(extracted.Category)
		if !isValidCategory(cat, scope) {
			log.Printf("[extractItemsWithLLM] item[%d] skipped: invalid category %q for scope %s", i, extracted.Category, scope)
			continue
		}
		log.Printf("[extractItemsWithLLM] item[%d] category %q valid", i, extracted.Category)

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
			log.Printf("[extractItemsWithLLM] item[%d] skipped: no valid insightTypes (got %v, scopeTypes=%v)", i, extracted.InsightTypes, scopeTypeNames(scopeTypes))
			continue
		}
		log.Printf("[extractItemsWithLLM] item[%d] validTypes=%v", i, validTypes)

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

	log.Printf("[extractItemsWithLLM] %d items accepted, %d skipped", len(newItems), len(resp.Items)-len(newItems))
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
	log.Printf("[extractItemsHash] rawData=%d scopeTypes=%v", len(rawResult.RawDataList), scopeTypeNames(scopeTypes))

	now := time.Now()
	var newItems []*memind.MemoryItem

	for i, rd := range rawResult.RawDataList {
		hash := simpleHash(rd.Caption)
		existing, _ := e.memStore.Items().GetItemByHash(memoryID, hash)
		if existing != nil {
			log.Printf("[extractItemsHash] rawData[%d] skipped: duplicate hash %s", i, hash)
			continue
		}
		log.Printf("[extractItemsHash] rawData[%d] hash=%s caption=%q", i, hash, rd.Caption)

		category := categories[0]
		if len(categories) > 1 {
			idx := 0
			for _, c := range hash {
				idx += int(c)
				break
			}
			category = categories[idx%len(categories)]
		}
		log.Printf("[extractItemsHash] rawData[%d] category=%s", i, category)

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

	log.Printf("[extractItemsHash] %d new items after dedup", len(newItems))
	if len(newItems) == 0 {
		return &ItemExtractResult{}, nil
	}

	if err := e.memStore.Items().UpsertItems(memoryID, newItems); err != nil {
		log.Printf("[extractItemsHash] UpsertItems error: %v", err)
		return nil, err
	}
	log.Printf("[extractItemsHash] UpsertItems ok")

	for _, item := range newItems {
		docID := fmt.Sprintf("item-%d", item.ID)
		log.Printf("[extractItemsHash] indexing item id=%d content=%q", item.ID, item.Content)
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

// extractInsights - 第三阶段：基于提取的条目生成洞察（Java InsightLayer 移植版）
//
// 新流程（匹配 Java InsightLayer / LlmInsightGroupClassifier）：
//  1. 按洞察类型收集条目
//  2. 每个类型内：LLM 语义分组（InsightGroupPrompts）
//  3. 每个分组内：LLM 多条目合成 LEAF 洞察（InsightLeafPrompts）
//  4. 无 LLM 时回退为每条目单点摘要
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

	// 按类型收集条目（LLM 路径按 insightTypes，哈希路径按 Category）
	typeItems := make(map[string][]*memind.MemoryItem)
	for _, item := range itemResult.NewItems {
		assignedTypes := e.resolveItemInsightTypes(item, typeMap)
		for _, t := range assignedTypes {
			typeItems[t.Name] = append(typeItems[t.Name], item)
		}
	}

	hasLLM := false
	if _, ok := llmClient.(*llm.NoOpChatClient); !ok {
		hasLLM = true
	}

	for typeName, items := range typeItems {
		t := typeMap[typeName]

		var generatedInsights []*memind.MemoryInsight

		if hasLLM {
			// LLM 路径：分组 → 每组合成 LEAF（更新已有或新建）
			groups := e.groupItemsForType(llmClient, items, t, cfg.Language)
			for groupName, groupItems := range groups {
				if groupName == "UNRELATED" {
					continue
				}
				// 查找是否已有该分组的 LEAF
				existingLeaf, _ := e.memStore.Insights().GetLeafByGroup(memoryID, t.Name, groupName)
				points := e.generateLeafInsights(llmClient, groupItems, t, groupName, cfg.Language)
				if len(points) == 0 {
					continue
				}
				if existingLeaf != nil {
					existingLeaf.Points = points
					existingLeaf.UpdatedAt = time.Now()
					existingLeaf.Version++
					generatedInsights = append(generatedInsights, existingLeaf)
				} else {
					ins := &memind.MemoryInsight{
						MemoryID:  memoryID.Identifier(),
						Type:      t.Name,
						Scope:     t.Scope,
						Name:      groupName,
						Points:    points,
						CreatedAt: now,
						UpdatedAt: now,
						Tier:      memind.TierLeaf,
						Version:   1,
					}
					generatedInsights = append(generatedInsights, ins)
				}
			}
		} else {
			// 回退路径：每条目生成单点摘要（更新已有或新建）
			var leafByGroup *memind.MemoryInsight
			for _, item := range items {
				if leafByGroup == nil {
					leafByGroup, _ = e.memStore.Insights().GetLeafByGroup(memoryID, t.Name, t.Name)
				}
				point := memind.InsightPoint{
					PointID:       fmt.Sprintf("sp-%d", time.Now().UnixNano()),
					Type:          memind.PointTypeSummary,
					Content:       item.Content,
					SourceItemIDs: []string{fmt.Sprintf("%d", item.ID)},
				}
				if leafByGroup != nil {
					leafByGroup.Points = append(leafByGroup.Points, point)
					leafByGroup.UpdatedAt = time.Now()
					leafByGroup.Version++
				}
			}
			if leafByGroup != nil {
				generatedInsights = append(generatedInsights, leafByGroup)
			} else {
				for _, item := range items {
					ins := &memind.MemoryInsight{
						MemoryID: memoryID.Identifier(),
						Type:     t.Name,
						Scope:    t.Scope,
						Name:     t.Name,
						Points: []memind.InsightPoint{{
							PointID:       fmt.Sprintf("sp-%d", time.Now().UnixNano()),
							Type:          memind.PointTypeSummary,
							Content:       item.Content,
							SourceItemIDs: []string{fmt.Sprintf("%d", item.ID)},
						}},
						CreatedAt: now,
						UpdatedAt: now,
						Tier:      memind.TierLeaf,
						Version:   1,
					}
					generatedInsights = append(generatedInsights, ins)
				}
			}
		}

		// 统一持久化、索引、向量化
		for _, ins := range generatedInsights {
			if err := e.memStore.Insights().UpsertInsights(memoryID, []*memind.MemoryInsight{ins}); err != nil {
				return nil, fmt.Errorf("upsert insight: %w", err)
			}
			if e.textSearch != nil {
				docID := fmt.Sprintf("insight-%d", ins.ID)
				_ = e.textSearch.Index(memoryID, docID, ins.PointsContent(), tsearch.TargetInsight)
			}
			if e.vector != nil {
				_, _ = e.vector.Store(memoryID, ins.PointsContent(), map[string]any{"type": "insight", "insight_id": ins.ID})
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

// groupItemsForType - 调用 InsightGroupPrompts LLM 对同类条目进行语义分组
//
// 返回 map[groupName][]items。hash 路径无 LLM 时按 item ID 分桶。
func (e *DefaultExtractor) groupItemsForType(client llm.StructuredChatClient, items []*memind.MemoryItem, typeDef memind.MemoryInsightType, language string) map[string][]*memind.MemoryItem {
	hasLLM := false
	if _, ok := client.(*llm.NoOpChatClient); !ok {
		hasLLM = true
	}

	if hasLLM {
		// 构造 LLM 上下文：列出所有条目标题和内容
		var itemList string
		for i, item := range items {
			itemList += fmt.Sprintf("[item %d] %s\n", item.ID, item.Content)
			if i < len(items)-1 {
				itemList += "\n"
			}
		}

		sysPrompt := strings.ReplaceAll(InsightGroupSystemPrompt, "{{insight_type_name}}", typeDef.Name)
		sysPrompt = strings.ReplaceAll(sysPrompt, "{{insight_type_description}}", typeDef.Description)
		if language != "" {
			sysPrompt += fmt.Sprintf("\n\nIMPORTANT: Output in language: %s.", language)
		}

		var resp insightGroupResponse
		err := client.CallStructured([]llm.ChatMessage{
			{Role: llm.RoleSystem, Content: sysPrompt},
			{Role: llm.RoleUser, Content: fmt.Sprintf("Assign these items into semantic groups:\n\n%s", itemList)},
		}, &resp)
		if err == nil && len(resp.Assignments) > 0 {
			groups := make(map[string][]*memind.MemoryItem)
			itemByID := make(map[int64]*memind.MemoryItem)
			for _, item := range items {
				itemByID[item.ID] = item
			}
			for _, a := range resp.Assignments {
				if item, ok := itemByID[a.ItemID]; ok {
					groups[a.GroupName] = append(groups[a.GroupName], item)
				}
			}
			return groups
		}
	}

	// 回退：每条目独立成组
	groups := make(map[string][]*memind.MemoryItem)
	for _, item := range items {
		groupName := fmt.Sprintf("item-%d", item.ID)
		groups[groupName] = append(groups[groupName], item)
	}
	return groups
}

// generateLeafInsights - 调用 InsightLeafPrompts LLM 对一个语义分组的条目合成 LEAF 洞察点
//
// 输入同一分组的多个条目，LLM 多条目合成（SUMMARY/REASONING），
// 无 LLM 时回退为每条目单点摘要。
func (e *DefaultExtractor) generateLeafInsights(client llm.StructuredChatClient, items []*memind.MemoryItem, typeDef memind.MemoryInsightType, groupName, language string) []memind.InsightPoint {
	hasLLM := false
	if _, ok := client.(*llm.NoOpChatClient); !ok {
		hasLLM = true
	}

	if hasLLM {
		// 构造 LLM 上下文：列出分组内所有条目
		var itemList string
		for i, item := range items {
			itemList += fmt.Sprintf("[item %d] %s\n", item.ID, item.Content)
			if i < len(items)-1 {
				itemList += "\n"
			}
		}

		sysPrompt := strings.ReplaceAll(InsightLeafSystemPrompt, "{{insight_type}}", typeDef.Name)
		sysPrompt = strings.ReplaceAll(sysPrompt, "{{insight_description}}", typeDef.Description)
		sysPrompt = strings.ReplaceAll(sysPrompt, "{{group_name}}", groupName)
		if language != "" {
			sysPrompt += fmt.Sprintf("\n\nIMPORTANT: Output in language: %s.", language)
		}

		var resp insightLeafResponse
		err := client.CallStructured([]llm.ChatMessage{
			{Role: llm.RoleSystem, Content: sysPrompt},
			{Role: llm.RoleUser, Content: fmt.Sprintf("Synthesize insight points from these memory items:\n\n%s", itemList)},
		}, &resp)
		if err == nil && len(resp.Points) > 0 {
			var points []memind.InsightPoint
			for _, p := range resp.Points {
				sourceIDs := p.SourceItemIDs
				if len(sourceIDs) == 0 {
					for _, item := range items {
						sourceIDs = append(sourceIDs, fmt.Sprintf("%d", item.ID))
					}
				}
				points = append(points, memind.InsightPoint{
					PointID:       fmt.Sprintf("sp-%d-%d", time.Now().UnixNano(), len(points)),
					Type:          memind.PointType(p.Type),
					Content:       p.Content,
					SourceItemIDs: sourceIDs,
				})
			}
			return points
		}
	}

	// 回退：每条目生成单点摘要
	var points []memind.InsightPoint
	for _, item := range items {
		points = append(points, memind.InsightPoint{
			PointID:       fmt.Sprintf("sp-%d", time.Now().UnixNano()),
			Type:          memind.PointTypeSummary,
			Content:       item.Content,
			SourceItemIDs: []string{fmt.Sprintf("%d", item.ID)},
		})
	}
	return points
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
func scopeTypeNames(types []memind.MemoryInsightType) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.Name
	}
	return names
}

func isValidCategory(cat memind.MemoryCategory, scope memind.MemoryScope) bool {
	var candidates []memind.MemoryCategory
	if scope == memind.ScopeAgent {
		candidates = memind.AgentCategories()
	} else {
		candidates = memind.UserCategories()
	}
	for _, c := range candidates {
		if cat == c {
			return true
		}
	}
	return false
}

// insightGroupAssignment - 洞察分组 LLM 响应的单条目分配
type insightGroupAssignment struct {
	ItemID    int64  `json:"itemId"`
	GroupName string `json:"groupName"`
}

// insightGroupResponse - 洞察分组 LLM 响应
type insightGroupResponse struct {
	Assignments []insightGroupAssignment `json:"assignments"`
}

// insightLeafPoint - 洞察叶子节点 LLM 响应的单个洞察点
type insightLeafPoint struct {
	Type          string   `json:"type"`
	Content       string   `json:"content"`
	SourceItemIDs []string `json:"sourceItemIds"`
	PointReason   string   `json:"point_reason,omitempty"`
}

// insightLeafResponse - 洞察叶子节点 LLM 响应
type insightLeafResponse struct {
	Points []insightLeafPoint `json:"points"`
}

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
