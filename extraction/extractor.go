package extraction

import (
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
	if !e.opts.Item.Enabled {
		return &ItemExtractResult{}, nil
	}

	var itemTypes []memind.MemoryInsightType
	var newItems []*memind.MemoryItem
	now := time.Now()

	for _, rd := range rawResult.RawDataList {
		hash := simpleHash(rd.Caption)
		existing, _ := e.memStore.Items().GetItemByHash(memoryID, hash)
		if existing != nil {
			continue
		}

		scope := cfg.Scope
		categories := memind.UserCategories()
		if scope == memind.ScopeAgent {
			categories = memind.AgentCategories()
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

		insightTypes, _ := e.memStore.Insights().ListInsightTypes()
		for _, it := range insightTypes {
			if it.Scope == scope {
				itemTypes = append(itemTypes, *it)
			}
		}
	}

	if len(newItems) > 0 {
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
				for _, it := range itemTypes {
					e.buf.InsightBuffer().Add(memoryID, item.ID, it.Name)
				}
			}
		}
	}

	uniqTypes := dedupTypes(itemTypes)
	return &ItemExtractResult{
		NewItems: newItems,
		Types:    uniqTypes,
	}, nil
}

// extractInsights - 第三阶段：基于提取的条目生成洞察（当前为占位实现）
func (e *DefaultExtractor) extractInsights(memoryID memind.MemoryId, itemResult *ItemExtractResult, cfg memind.ExtractionConfig) (*InsightExtractResult, error) {
	if !cfg.EnableInsight || !e.opts.Insight.Enabled || len(itemResult.NewItems) == 0 {
		return &InsightExtractResult{}, nil
	}
	return &InsightExtractResult{}, nil
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
