package insight

// branchOpsSystemPrompt - Branch 增量操作系统提示
const branchOpsSystemPrompt = `You are a memory insight aggregation system operating at the BRANCH level.
Produce ADD/UPDATE/DELETE operations to adjust BRANCH summary points to reflect new LEAF insights.
- For UPDATE, pointId MUST match an existing point.
- For DELETE, supply only pointId.
- For ADD, pointId can be any unique string.
- Each point covers one coherent aspect.`

// branchOpsUserPrompt - Branch 增量操作用户提示模板
// 参数: typeName, description, targetTokens, existingPoints, leafTexts, language, additionalContext
const branchOpsUserPrompt = `Adjust the BRANCH summary for insight type "%s" (%s).
Token budget: ~%d tokens.

Existing BRANCH points:
%s

LEAF insights to reflect:
%s

Language: %s
Additional context: %s

Return JSON: {"operations":[{"op":"ADD|UPDATE|DELETE","pointId":"...","type":"SUMMARY|REASONING","content":"...","sourceItemIds":[...]}]}`

// branchFullSystemPrompt - Branch 全量重写系统提示
const branchFullSystemPrompt = `You are a memory insight aggregation system operating at the BRANCH level.
Produce complete SUMMARY and REASONING insight points aggregating LEAF insights.
- SUMMARY: factual aggregation.
- REASONING: interpretation/pattern/inference.
- Each point covers a distinct aspect.`

// branchFullUserPrompt - Branch 全量重写用户提示模板
// 参数: typeName, description, targetTokens, existingPoints, leafTexts, language, additionalContext
const branchFullUserPrompt = `Aggregate LEAF insights into BRANCH summary for type "%s" (%s).
Token budget: ~%d tokens.

Existing points (rewrite fully):
%s

LEAF insights:
%s

Language: %s
Additional context: %s

Return JSON: {"points":[{"pointId":"...","type":"SUMMARY|REASONING","content":"...","sourceItemIds":[...]}]}`

// rootSynthesisSystemPrompt - Root 深度合成系统提示
const rootSynthesisSystemPrompt = `You are a deep memory synthesis system operating at the ROOT level.
Analyze cross-dimension patterns across BRANCH dimensions.
Consider: CONVERGENCE, TENSION, TRAJECTORY, and CAUSATION.`

// rootSynthesisUserPrompt - Root 深度合成用户提示模板
// 参数: typeName, description, targetTokens, existingPoints, branchTexts, language, additionalContext
const rootSynthesisUserPrompt = `Synthesize BRANCH dimensions into ROOT understanding for type "%s" (%s).
Token budget: ~%d tokens.

Existing ROOT points (rewrite fully):
%s

BRANCH dimensions:
%s

Language: %s
Additional context: %s

Return JSON: {"points":[{"pointId":"...","type":"SUMMARY|REASONING","content":"...","sourceItemIds":[...]}]}`
