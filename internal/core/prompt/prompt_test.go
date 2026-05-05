package prompt

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
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
		"Do not edit issue files outside this batch.",
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
		"Update only the issue files that belong to this batch.",
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
		"<action_required>",
		"Begin work on **task_1.md** immediately",
		"do NOT ask the user",
		"<begin_now>",
		"Start the cy-execute-task workflow on **task_1.md**",
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

// TestBuildPRDTaskPromptIncludesActionDirectives guards the dispatch-bug fix
// from #144: PRD task prompts must open and close with explicit imperatives so
// the agent does not default to a "ready when you are" standby greeting and
// silently produce a no-op session that the runner then misclassifies as a
// successful task completion. Removing either directive without a replacement
// regresses the bug.
func TestBuildPRDTaskPromptIncludesActionDirectives(t *testing.T) {
	t.Parallel()

	task := model.IssueEntry{
		Name:    "task_42.md",
		AbsPath: "/tmp/.compozy/tasks/auth/task_42.md",
		Content: `---
status: pending
title: Wire login flow
type: backend
complexity: medium
---

# Task 42: Wire login flow
`,
	}

	promptText := buildPRDTaskPrompt(task, false, nil)

	openerIdx := strings.Index(promptText, "<action_required>")
	if openerIdx == -1 {
		t.Fatalf("expected PRD prompt to open with <action_required> directive")
	}
	closerIdx := strings.Index(promptText, "<begin_now>")
	if closerIdx == -1 {
		t.Fatalf("expected PRD prompt to close with <begin_now> directive")
	}
	if closerIdx <= openerIdx {
		t.Fatalf("expected <begin_now> after <action_required>, got opener=%d closer=%d", openerIdx, closerIdx)
	}

	// The opener must reference the specific task name and forbid asking for
	// confirmation — those two properties together are what stops the agent
	// from emitting "Ready when you are — let me know if you want me to start".
	requiredOpenerSnippets := []string{
		"Begin work on **task_42.md** immediately",
		"`cy-execute-task`",
		"do NOT ask the user",
	}
	for _, snippet := range requiredOpenerSnippets {
		if !strings.Contains(promptText, snippet) {
			t.Fatalf("expected opener directive to include %q", snippet)
		}
	}

	// The opener must come BEFORE the descriptive context so the agent reads
	// "act now" first and treats subsequent sections as supporting brief.
	titleIdx := strings.Index(promptText, "# Implementation Task:")
	contextIdx := strings.Index(promptText, "## Task Context")
	if titleIdx == -1 || contextIdx == -1 {
		t.Fatalf("expected title and context sections in prompt")
	}
	if titleIdx >= openerIdx || openerIdx >= contextIdx {
		t.Fatalf(
			"expected order: title(%d) -> action_required(%d) -> task context(%d)",
			titleIdx, openerIdx, contextIdx,
		)
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

	prdAddendum, err := BuildSystemPromptAddendum(BatchParams{
		Mode: model.ExecutionModePRDTasks,
		Memory: &WorkflowMemoryContext{
			WorkflowPath:            "/tmp/.compozy/tasks/demo/memory/MEMORY.md",
			TaskPath:                "/tmp/.compozy/tasks/demo/memory/task_1.md",
			TaskNeedsCompaction:     true,
			WorkflowNeedsCompaction: false,
		},
	})
	if err != nil {
		t.Fatalf("BuildSystemPromptAddendum() error = %v", err)
	}
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

	reviewAddendum, err := BuildSystemPromptAddendum(BatchParams{Mode: model.ExecutionModePRReview})
	if err != nil {
		t.Fatalf("BuildSystemPromptAddendum() error = %v", err)
	}
	if reviewAddendum != "" {
		t.Fatalf("expected review mode to omit system prompt addendum, got %q", reviewAddendum)
	}
}

func TestBuildAppliesPromptHookMutations(t *testing.T) {
	t.Parallel()

	const preBuildMarker = "PRE-BUILD-HOOK"
	const postBuildMarker = "\nPOST-BUILD-HOOK"

	manager := &promptHookManager{
		mutators: map[string]func(any) (any, error){
			"prompt.pre_build": func(input any) (any, error) {
				payload := input.(promptPreBuildPayload)
				payload.BatchParams.BatchGroups["task_1"][0].Content += "\n" + preBuildMarker + "\n"
				return payload, nil
			},
			"prompt.post_build": func(input any) (any, error) {
				payload := input.(promptPostBuildPayload)
				payload.PromptText += postBuildMarker
				return payload, nil
			},
		},
	}

	promptText, err := Build(BatchParams{
		Mode:       model.ExecutionModePRDTasks,
		RunID:      "run-123",
		JobID:      "job-123",
		RuntimeMgr: manager,
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
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !strings.Contains(promptText, preBuildMarker) {
		t.Fatalf("expected prompt to include pre-build marker, got:\n%s", promptText)
	}
	if !strings.Contains(promptText, postBuildMarker) {
		t.Fatalf("expected prompt to include post-build marker, got:\n%s", promptText)
	}

	wantHooks := []string{"prompt.pre_build", "prompt.post_build"}
	if got := manager.mutableHooks; !reflect.DeepEqual(got, wantHooks) {
		t.Fatalf("unexpected prompt hook order\nwant: %#v\ngot:  %#v", wantHooks, got)
	}
}

func TestBuildSystemPromptAddendumAppliesPreSystemMutation(t *testing.T) {
	t.Parallel()

	const marker = "\nPRE-SYSTEM-HOOK"
	manager := &promptHookManager{
		mutators: map[string]func(any) (any, error){
			"prompt.pre_system": func(input any) (any, error) {
				payload := input.(promptPreSystemPayload)
				payload.SystemAddendum += marker
				return payload, nil
			},
		},
	}

	addendum, err := BuildSystemPromptAddendum(BatchParams{
		Mode:       model.ExecutionModePRDTasks,
		RunID:      "run-321",
		JobID:      "job-321",
		RuntimeMgr: manager,
		Memory: &WorkflowMemoryContext{
			WorkflowPath: "/tmp/.compozy/tasks/demo/memory/MEMORY.md",
			TaskPath:     "/tmp/.compozy/tasks/demo/memory/task_1.md",
		},
	})
	if err != nil {
		t.Fatalf("BuildSystemPromptAddendum() error = %v", err)
	}
	if !strings.Contains(addendum, marker) {
		t.Fatalf("expected system addendum to include pre-system marker, got:\n%s", addendum)
	}

	wantHooks := []string{"prompt.pre_system"}
	if got := manager.mutableHooks; !reflect.DeepEqual(got, wantHooks) {
		t.Fatalf("unexpected system hook order\nwant: %#v\ngot:  %#v", wantHooks, got)
	}
}

func TestSafeFileName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		wantPrefix string
	}{
		{
			name:       "Should preserve a sanitized prefix for windows separators",
			input:      `dir\subdir/file.go`,
			wantPrefix: "dir_subdir_file.go-",
		},
		{
			name:       "Should preserve a sanitized prefix for spaced paths",
			input:      "dir with spaces/file.go",
			wantPrefix: "dir_with_spaces_file.go-",
		},
	}

	suffixPattern := regexp.MustCompile(`-[a-f0-9]{6}$`)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := SafeFileName(tc.input)
			if !strings.HasPrefix(got, tc.wantPrefix) {
				t.Fatalf("unexpected safe file name prefix: %q", got)
			}
			if !suffixPattern.MatchString(got) {
				t.Fatalf("expected hashed suffix in %q", got)
			}
		})
	}

	t.Run("Should produce different names for inputs with the same sanitized base", func(t *testing.T) {
		t.Parallel()

		first := SafeFileName("dir file.go")
		second := SafeFileName("dir\tfile.go")
		if first == second {
			t.Fatalf("expected collision-resistant file names, got %q", first)
		}
	})
}

func TestBuildDispatchesByMode(t *testing.T) {
	t.Parallel()

	prdPrompt, err := Build(BatchParams{
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
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !strings.Contains(prdPrompt, "# Implementation Task: task_1.md") {
		t.Fatalf("expected PRD build dispatch, got:\n%s", prdPrompt)
	}

	reviewPrompt, err := Build(BatchParams{
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
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
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

type promptHookManager struct {
	mutators     map[string]func(any) (any, error)
	mutableHooks []string
}

func (*promptHookManager) Start(context.Context) error { return nil }

func (*promptHookManager) Shutdown(context.Context) error { return nil }

func (m *promptHookManager) DispatchMutableHook(
	_ context.Context,
	hook string,
	input any,
) (any, error) {
	if m == nil {
		return input, nil
	}
	m.mutableHooks = append(m.mutableHooks, hook)
	if m.mutators == nil {
		return input, nil
	}
	if mutate := m.mutators[hook]; mutate != nil {
		return mutate(input)
	}
	return input, nil
}

func (*promptHookManager) DispatchObserverHook(context.Context, string, any) {}
