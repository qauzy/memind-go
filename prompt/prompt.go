package prompt

import (
	"bytes"
	"text/template"
)

// PromptTemplate - 基于 text/template 的提示词模板
type PromptTemplate struct {
	tmpl *template.Template
}

// NewPromptTemplate - 从模板字符串创建提示词模板
func NewPromptTemplate(tmplStr string) (*PromptTemplate, error) {
	tmpl, err := template.New("prompt").Parse(tmplStr)
	if err != nil {
		return nil, err
	}
	return &PromptTemplate{tmpl: tmpl}, nil
}

// MustNewPromptTemplate - 带 panic 的创建函数（用于初始化）
func MustNewPromptTemplate(tmplStr string) *PromptTemplate {
	tmpl, err := NewPromptTemplate(tmplStr)
	if err != nil {
		panic(err)
	}
	return tmpl
}

// Execute - 执行模板并返回渲染结果
func (p *PromptTemplate) Execute(data any) (string, error) {
	var buf bytes.Buffer
	if err := p.tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
