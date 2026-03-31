package prompt

import (
	"strings"
	"testing"

	"github.com/compozy/looper/internal/looper/model"
)

func TestBuildSystemPromptIncludesModeSpecificInstructionsAndJobSignal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		mode     model.ExecutionMode
		snippets []string
	}{
		{
			name: "prd tasks",
			mode: model.ExecutionModePRDTasks,
			snippets: []string{
				"interactive terminal workflow",
				"`execute-prd-task`",
				"`verification-before-completion`",
				"_techspec.md",
				"_tasks.md",
				"Keep scope tight to the current task",
				"http://localhost:4321/job/done",
				`{"id":"batch-001"}`,
			},
		},
		{
			name: "pr review",
			mode: model.ExecutionModePRReview,
			snippets: []string{
				"interactive terminal workflow",
				"`fix-reviews`",
				"`verification-before-completion`",
				"Triage each issue",
				"Looper resolves provider threads after the batch succeeds.",
				"http://localhost:4321/job/done",
				`{"id":"batch-001"}`,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			promptText := BuildSystemPrompt(tc.mode, "batch-001", 4321)
			for _, snippet := range tc.snippets {
				if !strings.Contains(promptText, snippet) {
					t.Fatalf("expected prompt to include %q, got %q", snippet, promptText)
				}
			}
		})
	}
}

func TestBuildSystemPromptFallsBackToGenericInstructionsForUnknownMode(t *testing.T) {
	t.Parallel()

	promptText := BuildSystemPrompt(model.ExecutionMode("unknown"), "batch-999", 9877)

	requiredSnippets := []string{
		"Follow the referenced prompt file as the source of truth for this job.",
		"Use `verification-before-completion` before any completion claim.",
		"http://localhost:9877/job/done",
		`{"id":"batch-999"}`,
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(promptText, snippet) {
			t.Fatalf("expected generic prompt to include %q, got %q", snippet, promptText)
		}
	}
}
