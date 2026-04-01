package prompt

import (
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestClaudeReasoningPromptUsesEmbeddedTemplates(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"low":    "Think concisely and act quickly. Prefer direct solutions.",
		"medium": "Think hard through problems carefully before acting. Balance speed with thoroughness.",
		"high":   "Ultrathink deeply and comprehensively before taking action.",
		"xhigh":  "Ultra-deep thinking mode: Exhaustively analyze every aspect of the problem.",
	}

	for reasoning, snippet := range cases {
		t.Run(reasoning, func(t *testing.T) {
			t.Parallel()

			promptText := ClaudeReasoningPrompt(reasoning)
			if !strings.Contains(promptText, snippet) {
				t.Fatalf("expected prompt for %q to include %q, got %q", reasoning, snippet, promptText)
			}
		})
	}
}

func TestBuildCodeReviewPromptUsesInstalledSkillsAndAvoidsLegacyDependencies(t *testing.T) {
	t.Parallel()

	promptText := buildCodeReviewPrompt(BatchParams{
		Name:       "my-feature",
		Round:      1,
		Provider:   "coderabbit",
		PR:         "259",
		ReviewsDir: "/tmp/tasks/my-feature/reviews-001",
		Grouped:    true,
		AutoCommit: true,
		Mode:       model.ExecutionModePRReview,
		BatchGroups: map[string][]model.IssueEntry{
			"internal/app/service.go": {
				{
					Name:     "issue_003.md",
					AbsPath:  "/tmp/tasks/my-feature/reviews-001/issue_003.md",
					CodeFile: "internal/app/service.go",
				},
				{
					Name:     "issue_004.md",
					AbsPath:  "/tmp/tasks/my-feature/reviews-001/issue_004.md",
					CodeFile: "internal/app/service.go",
				},
			},
		},
	})

	requiredSnippets := []string{
		"`fix-reviews`",
		"`verification-before-completion`",
		"<batch_issue_files>",
		"Review round: `001`",
		"Issue range: `issue_003.md` → `issue_004.md`",
		"Compozy resolves provider threads after the batch succeeds.",
		"Grouped summaries: enabled",
		"Create exactly one local commit for this batch after clean verification.",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(promptText, snippet) {
			t.Fatalf("expected review prompt to include %q", snippet)
		}
	}

	forbiddenSnippets := []string{
		".claude/skills",
		"scripts/read_pr_issues.sh",
		"resolve_pr_issues.sh",
		"pnpm run",
		"fix-coderabbit-review",
	}
	for _, snippet := range forbiddenSnippets {
		if strings.Contains(promptText, snippet) {
			t.Fatalf("expected review prompt to omit %q", snippet)
		}
	}
}

func TestBuildCodeReviewPromptRespectsDisabledGroupedAndAutoCommitModes(t *testing.T) {
	t.Parallel()

	promptText := buildCodeReviewPrompt(BatchParams{
		Name:       "my-feature",
		Round:      2,
		Provider:   "coderabbit",
		PR:         "260",
		ReviewsDir: "/tmp/tasks/my-feature/reviews-002",
		Grouped:    false,
		AutoCommit: false,
		Mode:       model.ExecutionModePRReview,
		BatchGroups: map[string][]model.IssueEntry{
			"internal/app/service.go": {
				{
					Name:     "issue_007.md",
					AbsPath:  "/tmp/tasks/my-feature/reviews-002/issue_007.md",
					CodeFile: "internal/app/service.go",
				},
			},
		},
	})

	requiredSnippets := []string{
		"Grouped summaries: disabled",
		"Automatic commits: disabled (`--auto-commit=false`)",
		"Grouped tracker updates are disabled for this run.",
		"Do not create an automatic commit.",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(promptText, snippet) {
			t.Fatalf("expected review prompt to include %q", snippet)
		}
	}
}

func TestBuildPRDTaskPromptUsesInstalledSkillsAndLeavesOnlyTaskSpecificContext(t *testing.T) {
	t.Parallel()

	task := model.IssueEntry{
		Name:    "task_1.md",
		AbsPath: "/tmp/tasks/demo/task_1.md",
		Content: `## status: pending
<task_context>
  <domain>backend</domain>
  <type>feature</type>
  <scope>small</scope>
  <complexity>low</complexity>
</task_context>
`,
	}

	promptText := buildPRDTaskPrompt(task, false)

	requiredSnippets := []string{
		"`execute-prd-task`",
		"`verification-before-completion`",
		"## Task Files",
		"Task file: `/tmp/tasks/demo/task_1.md`",
		"Master tasks file: `/tmp/tasks/demo/_tasks.md`",
		"Automatic commits are disabled for this run (`--auto-commit=false`).",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(promptText, snippet) {
			t.Fatalf("expected PRD prompt to include %q", snippet)
		}
	}

	forbiddenSnippets := []string{
		".claude/skills",
		"pnpm run",
		"typecheck",
		"ONE-SHOT DIRECT IMPLEMENTATION",
		"NO PLANNING MODE",
		"Resume from the current workspace state instead of restarting from scratch.",
	}
	for _, snippet := range forbiddenSnippets {
		if strings.Contains(promptText, snippet) {
			t.Fatalf("expected PRD prompt to omit %q", snippet)
		}
	}
}

func TestBuildPRDTaskPromptRespectsAutoCommitFlag(t *testing.T) {
	t.Parallel()

	task := model.IssueEntry{
		Name:    "task_2.md",
		AbsPath: "/tmp/tasks/demo/task_2.md",
		Content: `## status: pending
<task_context>
  <domain>frontend</domain>
  <type>bugfix</type>
  <scope>medium</scope>
  <complexity>medium</complexity>
</task_context>
`,
	}

	withAutoCommit := buildPRDTaskPrompt(task, true)
	if !strings.Contains(
		withAutoCommit,
		"Create one local commit after clean verification, self-review, and tracking updates.",
	) {
		t.Fatalf("expected auto-commit prompt to include local commit instructions")
	}

	withoutAutoCommit := buildPRDTaskPrompt(task, false)
	if strings.Contains(
		withoutAutoCommit,
		"Create one local commit after clean verification, self-review, and tracking updates.",
	) {
		t.Fatalf("expected no-auto-commit prompt to omit automatic commit instructions")
	}
	if !strings.Contains(withoutAutoCommit, "Do not create an automatic commit for this run.") {
		t.Fatalf("expected no-auto-commit prompt to mention disabled automatic commits")
	}
}
