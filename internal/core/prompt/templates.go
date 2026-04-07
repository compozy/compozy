package prompt

import (
	"embed"
	"fmt"
)

//go:embed prompts/*.txt
var templateFS embed.FS

const (
	reasoningEffortLow    = "low"
	reasoningEffortMedium = "medium"
	reasoningEffortHigh   = "high"
	reasoningEffortXHigh  = "xhigh"
)

func ClaudeReasoningPrompt(reasoning string) string {
	switch reasoning {
	case reasoningEffortLow:
		return mustReadTemplate("claude-reasoning-low.txt")
	case reasoningEffortHigh:
		return mustReadTemplate("claude-reasoning-high.txt")
	case reasoningEffortXHigh:
		return mustReadTemplate("claude-reasoning-xhigh.txt")
	default:
		return mustReadTemplate("claude-reasoning-medium.txt")
	}
}

func mustReadTemplate(name string) string {
	content, err := templateFS.ReadFile("prompts/" + name)
	if err != nil {
		panic(fmt.Errorf("read embedded template %q: %w", name, err))
	}
	return string(content)
}
