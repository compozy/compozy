package run

import (
	"strings"
	"testing"

	"github.com/compozy/looper/internal/looper/model"
)

func TestBuildIDECommandConfigBuildsClaudeSystemPromptPerJob(t *testing.T) {
	t.Parallel()

	commandCfg := buildIDECommandConfig(&config{
		ide:             model.IDEClaude,
		mode:            model.ExecutionModePRDTasks,
		signalPort:      4321,
		reasoningEffort: "high",
	}, &job{safeName: "batch-001"})

	requiredSnippets := []string{
		"Ultrathink deeply and comprehensively before taking action.",
		"`execute-prd-task`",
		"http://localhost:4321/job/done",
		`{"id":"batch-001"}`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(commandCfg.SystemPrompt, snippet) {
			t.Fatalf("expected system prompt to include %q, got %q", snippet, commandCfg.SystemPrompt)
		}
	}
}

func TestBuildIDECommandConfigLeavesNonClaudeSystemPromptEmpty(t *testing.T) {
	t.Parallel()

	commandCfg := buildIDECommandConfig(&config{
		ide:             model.IDECodex,
		reasoningEffort: "medium",
	}, &job{safeName: "batch-001"})

	if commandCfg.SystemPrompt != "" {
		t.Fatalf("expected non-claude system prompt to be empty, got %q", commandCfg.SystemPrompt)
	}
}
