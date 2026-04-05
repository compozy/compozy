package prompt

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"gopkg.in/yaml.v3"
)

func TestClaudeReasoningPromptUsesEmbeddedTemplates(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		reasoningEffortLow:    "Think concisely and act quickly. Prefer direct solutions.",
		reasoningEffortMedium: "Think hard through problems carefully before acting. Balance speed with thoroughness.",
		reasoningEffortHigh:   "Ultrathink deeply and comprehensively before taking action.",
		reasoningEffortXHigh:  "Ultra-deep thinking mode: Exhaustively analyze every aspect of the problem.",
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
		ReviewsDir: "/tmp/.compozy/tasks/my-feature/reviews-001",
		AutoCommit: true,
		Mode:       model.ExecutionModePRReview,
		BatchGroups: map[string][]model.IssueEntry{
			"internal/app/service.go": {
				{
					Name:     "issue_003.md",
					AbsPath:  "/tmp/.compozy/tasks/my-feature/reviews-001/issue_003.md",
					CodeFile: "internal/app/service.go",
				},
				{
					Name:     "issue_004.md",
					AbsPath:  "/tmp/.compozy/tasks/my-feature/reviews-001/issue_004.md",
					CodeFile: "internal/app/service.go",
				},
			},
		},
	})

	requiredSnippets := []string{
		"`cy-fix-reviews`",
		"`cy-final-verify`",
		"<batch_issue_files>",
		"Review round: `001`",
		"Issue range: `issue_003.md` → `issue_004.md`",
		"Compozy resolves provider threads after the batch succeeds.",
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
	for _, snippet := range []string{"Grouped summaries:", "grouped tracker", "/grouped/"} {
		if strings.Contains(promptText, snippet) {
			t.Fatalf("expected review prompt to omit grouped-summary reference %q", snippet)
		}
	}
}

func TestBuildCodeReviewPromptRespectsManualCommitMode(t *testing.T) {
	t.Parallel()

	promptText := buildCodeReviewPrompt(BatchParams{
		Name:       "my-feature",
		Round:      2,
		Provider:   "coderabbit",
		PR:         "260",
		ReviewsDir: "/tmp/.compozy/tasks/my-feature/reviews-002",
		AutoCommit: false,
		Mode:       model.ExecutionModePRReview,
		BatchGroups: map[string][]model.IssueEntry{
			"internal/app/service.go": {
				{
					Name:     "issue_007.md",
					AbsPath:  "/tmp/.compozy/tasks/my-feature/reviews-002/issue_007.md",
					CodeFile: "internal/app/service.go",
				},
			},
		},
	})

	requiredSnippets := []string{
		"Automatic commits: disabled (`--auto-commit=false`)",
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
		AbsPath: "/tmp/.compozy/tasks/demo/task_1.md",
		Content: `---
status: pending
title: Example
type: backend
complexity: low
---

# Task 1: Example
`,
	}

	promptText := buildPRDTaskPrompt(task, false, &WorkflowMemoryContext{
		Directory:    "/tmp/.compozy/tasks/demo/memory",
		WorkflowPath: "/tmp/.compozy/tasks/demo/memory/MEMORY.md",
		TaskPath:     "/tmp/.compozy/tasks/demo/memory/task_1.md",
	})

	requiredSnippets := []string{
		"`cy-workflow-memory`",
		"`cy-execute-task`",
		"`cy-final-verify`",
		"## Workflow Memory",
		"Shared workflow memory: `/tmp/.compozy/tasks/demo/memory/MEMORY.md`",
		"Current task memory: `/tmp/.compozy/tasks/demo/memory/task_1.md`",
		"## Task Files",
		"Task file: `/tmp/.compozy/tasks/demo/task_1.md`",
		"Master tasks file: `/tmp/.compozy/tasks/demo/_tasks.md`",
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
		AbsPath: "/tmp/.compozy/tasks/demo/task_2.md",
		Content: `---
status: pending
title: Example
type: bugfix
complexity: medium
---

# Task 2: Example
`,
	}

	withAutoCommit := buildPRDTaskPrompt(task, true, nil)
	if !strings.Contains(
		withAutoCommit,
		"Create one local commit after clean verification, self-review, and tracking updates.",
	) {
		t.Fatalf("expected auto-commit prompt to include local commit instructions")
	}

	withoutAutoCommit := buildPRDTaskPrompt(task, false, nil)
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

func TestBuildPRDTaskPromptFlagsOversizedMemoryFiles(t *testing.T) {
	t.Parallel()

	task := model.IssueEntry{
		Name:    "task_3.md",
		AbsPath: "/tmp/.compozy/tasks/demo/task_3.md",
		Content: `---
status: pending
title: Example
type: backend
complexity: low
---

# Task 3: Example
`,
	}

	promptText := buildPRDTaskPrompt(task, false, &WorkflowMemoryContext{
		Directory:               "/tmp/.compozy/tasks/demo/memory",
		WorkflowPath:            "/tmp/.compozy/tasks/demo/memory/MEMORY.md",
		TaskPath:                "/tmp/.compozy/tasks/demo/memory/task_3.md",
		WorkflowNeedsCompaction: true,
		TaskNeedsCompaction:     true,
	})

	requiredSnippets := []string{
		"Compact the flagged memory files before proceeding with implementation.",
		"Shared workflow memory is over its soft limit: `/tmp/.compozy/tasks/demo/memory/MEMORY.md`",
		"Current task memory is over its soft limit: `/tmp/.compozy/tasks/demo/memory/task_3.md`",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(promptText, snippet) {
			t.Fatalf("expected PRD prompt to include %q", snippet)
		}
	}
}

func TestBuildSystemPromptAddendumIncludesWorkflowMemoryOnlyForPRDTasks(t *testing.T) {
	t.Parallel()

	prdAddendum := BuildSystemPromptAddendum(BatchParams{
		Mode: model.ExecutionModePRDTasks,
		Memory: &WorkflowMemoryContext{
			WorkflowPath:            "/tmp/.compozy/tasks/demo/memory/MEMORY.md",
			TaskPath:                "/tmp/.compozy/tasks/demo/memory/task_1.md",
			TaskNeedsCompaction:     true,
			WorkflowNeedsCompaction: false,
		},
	})
	requiredSnippets := []string{
		"<workflow_memory>",
		"`cy-workflow-memory`",
		"/tmp/.compozy/tasks/demo/memory/MEMORY.md",
		"/tmp/.compozy/tasks/demo/memory/task_1.md",
		"compact current task memory first",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(prdAddendum, snippet) {
			t.Fatalf("expected system prompt addendum to include %q", snippet)
		}
	}

	reviewAddendum := BuildSystemPromptAddendum(BatchParams{Mode: model.ExecutionModePRReview})
	if reviewAddendum != "" {
		t.Fatalf("expected review mode to omit system prompt addendum, got %q", reviewAddendum)
	}
}

func TestParseTaskFileHandlesV2AndLegacyMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantTask    model.TaskEntry
		wantErrIs   error
		wantErrText string
	}{
		{
			name: "parses v2 frontmatter with title",
			content: `---
status: pending
title: Example Task
type: backend
complexity: high
dependencies:
  - task_01
  - task_02
---

# Task 1: Example Task
`,
			wantTask: model.TaskEntry{
				Status:       "pending",
				Title:        "Example Task",
				TaskType:     "backend",
				Complexity:   "high",
				Dependencies: []string{"task_01", "task_02"},
			},
		},
		{
			name: "returns v1 sentinel for frontmatter with scope",
			content: `---
status: pending
title: Example Task
type: backend
scope: full
complexity: high
---

# Task 1: Example Task
`,
			wantErrIs: ErrV1TaskMetadata,
		},
		{
			name: "returns v1 sentinel for frontmatter with domain",
			content: `---
status: pending
title: Example Task
domain: core-runtime
type: backend
complexity: high
---

# Task 1: Example Task
`,
			wantErrIs: ErrV1TaskMetadata,
		},
		{
			name: "returns legacy sentinel for xml metadata",
			content: strings.Join([]string{
				"## status: pending",
				"<task_context><domain>backend</domain><type>backend</type><scope>small</scope><complexity>low</complexity></task_context>",
				"# Task 1: Example Task",
				"",
			}, "\n"),
			wantErrIs: ErrLegacyTaskMetadata,
		},
		{
			name: "allows missing title in v2 parser",
			content: `---
status: pending
type: backend
complexity: medium
---

# Task 1: Example Task
`,
			wantTask: model.TaskEntry{
				Status:     "pending",
				Title:      "",
				TaskType:   "backend",
				Complexity: "medium",
			},
		},
		{
			name: "requires status",
			content: `---
title: Example Task
type: backend
complexity: medium
---

# Task 1: Example Task
`,
			wantErrText: "task front matter missing status",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			task, err := ParseTaskFile(tt.content)
			if tt.wantErrIs != nil || tt.wantErrText != "" {
				if err == nil {
					t.Fatal("expected parse error")
				}
				if tt.wantErrIs != nil && !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("expected error %v, got %v", tt.wantErrIs, err)
				}
				if tt.wantErrText != "" && !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parse task file: %v", err)
			}
			if task.Status != tt.wantTask.Status ||
				task.Title != tt.wantTask.Title ||
				task.TaskType != tt.wantTask.TaskType ||
				task.Complexity != tt.wantTask.Complexity ||
				!reflect.DeepEqual(task.Dependencies, tt.wantTask.Dependencies) {
				t.Fatalf("unexpected parsed task\nwant: %#v\ngot:  %#v", tt.wantTask, task)
			}
		})
	}
}

func TestParseTaskFileFromTempDirFixture(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	taskPath := filepath.Join(dir, "task_01.md")
	content := `---
status: pending
title: Fixture Task
type: api
complexity: high
dependencies:
  - task_00
---

# Task 1: Fixture Task
`
	if err := os.WriteFile(taskPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write task fixture: %v", err)
	}

	body, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read task fixture: %v", err)
	}

	task, err := ParseTaskFile(string(body))
	if err != nil {
		t.Fatalf("parse task fixture: %v", err)
	}
	if task.Title != "Fixture Task" || task.TaskType != "api" || task.Complexity != "high" {
		t.Fatalf("unexpected parsed fixture task: %#v", task)
	}
	if !reflect.DeepEqual(task.Dependencies, []string{"task_00"}) {
		t.Fatalf("unexpected fixture dependencies: %#v", task.Dependencies)
	}
}

func TestLegacyTaskParsingHelpers(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		"## status: pending",
		"",
		"<task_context>",
		"  <domain>backend</domain>",
		"  <type>backend</type>",
		"  <scope>small</scope>",
		"  <complexity>high</complexity>",
		"  <dependencies>task_01, task_02</dependencies>",
		"</task_context>",
		"",
		"# Task 1: Legacy Example",
		"",
		"Legacy body.",
		"",
	}, "\n")

	task, err := ParseLegacyTaskFile(content)
	if err != nil {
		t.Fatalf("parse legacy task file: %v", err)
	}
	if task.Status != "pending" || task.TaskType != "backend" || task.Complexity != "high" {
		t.Fatalf("unexpected legacy task parse: %#v", task)
	}
	if !reflect.DeepEqual(task.Dependencies, []string{"task_01", "task_02"}) {
		t.Fatalf("unexpected legacy dependencies: %#v", task.Dependencies)
	}

	body, err := ExtractLegacyTaskBody(content)
	if err != nil {
		t.Fatalf("extract legacy body: %v", err)
	}
	if strings.Contains(body, "<task_context>") || strings.Contains(body, "## status:") {
		t.Fatalf("expected legacy body extraction to remove metadata, got:\n%s", body)
	}
	if !strings.Contains(body, "# Task 1: Legacy Example") || !strings.Contains(body, "Legacy body.") {
		t.Fatalf("expected body content to remain, got:\n%s", body)
	}
}

func TestTaskMetadataHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		task model.TaskEntry
		want bool
	}{
		{name: "completed is terminal", task: model.TaskEntry{Status: "completed"}, want: true},
		{name: "done is terminal", task: model.TaskEntry{Status: "done"}, want: true},
		{name: "finished is terminal", task: model.TaskEntry{Status: "finished"}, want: true},
		{name: "pending is not terminal", task: model.TaskEntry{Status: "pending"}, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsTaskCompleted(tt.task); got != tt.want {
				t.Fatalf("unexpected completion result: got %v want %v", got, tt.want)
			}
		})
	}

	if got := ExtractTaskNumber("task_042.md"); got != 42 {
		t.Fatalf("unexpected task number: %d", got)
	}
	if got := SafeFileName(`dir\subdir/file.go`); !strings.HasPrefix(got, "dir_subdir_file.go-") {
		t.Fatalf("unexpected safe file name: %q", got)
	}
}

func TestHasTaskV1FrontMatterKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rawYAML string
		want    bool
	}{
		{
			name: "detects domain",
			rawYAML: `
status: pending
domain: backend
type: backend
`,
			want: true,
		},
		{
			name: "detects scope case insensitively",
			rawYAML: `
status: pending
Scope: full
type: backend
`,
			want: true,
		},
		{
			name: "ignores v2 metadata",
			rawYAML: `
status: pending
title: Example
type: backend
`,
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var node yaml.Node
			if err := yaml.Unmarshal([]byte(strings.TrimSpace(tt.rawYAML)), &node); err != nil {
				t.Fatalf("unmarshal yaml node: %v", err)
			}
			if got := hasTaskV1FrontMatterKeys(&node); got != tt.want {
				t.Fatalf("unexpected v1 detection: got %v want %v", got, tt.want)
			}
		})
	}

	if hasTaskV1FrontMatterKeys(&yaml.Node{Kind: yaml.SequenceNode}) {
		t.Fatal("expected non-mapping node to be ignored")
	}
}

func TestBuildDispatchesByMode(t *testing.T) {
	t.Parallel()

	prdPrompt := Build(BatchParams{
		Mode:       model.ExecutionModePRDTasks,
		AutoCommit: false,
		BatchGroups: map[string][]model.IssueEntry{
			"task_1": {{
				Name:    "task_1.md",
				AbsPath: "/tmp/.compozy/tasks/demo/task_1.md",
				Content: `---
status: pending
title: Demo
type: backend
complexity: low
---

# Task 1: Demo
`,
			}},
		},
	})
	if !strings.Contains(prdPrompt, "# Implementation Task: task_1.md") {
		t.Fatalf("expected PRD build dispatch, got:\n%s", prdPrompt)
	}

	reviewPrompt := Build(BatchParams{
		Mode:       model.ExecutionModePRReview,
		Name:       "demo",
		Provider:   "coderabbit",
		PR:         "123",
		Round:      1,
		ReviewsDir: "/tmp/.compozy/tasks/demo/reviews-001",
		BatchGroups: map[string][]model.IssueEntry{
			"internal/app/service.go": {{
				Name:     "issue_001.md",
				AbsPath:  "/tmp/.compozy/tasks/demo/reviews-001/issue_001.md",
				CodeFile: "internal/app/service.go",
			}},
		},
	})
	if !strings.Contains(reviewPrompt, "<arguments>") {
		t.Fatalf("expected review build dispatch, got:\n%s", reviewPrompt)
	}
}

func TestFlattenAndSortIssues(t *testing.T) {
	t.Parallel()

	prdGroups := map[string][]model.IssueEntry{
		"b": {{Name: "task_10.md"}},
		"a": {{Name: "task_2.md"}},
	}
	prdIssues := FlattenAndSortIssues(prdGroups, model.ExecutionModePRDTasks)
	if got := []string{
		prdIssues[0].Name,
		prdIssues[1].Name,
	}; !reflect.DeepEqual(
		got,
		[]string{"task_2.md", "task_10.md"},
	) {
		t.Fatalf("unexpected prd ordering: %#v", got)
	}

	reviewGroups := map[string][]model.IssueEntry{
		"b": {{Name: "issue_010.md"}},
		"a": {{Name: "issue_002.md"}},
	}
	reviewIssues := FlattenAndSortIssues(reviewGroups, model.ExecutionModePRReview)
	if got := []string{
		reviewIssues[0].Name,
		reviewIssues[1].Name,
	}; !reflect.DeepEqual(
		got,
		[]string{"issue_002.md", "issue_010.md"},
	) {
		t.Fatalf("unexpected review ordering: %#v", got)
	}
}

func TestReviewParsingHelpers(t *testing.T) {
	t.Parallel()

	reviewContent := `---
status: resolved
file: internal/app/service.go
line: 42
severity: high
author: review-bot
provider_ref: thread:1
---

Review body.
`
	ctx, err := ParseReviewContext(reviewContent)
	if err != nil {
		t.Fatalf("parse review context: %v", err)
	}
	if ctx.Status != "resolved" || ctx.File != "internal/app/service.go" || ctx.Line != 42 {
		t.Fatalf("unexpected review context: %#v", ctx)
	}
	status, err := ParseReviewStatus(reviewContent)
	if err != nil {
		t.Fatalf("parse review status: %v", err)
	}
	if status != "resolved" {
		t.Fatalf("unexpected review status: %q", status)
	}
	resolved, err := IsReviewResolved(reviewContent)
	if err != nil {
		t.Fatalf("is review resolved: %v", err)
	}
	if !resolved {
		t.Fatal("expected resolved review to be terminal")
	}

	legacyContent := strings.Join([]string{
		"# Issue 001",
		"",
		"## Status: pending",
		"",
		"<review_context>",
		"  <file>internal/app/service.go</file>",
		"  <line>7</line>",
		"  <severity>medium</severity>",
		"  <author>review-bot</author>",
		"  <provider_ref>thread:1</provider_ref>",
		"</review_context>",
		"",
		"Legacy review body.",
		"",
	}, "\n")
	if !LooksLikeLegacyReviewFile(legacyContent) {
		t.Fatal("expected legacy review detection")
	}
	if _, err := ParseReviewContext(legacyContent); !errors.Is(err, ErrLegacyReviewMetadata) {
		t.Fatalf("expected legacy review sentinel, got %v", err)
	}

	legacyCtx, err := ParseLegacyReviewContext(legacyContent)
	if err != nil {
		t.Fatalf("parse legacy review context: %v", err)
	}
	if legacyCtx.Status != "pending" || legacyCtx.File != "internal/app/service.go" || legacyCtx.Line != 7 {
		t.Fatalf("unexpected legacy review context: %#v", legacyCtx)
	}

	legacyBody, err := ExtractLegacyReviewBody(legacyContent)
	if err != nil {
		t.Fatalf("extract legacy review body: %v", err)
	}
	if strings.Contains(legacyBody, "<review_context>") || strings.Contains(legacyBody, "## Status:") {
		t.Fatalf("expected legacy review body extraction to remove metadata, got:\n%s", legacyBody)
	}
	if !strings.Contains(legacyBody, "Legacy review body.") {
		t.Fatalf("expected legacy review body content to remain, got:\n%s", legacyBody)
	}
}
