package prompt

import (
	"strings"
)

type PromptResult struct {
	SystemPrompt string `json:"systemPrompt"`
	UserPrompt   string `json:"userPrompt"`
}

func NewPromptResult(system, user string) PromptResult {
	return PromptResult{SystemPrompt: system, UserPrompt: user}
}

type PromptTemplate struct {
	Name      string
	Sections  map[string]string
	Variables map[string]string
	order     []string
}

func NewPromptTemplate(name string) *PromptTemplate {
	return &PromptTemplate{
		Name:      name,
		Sections:  make(map[string]string),
		Variables: make(map[string]string),
	}
}

func (t *PromptTemplate) Section(name, content string) *PromptTemplate {
	t.Sections[name] = content
	t.order = append(t.order, name)
	return t
}

func (t *PromptTemplate) Variable(key, value string) *PromptTemplate {
	t.Variables[key] = value
	return t
}

func (t *PromptTemplate) Render(language string) PromptResult {
	var systemParts []string
	for _, section := range t.order {
		content := t.Sections[section]
		for k, v := range t.Variables {
			content = strings.ReplaceAll(content, "{{"+k+"}}", v)
		}
		systemParts = append(systemParts, content)
	}
	system := strings.Join(systemParts, "\n\n")
	langRule := "Always respond in " + language + "."
	if !strings.Contains(system, langRule) {
		system = langRule + "\n\n" + system
	}
	userPrompt := ""
	if up, ok := t.Variables["user_prompt"]; ok {
		userPrompt = up
	}
	return PromptResult{SystemPrompt: system, UserPrompt: userPrompt}
}
