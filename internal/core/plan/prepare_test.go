package plan

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
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

	entries, err := readTaskEntries(dir, false)
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

	entries, err := readTaskEntries(dir, false)
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

	entries, err := readTaskEntries(dir, false)
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

	_, err := readTaskEntries(dir, false)
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

	jobs, err := prepareJobs(&model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		TasksDir:      issuesDir,
		BatchSize:     5,
		Mode:          model.ExecutionModePRDTasks,
	}, groups, runArtifacts, nil)
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

func TestBuildBatchJobWrapsMemoryPreparationErrorWithTaskPath(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "tasks-demo-test-run")
	tasksDirFile := filepath.Join(t.TempDir(), "tasks.md")
	if err := os.WriteFile(tasksDirFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write tasks dir sentinel file: %v", err)
	}

	issuePath := filepath.Join(t.TempDir(), "task_01.md")
	_, err := buildBatchJob(
		&model.RuntimeConfig{
			Name:     "demo",
			TasksDir: tasksDirFile,
			Mode:     model.ExecutionModePRDTasks,
		},
		runArtifacts,
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

	jobs, err := prepareJobs(&model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		Round:         7,
		BatchSize:     3,
		Mode:          model.ExecutionModePRReview,
	}, groups, runArtifacts, nil)
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

	jobs, err := prepareJobs(cfg, groups, runArtifacts, agentExecution)
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

	metaPath := reviews.MetaPath(reviewDir)
	metaContent, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	withoutPR := strings.Replace(string(metaContent), "pr: \n", "", 1)
	if err := os.WriteFile(metaPath, []byte(withoutPR), 0o600); err != nil {
		t.Fatalf("rewrite meta without pr: %v", err)
	}

	cfg := &model.RuntimeConfig{
		ReviewsDir:    reviewDir,
		WorkspaceRoot: workspaceRoot,
		IDE:           model.IDECodex,
		DryRun:        true,
		Mode:          model.ExecutionModePRReview,
	}

	prep, err := Prepare(context.Background(), cfg, nil)
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

	prep, err := Prepare(context.Background(), &model.RuntimeConfig{
		Name:          "demo",
		WorkspaceRoot: workspaceRoot,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModePRDTasks,
	}, nil)
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
	if !strings.HasPrefix(
		runArtifacts.RunDir,
		filepath.Join(workspaceRoot, ".compozy", "runs")+string(filepath.Separator),
	) {
		t.Fatalf("expected run dir under workspace runs root, got %q", runArtifacts.RunDir)
	}
	for _, job := range prep.Jobs {
		assertJobUsesRunArtifacts(t, runArtifacts, job)
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

	prep, err := Prepare(context.Background(), &model.RuntimeConfig{
		ReviewsDir:      reviewDir,
		WorkspaceRoot:   workspaceRoot,
		DryRun:          true,
		IDE:             model.IDECodex,
		BatchSize:       10,
		Mode:            model.ExecutionModePRReview,
		IncludeResolved: false,
	}, nil)
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
	if !strings.HasPrefix(
		runArtifacts.RunDir,
		filepath.Join(workspaceRoot, ".compozy", "runs")+string(filepath.Separator),
	) {
		t.Fatalf("expected run dir under workspace runs root, got %q", runArtifacts.RunDir)
	}
	assertJobUsesRunArtifacts(t, runArtifacts, prep.Jobs[0])
}

func TestPrepareExecModeBuildsSinglePromptBackedJobWithRunMetadata(t *testing.T) {
	t.Parallel()

	workspaceRoot := t.TempDir()
	promptPath := filepath.Join(workspaceRoot, "prompt.md")
	if err := os.WriteFile(promptPath, []byte("Summarize the repository state\n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	prep, err := Prepare(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot: workspaceRoot,
		PromptFile:    promptPath,
		DryRun:        true,
		IDE:           model.IDECodex,
		Mode:          model.ExecutionModeExec,
		OutputFormat:  model.OutputFormatJSON,
	}, nil)
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
