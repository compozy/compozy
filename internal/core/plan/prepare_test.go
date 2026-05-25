package plan

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/memory"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
)

func TestReadTaskEntriesSortsNumericallyAndFiltersCompleted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := map[string]string{
		"task_10.md": "---\nstatus: pending\ntitle: Task 10\ntype: backend\ncomplexity: low\n---\n\n# Task 10\n",
		"task_2.md":  "---\nstatus: pending\ntitle: Task 2\ntype: backend\ncomplexity: low\n---\n\n# Task 2\n",
		"task_3.md":  "---\nstatus: completed\ntitle: Task 3\ntype: backend\ncomplexity: low\n---\n\n# Task 3\n",
		"notes.md":   "ignored\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	entries, err := readTaskEntries(dir, false, false)
	if err != nil {
		t.Fatalf("readTaskEntries: %v", err)
	}

	gotNames := []string{entries[0].Name, entries[1].Name}
	wantNames := []string{"task_2.md", "task_10.md"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected task order\nwant: %#v\ngot:  %#v", wantNames, gotNames)
	}
	if len(entries) != 2 {
		t.Fatalf("expected completed tasks to be filtered, got %d entries", len(entries))
	}
}

func TestResolveInputsUsesDefaultPRDDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)
	if err := os.MkdirAll(model.TaskDirectory("demo"), 0o755); err != nil {
		t.Fatalf("mkdir prd dir: %v", err)
	}

	prValue, inputDir, resolved, err := resolveInputs(&model.RuntimeConfig{
		Name: "demo",
		Mode: model.ExecutionModePRDTasks,
	})
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if prValue != "demo" {
		t.Fatalf("unexpected pr value: %q", prValue)
	}
	if inputDir != model.TaskDirectory("demo") {
		t.Fatalf("unexpected input dir: %q", inputDir)
	}
	wantResolved := filepath.Join(tmp, model.TaskDirectory("demo"))
	if resolved != wantResolved {
		t.Fatalf("unexpected resolved dir\nwant: %q\ngot:  %q", wantResolved, resolved)
	}
}

func TestResolveInputsUsesWorkspaceRootForDefaultTaskDirectory(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	t.Chdir(nested)

	tasksDir := filepath.Join(root, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	name, inputDir, resolved, err := resolveInputs(&model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: root,
		Mode:          model.ExecutionModePRDTasks,
	})
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if name != "demo" {
		t.Fatalf("unexpected resolved name: %q", name)
	}
	if inputDir != tasksDir {
		t.Fatalf("unexpected input dir\nwant: %q\ngot:  %q", tasksDir, inputDir)
	}
	if resolved != tasksDir {
		t.Fatalf("unexpected resolved dir\nwant: %q\ngot:  %q", tasksDir, resolved)
	}
}

func TestValidateAndFilterEntriesReportsCompletedTaskWorkflowsSeparately(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"task_1.md": "---\nstatus: completed\ntitle: Task 1\ntype: backend\ncomplexity: low\n---\n\n# Task 1\n",
		"task_2.md": "---\nstatus: done\ntitle: Task 2\ntype: backend\ncomplexity: low\n---\n\n# Task 2\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if _, err := tasks.RefreshTaskMeta(dir); err != nil {
		t.Fatalf("refresh task meta: %v", err)
	}

	entries, err := readTaskEntries(dir, false, false)
	if err != nil {
		t.Fatalf("readTaskEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected all completed tasks to be filtered, got %d entries", len(entries))
	}

	var gotErr error
	output := captureSlogOutput(t, func() {
		_, gotErr = validateAndFilterEntries(entries, &model.RuntimeConfig{
			Mode:             model.ExecutionModePRDTasks,
			TasksDir:         dir,
			IncludeCompleted: false,
		})
	})

	if gotErr == nil || !errors.Is(gotErr, ErrNoWork) {
		t.Fatalf("expected ErrNoWork, got %v", gotErr)
	}
	if !strings.Contains(output, "All task files are already completed. Nothing to do.") {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestValidateAndFilterEntriesKeepsEmptyTaskDirectoriesDistinct(t *testing.T) {
	dir := t.TempDir()
	if _, err := tasks.RefreshTaskMeta(dir); err != nil {
		t.Fatalf("refresh task meta: %v", err)
	}

	entries, err := readTaskEntries(dir, false, false)
	if err != nil {
		t.Fatalf("readTaskEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no task entries, got %d", len(entries))
	}

	var gotErr error
	output := captureSlogOutput(t, func() {
		_, gotErr = validateAndFilterEntries(entries, &model.RuntimeConfig{
			Mode:             model.ExecutionModePRDTasks,
			TasksDir:         dir,
			IncludeCompleted: false,
		})
	})

	if gotErr == nil || !errors.Is(gotErr, ErrNoWork) {
		t.Fatalf("expected ErrNoWork, got %v", gotErr)
	}
	if !strings.Contains(output, "No task files found.") {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestResolveInputsInfersTaskNameFromTasksDir(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	tasksDir := filepath.Join(tmp, model.TasksBaseDir(), "multi-repo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	prValue, inputDir, resolved, err := resolveInputs(&model.RuntimeConfig{
		TasksDir: tasksDir,
		Mode:     model.ExecutionModePRDTasks,
	})
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if prValue != "multi-repo" {
		t.Fatalf("expected inferred task name multi-repo, got %q", prValue)
	}
	if inputDir != tasksDir {
		t.Fatalf("expected input dir to remain unchanged, got %q", inputDir)
	}
	if resolved != tasksDir {
		t.Fatalf("expected resolved dir %q, got %q", tasksDir, resolved)
	}
}

func TestReadTaskEntriesRejectsV1TaskArtifactsWithMigrateGuidance(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `---
status: pending
domain: backend
type: backend
scope: full
complexity: low
---

# Task 1: Example
`
	if err := os.WriteFile(filepath.Join(dir, "task_1.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("write v1 task: %v", err)
	}

	_, err := readTaskEntries(dir, false, false)
	if err == nil {
		t.Fatal("expected readTaskEntries to fail for v1 task metadata")
	}
	if !strings.Contains(err.Error(), "run `compozy migrate`") {
		t.Fatalf("expected migrate guidance, got %v", err)
	}
}

func TestPrepareJobsForPRDTasksForcesSingleBatchPerTask(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runArtifacts := model.NewRunArtifacts(workspaceRoot, "tasks-demo-test-run")
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}
	issuesDir := t.TempDir()
	groups := map[string][]model.IssueEntry{
		"task_1": {
			{
				Name:     "task_1.md",
				AbsPath:  filepath.Join(issuesDir, "task_1.md"),
				Content:  "---\nstatus: pending\ntitle: Task 1\ntype: backend\ncomplexity: low\n---\n\n# Task 1\n",
				CodeFile: "task_1",
			},
		},
		"task_2": {
			{
				Name:     "task_2.md",
				AbsPath:  filepath.Join(issuesDir, "task_2.md"),
				Content:  "---\nstatus: pending\ntitle: Task 2\ntype: backend\ncomplexity: low\n---\n\n# Task 2\n",
				CodeFile: "task_2",
			},
		},
	}

	jobs, err := prepareJobs(context.Background(), &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      issuesDir,
		BatchSize:     5,
		Mode:          model.ExecutionModePRDTasks,
	}, groups, runArtifacts, nil, nil)
	if err != nil {
		t.Fatalf("prepareJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected one batch per task in prd mode, got %d", len(jobs))
	}
	for _, job := range jobs {
		if len(job.CodeFiles) != 1 {
			t.Fatalf("expected single-file jobs in prd mode, got %#v", job.CodeFiles)
		}
		if job.TaskTitle == "" {
			t.Fatalf("expected prd job to carry task title, got %#v", job)
		}
		if got, want := job.TaskType, "backend"; got != want {
			t.Fatalf("expected prd job type %q, got %q", want, got)
		}
		assertJobUsesRunArtifacts(t, runArtifacts, job)
		if _, err := os.Stat(job.OutPromptPath); err != nil {
			t.Fatalf("expected prompt artifact to be written: %v", err)
		}
		if _, err := os.Stat(memory.WorkflowPath(issuesDir)); err != nil {
			t.Fatalf("expected workflow memory artifact to be written: %v", err)
		}
		if _, err := os.Stat(memory.TaskPath(issuesDir, job.CodeFiles[0]+".md")); err != nil {
			t.Fatalf("expected task memory artifact to be written: %v", err)
		}
		if !strings.Contains(job.SystemPrompt, "<workflow_memory>") {
			t.Fatalf("expected prd job to include workflow-memory system prompt, got %q", job.SystemPrompt)
		}
	}
}

func TestPrepareJobsResolvesPerTaskRuntimeOverrides(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runArtifacts := model.NewRunArtifacts(workspaceRoot, "tasks-runtime-test-run")
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}
	tasksDir := t.TempDir()
	groups := map[string][]model.IssueEntry{
		"task_01": {{
			Name:     "task_01.md",
			AbsPath:  filepath.Join(tasksDir, "task_01.md"),
			Content:  "---\nstatus: pending\ntitle: Frontend task\ntype: frontend\ncomplexity: low\n---\n\n# Task 1\n",
			CodeFile: "task_01",
		}},
		"task_02": {{
			Name:     "task_02.md",
			AbsPath:  filepath.Join(tasksDir, "task_02.md"),
			Content:  "---\nstatus: pending\ntitle: Backend task\ntype: backend\ncomplexity: low\n---\n\n# Task 2\n",
			CodeFile: "task_02",
		}},
	}

	jobs, err := prepareJobs(context.Background(), &model.RuntimeConfig{
		Name:            "demo",
		WorkspaceRoot:   workspaceRoot,
		TasksDir:        tasksDir,
		IDE:             model.IDECodex,
		Model:           "gpt-5.5",
		ReasoningEffort: "medium",
		Mode:            model.ExecutionModePRDTasks,
		TaskRuntimeRules: []model.TaskRuntimeRule{
			{
				Type:            testStringPointer("frontend"),
				IDE:             testStringPointer(model.IDEClaude),
				Model:           testStringPointer("sonnet"),
				ReasoningEffort: testStringPointer("high"),
			},
			{
				ID:    testStringPointer("task_02"),
				Model: testStringPointer("codex-fast"),
			},
		},
	}, groups, runArtifacts, nil, nil)
	if err != nil {
		t.Fatalf("prepareJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].IDE != model.IDEClaude || jobs[0].Model != "sonnet" || jobs[0].ReasoningEffort != "high" {
		t.Fatalf("unexpected frontend runtime: %#v", jobs[0])
	}
	if jobs[1].IDE != model.IDECodex || jobs[1].Model != "codex-fast" || jobs[1].ReasoningEffort != "medium" {
		t.Fatalf("unexpected backend runtime: %#v", jobs[1])
	}
}

func TestPrepareJobsRejectsPerTaskRuntimeThatCannotReuseGlobalAddDirs(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runArtifacts := model.NewRunArtifacts(workspaceRoot, "tasks-invalid-runtime-test-run")
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}
	tasksDir := t.TempDir()
	groups := map[string][]model.IssueEntry{
		"task_01": {{
			Name:     "task_01.md",
			AbsPath:  filepath.Join(tasksDir, "task_01.md"),
			Content:  "---\nstatus: pending\ntitle: Frontend task\ntype: frontend\ncomplexity: low\n---\n\n# Task 1\n",
			CodeFile: "task_01",
		}},
	}

	_, err := prepareJobs(context.Background(), &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      tasksDir,
		IDE:           model.IDECodex,
		AddDirs:       []string{"../shared"},
		Mode:          model.ExecutionModePRDTasks,
		TaskRuntimeRules: []model.TaskRuntimeRule{{
			ID:  testStringPointer("task_01"),
			IDE: testStringPointer(model.IDEGemini),
		}},
	}, groups, runArtifacts, nil, nil)
	if err == nil {
		t.Fatal("expected incompatible per-task runtime error")
	}
	if !strings.Contains(err.Error(), `resolve runtime for task "task_01"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildBatchJobWrapsMemoryPreparationErrorWithTaskPath(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "tasks-demo-test-run")
	tasksDirFile := filepath.Join(t.TempDir(), "tasks.md")
	if err := os.WriteFile(tasksDirFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write tasks dir sentinel file: %v", err)
	}

	issuePath := filepath.Join(t.TempDir(), "task_01.md")
	_, err := buildBatchJob(
		context.Background(),
		&model.RuntimeConfig{
			Name:     "demo",
			TasksDir: tasksDirFile,
			Mode:     model.ExecutionModePRDTasks,
		},
		runArtifacts,
		nil,
		0,
		[]model.IssueEntry{
			{
				Name:    "task_01.md",
				AbsPath: issuePath,
				Content: "---\nstatus: pending\ntitle: Task 1\ntype: backend\ncomplexity: low\n---\n\n# Task 1\n",
			},
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected buildBatchJob to fail when workflow memory cannot be prepared")
	}
	if !strings.Contains(err.Error(), "prepare memory for "+issuePath) {
		t.Fatalf("expected wrapped task path in memory preparation error, got %v", err)
	}
}

func TestPrepareJobsWrapsBatchBuildFailuresWithBatchIndex(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runArtifacts := model.NewRunArtifacts(workspaceRoot, "tasks-demo-batch-error-test-run")
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}

	tasksDir := t.TempDir()
	taskOnePath := filepath.Join(tasksDir, "task_1.md")
	taskTwoPath := filepath.Join(tasksDir, "task_2.md")
	groups := map[string][]model.IssueEntry{
		"task_1": {
			{
				Name:     "task_1.md",
				AbsPath:  taskOnePath,
				Content:  "---\nstatus: pending\ntitle: Task 1\ntype: backend\ncomplexity: low\n---\n\n# Task 1\n",
				CodeFile: "task_1",
			},
		},
		"task_2": {
			{
				Name:     "task_2.md",
				AbsPath:  taskTwoPath,
				Content:  "---\ntitle: Task 2\ntype: backend\ncomplexity: low\n---\n\n# Task 2\n",
				CodeFile: "task_2",
			},
		},
	}

	_, err := prepareJobs(context.Background(), &model.RuntimeConfig{
		Name:     "demo",
		TasksDir: tasksDir,
		Mode:     model.ExecutionModePRDTasks,
	}, groups, runArtifacts, nil, nil)
	if err == nil {
		t.Fatal("expected prepareJobs to fail when the second batch task is invalid")
	}
	if !strings.Contains(err.Error(), "build batch 2/2:") {
		t.Fatalf("expected batch index wrapper in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "parse task artifact "+taskTwoPath) {
		t.Fatalf("expected wrapped task parse error for second batch, got %v", err)
	}
}

func TestPrepareJobsForReviewModeUsesSharedRunArtifactsLayout(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	runArtifacts := model.NewRunArtifacts(workspaceRoot, "reviews-demo-round-007-test-run")
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}
	groups := map[string][]model.IssueEntry{
		"internal/app/service.go": {
			{
				Name:     "issue_001.md",
				AbsPath:  filepath.Join(t.TempDir(), "issue_001.md"),
				Content:  "---\nstatus: pending\nfile: internal/app/service.go\n---\n\n# Issue 1\n",
				CodeFile: "internal/app/service.go",
			},
		},
	}

	jobs, err := prepareJobs(context.Background(), &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		Round:         7,
		BatchSize:     3,
		Mode:          model.ExecutionModePRReview,
	}, groups, runArtifacts, nil, nil)
	if err != nil {
		t.Fatalf("prepareJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one review batch, got %d", len(jobs))
	}

	job := jobs[0]
	assertJobUsesRunArtifacts(t, runArtifacts, job)
	baseName := filepath.Base(job.OutPromptPath)
	if !strings.HasPrefix(baseName, "internal_app_service.go-") || !strings.HasSuffix(baseName, ".prompt.md") {
		t.Fatalf("unexpected review prompt filename: %q", baseName)
	}
	if len(job.MCPServers) != 1 {
		t.Fatalf("expected reserved MCP server for review jobs without reusable agents, got %#v", job.MCPServers)
	}
	if job.MCPServers[0].Stdio == nil || job.MCPServers[0].Stdio.Name != reusableagents.ReservedMCPServerName {
		t.Fatalf("unexpected reserved MCP server wiring: %#v", job.MCPServers)
	}
}

func TestPrepareJobsWithSelectedAgentAppendsCanonicalSystemPrompt(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	runArtifacts := model.NewRunArtifacts(workspaceRoot, "tasks-demo-agent-test-run")
	if err := os.MkdirAll(runArtifacts.JobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}

	tasksDir := t.TempDir()
	groups := map[string][]model.IssueEntry{
		"task_1": {
			{
				Name:     "task_1.md",
				AbsPath:  filepath.Join(tasksDir, "task_1.md"),
				Content:  "---\nstatus: pending\ntitle: Task 1\ntype: backend\ncomplexity: low\n---\n\n# Task 1\n",
				CodeFile: "task_1",
			},
		},
	}

	councilDir := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", "council")
	if err := os.MkdirAll(councilDir, 0o755); err != nil {
		t.Fatalf("mkdir council agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(councilDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Council",
			"description: Coordinates reviewers",
			"ide: claude",
			"model: agent-model",
			"reasoning_effort: high",
			"access_mode: default",
			"---",
			"",
			"You are the council agent.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write council agent: %v", err)
	}

	reviewerDir := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", "reviewer")
	if err := os.MkdirAll(reviewerDir, 0o755); err != nil {
		t.Fatalf("mkdir reviewer agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(reviewerDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Reviewer",
			"description: Reviews code",
			"ide: codex",
			"---",
			"",
			"Review the code.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write reviewer agent: %v", err)
	}

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      tasksDir,
		Mode:          model.ExecutionModePRDTasks,
		AgentName:     "council",
	}

	agentExecution, err := reusableagents.ResolveExecutionContext(context.Background(), cfg)
	if err != nil {
		t.Fatalf("resolve execution context: %v", err)
	}

	jobs, err := prepareJobs(context.Background(), cfg, groups, runArtifacts, nil, agentExecution)
	if err != nil {
		t.Fatalf("prepareJobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one job, got %d", len(jobs))
	}

	systemPrompt := jobs[0].SystemPrompt
	workflowIndex := strings.Index(systemPrompt, "<workflow_memory>")
	metadataIndex := strings.Index(systemPrompt, "<agent_metadata>")
	discoveryIndex := strings.Index(systemPrompt, "<available_agents>")
	bodyIndex := strings.Index(systemPrompt, "You are the council agent.")
	if workflowIndex < 0 || metadataIndex < 0 || discoveryIndex < 0 || bodyIndex < 0 {
		t.Fatalf(
			"expected workflow memory, metadata, discovery, and agent body in system prompt, got:\n%s",
			systemPrompt,
		)
	}
	if workflowIndex >= metadataIndex || metadataIndex >= discoveryIndex || discoveryIndex >= bodyIndex {
		t.Fatalf("expected canonical system prompt order, got:\n%s", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "- reviewer: Reviews code (workspace)") {
		t.Fatalf("expected compact discovery catalog entry, got:\n%s", systemPrompt)
	}
}

func TestPrepareAllowsReviewRoundsWithoutPR(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "review-without-pr", "reviews-007")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "",
		Round:     7,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	cfg := &model.RuntimeConfig{
		ReviewsDir:    reviewDir,
		WorkspaceRoot: workspaceRoot,
		IDE:           model.IDECodex,
		DryRun:        true,
		Mode:          model.ExecutionModePRReview,
	}

	scope := openRunScopeForTest(t, cfg)
	prep, err := Prepare(context.Background(), cfg, scope)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)
	if prep.ResolvedName != "review-without-pr" {
		t.Fatalf("unexpected resolved name: %q", prep.ResolvedName)
	}
	if prep.ResolvedProvider != "coderabbit" {
		t.Fatalf("unexpected resolved provider: %q", prep.ResolvedProvider)
	}
	if prep.ResolvedPR != "" {
		t.Fatalf("expected empty resolved pr, got %q", prep.ResolvedPR)
	}
	if prep.ResolvedRound != 7 {
		t.Fatalf("unexpected resolved round: %d", prep.ResolvedRound)
	}
	if len(prep.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(prep.Jobs))
	}
	if cfg.PR != "" {
		t.Fatalf("expected runtime config pr to remain empty, got %q", cfg.PR)
	}
	if cfg.Name != "review-without-pr" {
		t.Fatalf("unexpected runtime config name: %q", cfg.Name)
	}
	if cfg.Round != 7 {
		t.Fatalf("unexpected runtime config round: %d", cfg.Round)
	}
}

func TestPreparePRDTasksUsesSharedRunArtifactsWithoutChangingTaskOrder(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}

	files := map[string]string{
		"task_10.md": "---\nstatus: pending\ntitle: Task 10\ntype: backend\ncomplexity: low\n---\n\n# Task 10\n",
		"task_2.md":  "---\nstatus: pending\ntitle: Task 2\ntype: backend\ncomplexity: low\n---\n\n# Task 2\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tasksDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
	}
	scope := openRunScopeForTest(t, cfg)
	prep, err := Prepare(context.Background(), cfg, scope)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)
	if len(prep.Jobs) != 2 {
		t.Fatalf("expected two prepared jobs, got %d", len(prep.Jobs))
	}

	if got, want := prep.Jobs[0].CodeFiles, []string{"task_2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected first job order\nwant: %#v\ngot:  %#v", want, got)
	}
	if got, want := prep.Jobs[1].CodeFiles, []string{"task_10"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected second job order\nwant: %#v\ngot:  %#v", want, got)
	}

	runArtifacts := prep.RunArtifacts
	expectedRunArtifacts, err := model.ResolveHomeRunArtifacts(runArtifacts.RunID)
	if err != nil {
		t.Fatalf("resolve home run artifacts: %v", err)
	}
	if got, want := runArtifacts.RunDir, expectedRunArtifacts.RunDir; got != want {
		t.Fatalf("expected home-scoped run dir %q, got %q", want, got)
	}
	for _, job := range prep.Jobs {
		assertJobUsesRunArtifacts(t, runArtifacts, job)
	}
}

func TestPrepareWithNilManagerPreservesWorkflowPreparationBehavior(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	taskFile := filepath.Join(tasksDir, "task_01.md")
	if err := os.WriteFile(taskFile, []byte(`---
status: pending
title: Demo task
type: backend
complexity: low
---

# Task 01: Demo task
`), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      tasksDir,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
	}
	scope := openRunScopeForTest(t, cfg)
	prep, err := Prepare(context.Background(), cfg, scope)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	if prep.RuntimeManager() != nil {
		t.Fatalf("expected nil runtime manager when executable extensions are disabled, got %T", prep.RuntimeManager())
	}
	if len(prep.Jobs) != 1 {
		t.Fatalf("expected one prepared job, got %d", len(prep.Jobs))
	}
	assertJobUsesRunArtifacts(t, prep.RunArtifacts, prep.Jobs[0])
	if got, want := prep.Journal().Path(), prep.RunArtifacts.EventsPath; got != want {
		t.Fatalf("journal path = %q, want %q", got, want)
	}
}

func TestPrepareNoOpManagerMatchesNilManagerAndDispatchesPlanHooks(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task_01.md"), []byte(`---
status: pending
title: Demo task
type: backend
complexity: low
---

# Task 01: Demo task
`), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	nilCfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      tasksDir,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
		RunID:         "plan-nil-manager",
	}
	nilScope := openRunScopeForTest(t, nilCfg)
	nilPrep, err := Prepare(context.Background(), nilCfg, nilScope)
	if err != nil {
		t.Fatalf("prepare with nil manager: %v", err)
	}
	defer closePreparedJournalForTest(t, nilPrep)

	noopManager := &planHookManager{}
	noopCfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      tasksDir,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
		RunID:         "plan-noop-manager",
	}
	noopPrep, err := Prepare(
		context.Background(),
		noopCfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, noopCfg), manager: noopManager},
	)
	if err != nil {
		t.Fatalf("prepare with no-op manager: %v", err)
	}
	defer closePreparedJournalForTest(t, noopPrep)

	if got, want := snapshotPreparationForTest(
		noopPrep,
	), snapshotPreparationForTest(
		nilPrep,
	); !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("no-op manager changed prepare output\nwant: %#v\ngot:  %#v", want, got)
	}

	wantHooks := []string{
		"plan.pre_discover",
		"plan.post_discover",
		"plan.pre_group",
		"plan.post_group",
		"plan.pre_prepare_jobs",
		"plan.pre_resolve_task_runtime",
		"plan.post_prepare_jobs",
	}
	if got := filterHooksByPrefix(noopManager.mutableHooks, "plan."); !reflect.DeepEqual(got, wantHooks) {
		t.Fatalf("unexpected plan hook order\nwant: %#v\ngot:  %#v", wantHooks, got)
	}
}

func TestPreparePlanPreDiscoverExtraSourcesMergesAndDedupesEntries(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	extraDir := filepath.Join(workspaceRoot, "extra-tasks")
	for _, dir := range []string{tasksDir, extraDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	writeTask := func(dir, name, title string) {
		t.Helper()

		content := fmt.Sprintf(`---
status: pending
title: %s
type: backend
complexity: low
---

# %s
`, title, title)
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s/%s: %v", dir, name, err)
		}
	}
	writeTask(tasksDir, "task_01.md", "Task 01")
	writeTask(extraDir, "task_02.md", "Task 02")

	manager := &planHookManager{
		mutators: map[string]func(any) (any, error){
			"plan.pre_discover": func(input any) (any, error) {
				payload := input.(planPreDiscoverPayload)
				payload.ExtraSources = []string{
					filepath.Join("extra-tasks", "task_02.md"),
					filepath.Join(model.TasksBaseDir(), "demo", "task_01.md"),
				}
				return payload, nil
			},
		},
	}

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      tasksDir,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
		RunID:         "plan-extra-sources",
	}
	prep, err := Prepare(
		context.Background(),
		cfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, cfg), manager: manager},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	if len(prep.Jobs) != 2 {
		t.Fatalf("expected merged and deduped entries to produce 2 jobs, got %d", len(prep.Jobs))
	}

	gotFiles := []string{prep.Jobs[0].CodeFiles[0], prep.Jobs[1].CodeFiles[0]}
	wantFiles := []string{"task_01", "task_02"}
	if !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Fatalf("unexpected merged task order\nwant: %#v\ngot:  %#v", wantFiles, gotFiles)
	}
}

func TestPreparePlanPostDiscoverMutationUpdatesPromptArtifacts(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task_01.md"), []byte(`---
status: pending
title: Demo task
type: backend
complexity: low
---

# Task 01: Demo task
`), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	const marker = "\nPOST-DISCOVER-MARKER\n"
	manager := &planHookManager{
		mutators: map[string]func(any) (any, error){
			"plan.post_discover": func(input any) (any, error) {
				payload := input.(planPostDiscoverPayload)
				payload.Entries[0].Content += marker
				return payload, nil
			},
		},
	}

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      tasksDir,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
		RunID:         "plan-post-discover",
	}
	prep, err := Prepare(
		context.Background(),
		cfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, cfg), manager: manager},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	if got := string(prep.Jobs[0].Prompt); !strings.Contains(got, marker) {
		t.Fatalf("expected prompt to include post-discover marker, got:\n%s", got)
	}
	promptArtifact, err := os.ReadFile(prep.Jobs[0].OutPromptPath)
	if err != nil {
		t.Fatalf("read prompt artifact: %v", err)
	}
	if got := string(promptArtifact); !strings.Contains(got, marker) {
		t.Fatalf("expected prompt artifact to include post-discover marker, got:\n%s", got)
	}
}

func TestPreparePlanPostGroupMutationChangesPrepareJobsInput(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo", "reviews-007")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     7,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	manager := &planHookManager{
		mutators: map[string]func(any) (any, error){
			"plan.post_group": func(input any) (any, error) {
				payload := input.(planGroupsPayload)
				payload.Groups["internal/app/service.go"] = append(
					payload.Groups["internal/app/service.go"],
					model.IssueEntry{
						Name:     "issue_999.md",
						AbsPath:  filepath.Join(reviewDir, "issue_999.md"),
						Content:  "---\nstatus: pending\nfile: internal/app/service.go\n---\n\n# Synthetic issue\n",
						CodeFile: "internal/app/service.go",
					},
				)
				return payload, nil
			},
		},
	}

	cfg := &model.RuntimeConfig{
		ReviewsDir:    reviewDir,
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		BatchSize:     10,
		Mode:          model.ExecutionModePRReview,
		RunID:         "plan-post-group",
	}
	prep, err := Prepare(
		context.Background(),
		cfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, cfg), manager: manager},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	if len(prep.Jobs) != 1 {
		t.Fatalf("expected one review job, got %d", len(prep.Jobs))
	}
	if got := prep.Jobs[0].IssueCount(); got != 2 {
		t.Fatalf("expected post-group mutation to reach prepareJobs, got %d issues", got)
	}
}

func TestPreparePlanPreGroupMutationReassignsGroupingKey(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo", "reviews-007")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     7,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	manager := &planHookManager{
		mutators: map[string]func(any) (any, error){
			"plan.pre_group": func(input any) (any, error) {
				payload := input.(planEntriesPayload)
				payload.Entries[0].CodeFile = "internal/app/rewritten.go"
				return payload, nil
			},
		},
	}

	cfg := &model.RuntimeConfig{
		ReviewsDir:    reviewDir,
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		BatchSize:     10,
		Mode:          model.ExecutionModePRReview,
		RunID:         "plan-pre-group",
	}
	prep, err := Prepare(
		context.Background(),
		cfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, cfg), manager: manager},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	if len(prep.Jobs) != 1 {
		t.Fatalf("expected one review job, got %d", len(prep.Jobs))
	}
	if got, want := prep.Jobs[0].CodeFiles, []string{"internal/app/rewritten.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected regrouped code files\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestPreparePlanPostPrepareJobsMutationReplacesPreparedJobs(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task_01.md"), []byte(`---
status: pending
title: Demo task
type: backend
complexity: low
---

# Task 01: Demo task
`), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	const marker = "POST-PREPARE-JOBS"
	manager := &planHookManager{
		mutators: map[string]func(any) (any, error){
			"plan.post_prepare_jobs": func(input any) (any, error) {
				payload := input.(planJobsPayload)
				payload.Jobs[0].SystemPrompt = marker
				return payload, nil
			},
		},
	}

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      tasksDir,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
		RunID:         "plan-post-prepare-jobs",
	}
	prep, err := Prepare(
		context.Background(),
		cfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, cfg), manager: manager},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	if got := prep.Jobs[0].SystemPrompt; got != marker {
		t.Fatalf("expected post-prepare-jobs mutation to replace system prompt, got %q", got)
	}
}

func TestPreparePlanPreResolveTaskRuntimeMutationUpdatesPreparedJob(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task_01.md"), []byte(`---
status: pending
title: Demo task
type: frontend
complexity: low
---

# Task 01: Demo task
`), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	manager := &planHookManager{
		mutators: map[string]func(any) (any, error){
			"plan.pre_resolve_task_runtime": func(input any) (any, error) {
				payload := input.(planPreResolveTaskRuntimePayload)
				if payload.Task.ID != "task_01" {
					t.Fatalf("unexpected task id: %q", payload.Task.ID)
				}
				if !strings.HasPrefix(payload.Task.SafeName, "task_01") {
					t.Fatalf("unexpected task safe name: %q", payload.Task.SafeName)
				}
				payload.Runtime = model.TaskRuntime{
					IDE:             model.IDECodex,
					Model:           "gpt-5.5",
					ReasoningEffort: "xhigh",
				}
				return payload, nil
			},
		},
	}

	cfg := &model.RuntimeConfig{
		Name:            "demo",
		WorkspaceRoot:   workspaceRoot,
		TasksDir:        tasksDir,
		DryRun:          true,
		IDE:             model.IDECodex,
		Model:           "codex-fast",
		ReasoningEffort: "medium",
		Mode:            model.ExecutionModePRDTasks,
		RunID:           "plan-pre-resolve-task-runtime",
	}
	prep, err := Prepare(
		context.Background(),
		cfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, cfg), manager: manager},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	if got := prep.Jobs[0].Model; got != "gpt-5.5" {
		t.Fatalf("prepared job model = %q, want %q", got, "gpt-5.5")
	}
	if got := prep.Jobs[0].ReasoningEffort; got != "xhigh" {
		t.Fatalf("prepared job reasoning = %q, want %q", got, "xhigh")
	}
}

func TestPreparePlanPostPrepareJobsRejectsRuntimeMutation(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "task_01.md"), []byte(`---
status: pending
title: Demo task
type: backend
complexity: low
---

# Task 01: Demo task
`), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	manager := &planHookManager{
		mutators: map[string]func(any) (any, error){
			"plan.post_prepare_jobs": func(input any) (any, error) {
				payload := input.(planJobsPayload)
				payload.Jobs[0].Model = "codex-fast"
				return payload, nil
			},
		},
	}

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      tasksDir,
		DryRun:        true,
		IDE:           model.IDECodex,
		Model:         "gpt-5.5",
		Mode:          model.ExecutionModePRDTasks,
		RunID:         "plan-post-prepare-jobs-runtime-mutation",
	}
	_, err := Prepare(
		context.Background(),
		cfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, cfg), manager: manager},
	)
	if err == nil {
		t.Fatal("prepare error = nil, want runtime mutation failure")
	}
	if !strings.Contains(
		err.Error(),
		"plan.post_prepare_jobs cannot mutate job runtime after task runtime resolution",
	) {
		t.Fatalf("prepare error = %q, want runtime mutation guard", err.Error())
	}
}

func TestPrepareReviewModeUsesSharedRunArtifactsWithoutChangingFilterBehavior(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo", "reviews-007")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     7,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
		},
		{
			Title:       "Trim whitespace",
			File:        "internal/app/service.go",
			Line:        54,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_2,comment:RC_2",
			Body:        "Trim the incoming value before using it.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	resolvedIssuePath := filepath.Join(reviewDir, "issue_002.md")
	resolvedContent, err := os.ReadFile(resolvedIssuePath)
	if err != nil {
		t.Fatalf("read issue_002: %v", err)
	}
	resolvedContent = []byte(strings.Replace(string(resolvedContent), "status: pending", "status: resolved", 1))
	if err := os.WriteFile(resolvedIssuePath, resolvedContent, 0o600); err != nil {
		t.Fatalf("mark issue_002 resolved: %v", err)
	}
	if _, err := reviews.RefreshRoundMeta(reviewDir); err != nil {
		t.Fatalf("refresh round meta: %v", err)
	}

	cfg := &model.RuntimeConfig{
		ReviewsDir:      reviewDir,
		WorkspaceRoot:   workspaceRoot,
		DryRun:          true,
		IDE:             model.IDECodex,
		BatchSize:       10,
		Mode:            model.ExecutionModePRReview,
		IncludeResolved: false,
	}
	scope := openRunScopeForTest(t, cfg)
	prep, err := Prepare(context.Background(), cfg, scope)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)
	if len(prep.Jobs) != 1 {
		t.Fatalf("expected one prepared review job, got %d", len(prep.Jobs))
	}
	if got := prep.Jobs[0].IssueCount(); got != 1 {
		t.Fatalf("expected only unresolved review issue to remain, got %d", got)
	}

	runArtifacts := prep.RunArtifacts
	expectedRunArtifacts, err := model.ResolveHomeRunArtifacts(runArtifacts.RunID)
	if err != nil {
		t.Fatalf("resolve home run artifacts: %v", err)
	}
	if got, want := runArtifacts.RunDir, expectedRunArtifacts.RunDir; got != want {
		t.Fatalf("expected home-scoped run dir %q, got %q", want, got)
	}
	assertJobUsesRunArtifacts(t, runArtifacts, prep.Jobs[0])
}

func TestPrepareReviewPreFetchCanChangeIncludeResolved(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo", "reviews-007")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     7,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
		},
		{
			Title:       "Trim whitespace",
			File:        "internal/app/service.go",
			Line:        54,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_2,comment:RC_2",
			Body:        "Trim the incoming value before using it.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	resolvedIssuePath := filepath.Join(reviewDir, "issue_002.md")
	resolvedContent, err := os.ReadFile(resolvedIssuePath)
	if err != nil {
		t.Fatalf("read issue_002: %v", err)
	}
	resolvedContent = []byte(strings.Replace(string(resolvedContent), "status: pending", "status: resolved", 1))
	if err := os.WriteFile(resolvedIssuePath, resolvedContent, 0o600); err != nil {
		t.Fatalf("mark issue_002 resolved: %v", err)
	}
	if _, err := reviews.RefreshRoundMeta(reviewDir); err != nil {
		t.Fatalf("refresh round meta: %v", err)
	}

	manager := &planHookManager{
		mutators: map[string]func(any) (any, error){
			"review.pre_fetch": func(input any) (any, error) {
				payload := input.(reviewPreFetchPayload)
				payload.FetchConfig.IncludeResolved = true
				return payload, nil
			},
		},
	}
	cfg := &model.RuntimeConfig{
		ReviewsDir:      reviewDir,
		WorkspaceRoot:   workspaceRoot,
		DryRun:          true,
		IDE:             model.IDECodex,
		BatchSize:       10,
		Mode:            model.ExecutionModePRReview,
		IncludeResolved: false,
		RunID:           "review-pre-fetch",
	}
	prep, err := Prepare(
		context.Background(),
		cfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, cfg), manager: manager},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	if len(prep.Jobs) != 1 {
		t.Fatalf("expected one prepared review job, got %d", len(prep.Jobs))
	}
	if got := prep.Jobs[0].IssueCount(); got != 2 {
		t.Fatalf("expected review.pre_fetch to include resolved issues, got %d", got)
	}
}

func TestPrepareReviewHooksCanMutateFetchedIssuesAndBatchGroups(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo", "reviews-007")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     7,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	manager := &planHookManager{
		mutators: map[string]func(any) (any, error){
			"review.post_fetch": func(input any) (any, error) {
				payload := input.(reviewPostFetchPayload)
				payload.Issues = append(payload.Issues, model.IssueEntry{
					Name:     "issue_999.md",
					AbsPath:  filepath.Join(reviewDir, "issue_999.md"),
					Content:  "---\nstatus: pending\nfile: internal/app/service.go\n---\n\n# Synthetic issue\n",
					CodeFile: "internal/app/service.go",
				})
				return payload, nil
			},
			"review.pre_batch": func(input any) (any, error) {
				payload := input.(reviewPreBatchPayload)
				payload.Groups["internal/app/rewritten.go"] = payload.Groups["internal/app/service.go"]
				delete(payload.Groups, "internal/app/service.go")
				return payload, nil
			},
		},
	}
	cfg := &model.RuntimeConfig{
		ReviewsDir:    reviewDir,
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		BatchSize:     10,
		Mode:          model.ExecutionModePRReview,
		RunID:         "review-post-fetch-pre-batch",
	}
	prep, err := Prepare(
		context.Background(),
		cfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, cfg), manager: manager},
	)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	if len(prep.Jobs) != 1 {
		t.Fatalf("expected one review job, got %d", len(prep.Jobs))
	}
	if got := prep.Jobs[0].IssueCount(); got != 2 {
		t.Fatalf("expected mutated review issue list to reach batching, got %d issues", got)
	}
	if got, want := prep.Jobs[0].CodeFiles, []string{"internal/app/rewritten.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected review.pre_batch code files\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestPrepareReviewNilManagerMatchesNoopManagerSnapshot(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	reviewDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo", "reviews-007")
	if err := reviews.WriteRound(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     7,
		CreatedAt: time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
	}, []provider.ReviewItem{
		{
			Title:       "Add nil check",
			File:        "internal/app/service.go",
			Line:        42,
			Author:      "coderabbitai[bot]",
			ProviderRef: "thread:PRT_1,comment:RC_1",
			Body:        "Please add a nil check before dereferencing the pointer.",
		},
	}); err != nil {
		t.Fatalf("write round: %v", err)
	}

	nilCfg := &model.RuntimeConfig{
		ReviewsDir:    reviewDir,
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		BatchSize:     10,
		Mode:          model.ExecutionModePRReview,
		RunID:         "review-nil-manager",
	}
	nilPrep, err := Prepare(context.Background(), nilCfg, openRunScopeForTest(t, nilCfg))
	if err != nil {
		t.Fatalf("prepare nil manager: %v", err)
	}
	defer closePreparedJournalForTest(t, nilPrep)

	noopCfg := &model.RuntimeConfig{
		ReviewsDir:    reviewDir,
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		BatchSize:     10,
		Mode:          model.ExecutionModePRReview,
		RunID:         "review-nil-manager",
	}
	noopPrep, err := Prepare(
		context.Background(),
		noopCfg,
		&planRunScopeWithManager{RunScope: openRunScopeForTest(t, noopCfg), manager: &planHookManager{}},
	)
	if err != nil {
		t.Fatalf("prepare noop manager: %v", err)
	}
	defer closePreparedJournalForTest(t, noopPrep)

	if got, want := snapshotPreparationForTest(
		nilPrep,
	), snapshotPreparationForTest(
		noopPrep,
	); !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("nil manager snapshot mismatch\nnil:  %#v\nnoop: %#v", got, want)
	}
}

func TestPrepareExecModeBuildsSinglePromptBackedJobWithRunMetadata(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	promptPath := filepath.Join(workspaceRoot, "prompt.md")
	if err := os.WriteFile(promptPath, []byte("Summarize the repository state\n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	cfg := &model.RuntimeConfig{
		WorkspaceRoot: workspaceRoot,
		PromptFile:    promptPath,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModeExec,
		OutputFormat:  model.OutputFormatJSON,
	}
	scope := openRunScopeForTest(t, cfg)
	prep, err := Prepare(context.Background(), cfg, scope)
	if err != nil {
		t.Fatalf("prepare exec: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)
	if len(prep.Jobs) != 1 {
		t.Fatalf("expected one exec job, got %d", len(prep.Jobs))
	}
	if prep.Journal() == nil {
		t.Fatal("expected prepare to return a run journal")
	}
	if got := prep.Journal().Path(); got != prep.RunArtifacts.EventsPath {
		t.Fatalf("expected journal path %q, got %q", prep.RunArtifacts.EventsPath, got)
	}

	job := prep.Jobs[0]
	if got, want := job.CodeFiles, []string{"exec"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected exec code files\nwant: %#v\ngot:  %#v", want, got)
	}
	if got := string(job.Prompt); got != "Summarize the repository state\n" {
		t.Fatalf("unexpected exec prompt: %q", got)
	}
	assertJobUsesRunArtifacts(t, prep.RunArtifacts, job)
	for _, path := range []string{
		prep.RunArtifacts.RunMetaPath,
		job.OutPromptPath,
		job.OutLog,
		job.ErrLog,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected exec artifact %s: %v", path, err)
		}
	}
}

func closePreparedJournalForTest(t *testing.T, prep *model.SolvePreparation) {
	t.Helper()

	if prep == nil || prep.Journal() == nil {
		return
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := prep.CloseJournal(closeCtx); err != nil {
		t.Fatalf("close prepared journal: %v", err)
	}
}

func TestResolveInputsRejectsLegacyTasksDirInference(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	legacyTasksDir := filepath.Join(tmp, "tasks", "legacy")
	if err := os.MkdirAll(legacyTasksDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy tasks dir: %v", err)
	}

	_, _, _, err := resolveInputs(&model.RuntimeConfig{
		TasksDir: legacyTasksDir,
		Mode:     model.ExecutionModePRDTasks,
	})
	if err == nil {
		t.Fatal("expected legacy tasks dir inference to fail")
	}
	if !strings.Contains(err.Error(), filepath.ToSlash(model.TasksBaseDir())+"/<name>") {
		t.Fatalf("expected error to mention canonical tasks dir, got %v", err)
	}
}

func TestResolveInputsRejectsLegacyReviewsDirInference(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	legacyReviewsDir := filepath.Join(tmp, "tasks", "legacy", "reviews-001")
	if err := os.MkdirAll(legacyReviewsDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy reviews dir: %v", err)
	}

	_, _, _, err := resolveInputs(&model.RuntimeConfig{
		ReviewsDir: legacyReviewsDir,
		Mode:       model.ExecutionModePRReview,
	})
	if err == nil {
		t.Fatal("expected legacy reviews dir inference to fail")
	}
	if !strings.Contains(err.Error(), filepath.ToSlash(model.TasksBaseDir())+"/<name>/reviews-NNN") {
		t.Fatalf("expected error to mention canonical reviews dir, got %v", err)
	}
}

func captureSlogOutput(t *testing.T, fn func()) string {
	t.Helper()

	originalLogger := slog.Default()
	var buffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buffer, nil))
	slog.SetDefault(logger)
	defer slog.SetDefault(originalLogger)

	fn()

	return buffer.String()
}

func assertJobUsesRunArtifacts(t *testing.T, runArtifacts model.RunArtifacts, job model.Job) {
	t.Helper()

	if got, want := filepath.Dir(job.OutPromptPath), runArtifacts.JobsDir; got != want {
		t.Fatalf("unexpected prompt directory\nwant: %q\ngot:  %q", want, got)
	}
	if got, want := filepath.Dir(job.OutLog), runArtifacts.JobsDir; got != want {
		t.Fatalf("unexpected stdout log directory\nwant: %q\ngot:  %q", want, got)
	}
	if got, want := filepath.Dir(job.ErrLog), runArtifacts.JobsDir; got != want {
		t.Fatalf("unexpected stderr log directory\nwant: %q\ngot:  %q", want, got)
	}

	jobArtifacts := runArtifacts.JobArtifacts(job.SafeName)
	if got, want := job.OutPromptPath, jobArtifacts.PromptPath; got != want {
		t.Fatalf("unexpected prompt path\nwant: %q\ngot:  %q", want, got)
	}
	if got, want := job.OutLog, jobArtifacts.OutLogPath; got != want {
		t.Fatalf("unexpected stdout log path\nwant: %q\ngot:  %q", want, got)
	}
	if got, want := job.ErrLog, jobArtifacts.ErrLogPath; got != want {
		t.Fatalf("unexpected stderr log path\nwant: %q\ngot:  %q", want, got)
	}
}

func testStringPointer(value string) *string {
	return &value
}

func openRunScopeForTest(t *testing.T, cfg *model.RuntimeConfig) model.RunScope {
	t.Helper()

	if cfg != nil && strings.TrimSpace(cfg.RunID) == "" {
		cfgCopy := *cfg
		cfgCopy.RunID = strings.NewReplacer("/", "-", " ", "-").Replace(strings.TrimSpace(t.Name()))
		cfg = &cfgCopy
	}

	scope, err := model.OpenRunScope(context.Background(), cfg, model.OpenRunScopeOptions{})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}
	return scope
}

type planHookManager struct {
	mutators     map[string]func(any) (any, error)
	mutableHooks []string
}

func (*planHookManager) Start(context.Context) error { return nil }

func (*planHookManager) Shutdown(context.Context) error { return nil }

func (m *planHookManager) DispatchMutableHook(
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

func (*planHookManager) DispatchObserverHook(context.Context, string, any) {}

type planRunScopeWithManager struct {
	model.RunScope
	manager model.RuntimeManager
}

func (s *planRunScopeWithManager) RunManager() model.RuntimeManager {
	if s == nil {
		return nil
	}
	return s.manager
}

type prepareSnapshot struct {
	ResolvedName     string
	ResolvedProvider string
	ResolvedPR       string
	ResolvedRound    int
	Jobs             []prepareJobSnapshot
}

type prepareJobSnapshot struct {
	Prompt       string
	SystemPrompt string
	CodeFiles    []string
	TaskTitle    string
	TaskType     string
	IssueNames   []string
}

func snapshotPreparationForTest(prep *model.SolvePreparation) prepareSnapshot {
	if prep == nil {
		return prepareSnapshot{}
	}

	snapshot := prepareSnapshot{
		ResolvedName:     prep.ResolvedName,
		ResolvedProvider: prep.ResolvedProvider,
		ResolvedPR:       prep.ResolvedPR,
		ResolvedRound:    prep.ResolvedRound,
		Jobs:             make([]prepareJobSnapshot, 0, len(prep.Jobs)),
	}

	for i := range prep.Jobs {
		job := prep.Jobs[i]
		issueNames := make([]string, 0)
		for _, codeFile := range sortedBatchGroupKeys(job.Groups) {
			for _, entry := range job.Groups[codeFile] {
				issueNames = append(issueNames, entry.Name)
			}
		}
		snapshot.Jobs = append(snapshot.Jobs, prepareJobSnapshot{
			Prompt:       string(job.Prompt),
			SystemPrompt: job.SystemPrompt,
			CodeFiles:    append([]string(nil), job.CodeFiles...),
			TaskTitle:    job.TaskTitle,
			TaskType:     job.TaskType,
			IssueNames:   issueNames,
		})
	}

	return snapshot
}

func sortedBatchGroupKeys(groups map[string][]model.IssueEntry) []string {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func filterHooksByPrefix(hooks []string, prefix string) []string {
	filtered := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		if strings.HasPrefix(hook, prefix) {
			filtered = append(filtered, hook)
		}
	}
	return filtered
}

func writeRecursiveTaskFixture(t *testing.T, tasksDir string) {
	t.Helper()
	body := "---\nstatus: pending\ntitle: %s\ntype: backend\ncomplexity: low\n---\n\n# %s\n"
	files := map[string]string{
		filepath.Join(tasksDir, "task_01.md"):                        "root",
		filepath.Join(tasksDir, "features", "auth", "task_01.md"):    "auth",
		filepath.Join(tasksDir, "features", "payment", "task_01.md"): "payment",
	}
	for path, label := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		content := fmt.Sprintf(body, label, label)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

func TestReadIssueEntriesBranchesOnRecursive(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeRecursiveTaskFixture(t, tasksDir)

	cases := []struct {
		name      string
		recursive bool
		wantNames []string
	}{
		{
			name:      "flat ignores nested directories",
			recursive: false,
			wantNames: []string{"task_01.md"},
		},
		{
			name:      "recursive discovers nested directories",
			recursive: true,
			wantNames: []string{
				"task_01.md",
				"features/auth/task_01.md",
				"features/payment/task_01.md",
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			entries, err := readIssueEntries(tasksDir, model.ExecutionModePRDTasks, false, tc.recursive)
			if err != nil {
				t.Fatalf("readIssueEntries: %v", err)
			}
			gotNames := make([]string, 0, len(entries))
			for _, entry := range entries {
				gotNames = append(gotNames, entry.Name)
			}
			if !reflect.DeepEqual(gotNames, tc.wantNames) {
				t.Fatalf("unexpected entry order\nwant: %#v\ngot:  %#v", tc.wantNames, gotNames)
			}
		})
	}
}

func resolvePreparedEntriesForTest(
	t *testing.T,
	cfg *model.RuntimeConfig,
) ([]model.IssueEntry, *model.SolvePreparation) {
	t.Helper()
	scope := openRunScopeForTest(t, cfg)
	prep := &model.SolvePreparation{}
	prep.RunArtifacts = scope.RunArtifacts()
	prep.SetRunScope(scope)
	entries, err := resolvePreparedEntries(context.Background(), prep, cfg)
	if err != nil {
		closePreparedJournalForTest(t, prep)
		t.Fatalf("resolvePreparedEntries: %v", err)
	}
	t.Cleanup(func() { closePreparedJournalForTest(t, prep) })
	return entries, prep
}

func TestResolvePreparedEntriesDiscoversNestedTasksWhenRecursive(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	writeRecursiveTaskFixture(t, tasksDir)

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
		Recursive:     true,
	}
	entries, _ := resolvePreparedEntriesForTest(t, cfg)

	wantNames := []string{
		"task_01.md",
		"features/auth/task_01.md",
		"features/payment/task_01.md",
	}
	gotNames := make([]string, 0, len(entries))
	codeFiles := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		gotNames = append(gotNames, entry.Name)
		codeFiles[entry.CodeFile] = struct{}{}
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected entry order\nwant: %#v\ngot:  %#v", wantNames, gotNames)
	}
	if len(codeFiles) != len(entries) {
		t.Fatalf("expected unique CodeFile per nested task, got %d unique for %d entries", len(codeFiles), len(entries))
	}
}

func TestPreparePRDTasksRecursivePreservesDirectoryGroupedOrder(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	writeRecursiveTaskFixture(t, tasksDir)

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
		Recursive:     true,
	}
	scope := openRunScopeForTest(t, cfg)
	prep, err := Prepare(context.Background(), cfg, scope)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	defer closePreparedJournalForTest(t, prep)

	gotCodeFiles := make([][]string, 0, len(prep.Jobs))
	for _, job := range prep.Jobs {
		gotCodeFiles = append(gotCodeFiles, job.CodeFiles)
	}
	wantCodeFiles := [][]string{
		{"task_01"},
		{"features/auth/task_01"},
		{"features/payment/task_01"},
	}
	if !reflect.DeepEqual(gotCodeFiles, wantCodeFiles) {
		t.Fatalf("unexpected recursive job order\nwant: %#v\ngot:  %#v", wantCodeFiles, gotCodeFiles)
	}
}

func TestResolvePreparedEntriesIgnoresSubdirsWhenFlat(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	writeRecursiveTaskFixture(t, tasksDir)

	cfg := &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
		Recursive:     false,
	}
	entries, _ := resolvePreparedEntriesForTest(t, cfg)

	if len(entries) != 1 {
		t.Fatalf("expected only the root task in flat mode, got %d entries", len(entries))
	}
	if got, want := entries[0].Name, "task_01.md"; got != want {
		t.Fatalf("unexpected entry name\nwant: %q\ngot:  %q", want, got)
	}
}
