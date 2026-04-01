package prompt

import (
	"embed"
)

//go:embed prompts/*.txt
var templateFS embed.FS

func ClaudeReasoningPrompt(reasoning string) string {
	switch reasoning {
	case "low":
		return mustReadTemplate("claude-reasoning-low.txt")
	case "high":
		return mustReadTemplate("claude-reasoning-high.txt")
	case "xhigh":
		return mustReadTemplate("claude-reasoning-xhigh.txt")
	default:
		return mustReadTemplate("claude-reasoning-medium.txt")
	}
}

func mustReadTemplate(name string) string {
	content, err := templateFS.ReadFile("prompts/" + name)
	if err != nil {
		return ""
	}
	return string(content)
}
