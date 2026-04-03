package setup

import (
	"fmt"
	"strings"
)

var runtimeIDEAgentNames = map[string]string{
	"claude":       "claude-code",
	"codex":        "codex",
	"cursor-agent": "cursor",
	"droid":        "droid",
	"gemini":       "gemini-cli",
	"opencode":     "opencode",
	"pi":           "pi",
}

// AgentNameForIDE maps a compozy runtime IDE name to the setup agent name.
func AgentNameForIDE(ide string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(ide))
	if normalized == "" {
		return "", fmt.Errorf("map runtime IDE to setup agent: IDE is required")
	}

	agentName, ok := runtimeIDEAgentNames[normalized]
	if !ok {
		return "", fmt.Errorf("map runtime IDE %q to setup agent: unsupported IDE", ide)
	}
	return agentName, nil
}
