package insight

import (
	"encoding/json"
	"fmt"

	memind "github.com/openmemind/memind-go"
	"github.com/openmemind/memind-go/llm"
)

// InsightGenerator - 洞察点生成器接口
// Modified: 2026-05-28 - 原版 Java InsightGenerator 的 Go 移植
type InsightGenerator interface {
	// GenerateBranchPointOps - 生成 Branch 增量操作（ADD/UPDATE/DELETE）
	GenerateBranchPointOps(insightType memind.MemoryInsightType, existingPoints []memind.InsightPoint, leafInsights []*memind.MemoryInsight, targetTokens int, additionalContext string, language string) (*memind.InsightPointOpsResponse, error)

	// GenerateBranchSummary - 全量重写 Branch points
	GenerateBranchSummary(insightType memind.MemoryInsightType, existingPoints []memind.InsightPoint, leafInsights []*memind.MemoryInsight, targetTokens int, additionalContext string, language string) (*memind.InsightPointGenerateResponse, error)

	// GenerateRootSynthesis - 深度合成 Root points
	GenerateRootSynthesis(rootType memind.MemoryInsightType, existingPoints []memind.InsightPoint, branchInsights []*memind.MemoryInsight, targetTokens int, additionalContext string, language string) (*memind.InsightPointGenerateResponse, error)
}

// LlmInsightGenerator - LLM 驱动的洞察生成器
// Modified: 2026-05-28 - 原版 Java LlmInsightGenerator 的 Go 移植
type LlmInsightGenerator struct {
	llm *llm.ChatClientRegistry
}

// NewLlmInsightGenerator - 创建 LLM 洞察生成器
func NewLlmInsightGenerator(llm *llm.ChatClientRegistry) *LlmInsightGenerator {
	return &LlmInsightGenerator{llm: llm}
}

// GenerateBranchPointOps - 调用 LLM 生成 Branch 增量操作
func (g *LlmInsightGenerator) GenerateBranchPointOps(insightType memind.MemoryInsightType, existingPoints []memind.InsightPoint, leafInsights []*memind.MemoryInsight, targetTokens int, additionalContext string, language string) (*memind.InsightPointOpsResponse, error) {
	client := g.llm.Resolve(llm.SlotInsightGenerator)
	if _, ok := client.(*llm.NoOpChatClient); ok {
		return nil, nil
	}

	var leafTexts string
	for i, leaf := range leafInsights {
		leafTexts += fmt.Sprintf("--- Leaf %d (%s) ---\n%s\n", i+1, leaf.Group, leaf.PointsContent())
	}

	userPrompt := fmt.Sprintf(branchOpsUserPrompt,
		insightType.Name, insightType.Description, targetTokens,
		formatExistingPoints(existingPoints),
		leafTexts,
		language, additionalContext)

	var resp memind.InsightPointOpsResponse
	err := client.CallStructured([]llm.ChatMessage{
		{Role: llm.RoleSystem, Content: branchOpsSystemPrompt},
		{Role: llm.RoleUser, Content: userPrompt},
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GenerateBranchSummary - 全量重写 Branch points
func (g *LlmInsightGenerator) GenerateBranchSummary(insightType memind.MemoryInsightType, existingPoints []memind.InsightPoint, leafInsights []*memind.MemoryInsight, targetTokens int, additionalContext string, language string) (*memind.InsightPointGenerateResponse, error) {
	client := g.llm.Resolve(llm.SlotInsightGenerator)
	if _, ok := client.(*llm.NoOpChatClient); ok {
		return nil, nil
	}

	var leafTexts string
	for i, leaf := range leafInsights {
		leafTexts += fmt.Sprintf("--- Leaf %d (%s) ---\n%s\n", i+1, leaf.Group, leaf.PointsContent())
	}

	userPrompt := fmt.Sprintf(branchFullUserPrompt,
		insightType.Name, insightType.Description, targetTokens,
		formatExistingPoints(existingPoints),
		leafTexts,
		language, additionalContext)

	var resp memind.InsightPointGenerateResponse
	err := client.CallStructured([]llm.ChatMessage{
		{Role: llm.RoleSystem, Content: branchFullSystemPrompt},
		{Role: llm.RoleUser, Content: userPrompt},
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GenerateRootSynthesis - 深度合成 Root points
func (g *LlmInsightGenerator) GenerateRootSynthesis(rootType memind.MemoryInsightType, existingPoints []memind.InsightPoint, branchInsights []*memind.MemoryInsight, targetTokens int, additionalContext string, language string) (*memind.InsightPointGenerateResponse, error) {
	client := g.llm.Resolve(llm.SlotInsightGenerator)
	if _, ok := client.(*llm.NoOpChatClient); ok {
		return nil, nil
	}

	var branchTexts string
	for i, branch := range branchInsights {
		branchTexts += fmt.Sprintf("--- Branch %d (%s) ---\n%s\n", i+1, branch.Type, branch.PointsContent())
	}

	userPrompt := fmt.Sprintf(rootSynthesisUserPrompt,
		rootType.Name, rootType.Description, targetTokens,
		formatExistingPoints(existingPoints),
		branchTexts,
		language, additionalContext)

	var resp memind.InsightPointGenerateResponse
	err := client.CallStructured([]llm.ChatMessage{
		{Role: llm.RoleSystem, Content: rootSynthesisSystemPrompt},
		{Role: llm.RoleUser, Content: userPrompt},
	}, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// NoOpInsightGenerator - 空操作生成器（无 LLM 时使用）
type NoOpInsightGenerator struct{}

func (g *NoOpInsightGenerator) GenerateBranchPointOps(insightType memind.MemoryInsightType, existingPoints []memind.InsightPoint, leafInsights []*memind.MemoryInsight, targetTokens int, additionalContext string, language string) (*memind.InsightPointOpsResponse, error) {
	return nil, nil
}

func (g *NoOpInsightGenerator) GenerateBranchSummary(insightType memind.MemoryInsightType, existingPoints []memind.InsightPoint, leafInsights []*memind.MemoryInsight, targetTokens int, additionalContext string, language string) (*memind.InsightPointGenerateResponse, error) {
	return nil, nil
}

func (g *NoOpInsightGenerator) GenerateRootSynthesis(rootType memind.MemoryInsightType, existingPoints []memind.InsightPoint, branchInsights []*memind.MemoryInsight, targetTokens int, additionalContext string, language string) (*memind.InsightPointGenerateResponse, error) {
	return nil, nil
}

// formatExistingPoints - 将 points 格式化为 LLM 输入
func formatExistingPoints(points []memind.InsightPoint) string {
	if len(points) == 0 {
		return "(none)"
	}
	b, _ := json.MarshalIndent(points, "", "  ")
	return string(b)
}
