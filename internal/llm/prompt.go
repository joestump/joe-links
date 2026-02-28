// Governing: SPEC-0017 REQ "Default Prompt Template"
package llm

import (
	"bytes"
	_ "embed"
	"text/template"
)

//go:embed prompt.tmpl
var defaultPromptTemplate string

// PromptData holds the variables available in the prompt template.
type PromptData struct {
	URL         string
	Title       string
	Description string
}

// renderPrompt executes the prompt template with the given data.
// If customTemplate is non-empty it is used instead of the embedded default.
func renderPrompt(customTemplate string, data PromptData) (string, error) {
	src := defaultPromptTemplate
	if customTemplate != "" {
		src = customTemplate
	}

	tmpl, err := template.New("prompt").Parse(src)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
