package plan

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
		"task_10.md": "---\nstatus: pending\ndomain: x\ntype: feature\nscope: s\ncomplexity: low\n---\n\n# Task 10\n",
		"task_2.md":  "---\nstatus: pending\ndomain: x\ntype: feature\nscope: s\ncomplexity: low\n---\n\n# Task 2\n",
		"task_3.md":  "---\nstatus: completed\ndomain: x\ntype: feature\nscope: s\ncomplexity: low\n---\n\n# Task 3\n",
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
		"task_1.md": "---\nstatus: completed\ndomain: x\ntype: feature\nscope: s\ncomplexity: low\n---\n\n# Task 1\n",
		"task_2.md": "---\nstatus: done\ndomain: x\ntype: feature\nscope: s\ncomplexity: low\n---\n\n# Task 2\n",
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
	output := captureStandardOutput(t, func() {
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
	output := captureStandardOutput(t, func() {
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

func TestPrepareJobsForPRDTasksForcesSingleBatchWithoutGroupedSummaries(t *testing.T) {
	t.Parallel()

	promptRoot := t.TempDir()
	issuesDir := t.TempDir()
	groups := map[string][]model.IssueEntry{
		"task_1": {
			{
				Name:     "task_1.md",
				AbsPath:  filepath.Join(issuesDir, "task_1.md"),
				Content:  "---\nstatus: pending\ndomain: backend\ntype: feature\nscope: small\ncomplexity: low\n---\n\n# Task 1\n",
				CodeFile: "task_1",
			},
		},
		"task_2": {
			{
				Name:     "task_2.md",
				AbsPath:  filepath.Join(issuesDir, "task_2.md"),
				Content:  "---\nstatus: pending\ndomain: backend\ntype: feature\nscope: small\ncomplexity: low\n---\n\n# Task 2\n",
				CodeFile: "task_2",
			},
		},
	}

	jobs, groupedWritten, err := prepareJobs(&model.RuntimeConfig{
		Name:      "demo",
		TasksDir:  issuesDir,
		BatchSize: 5,
		Grouped:   true,
		Mode:      model.ExecutionModePRDTasks,
	}, groups, promptRoot, issuesDir)
	if err != nil {
		t.Fatalf("prepareJobs: %v", err)
	}
	if groupedWritten {
		t.Fatalf("expected grouped summaries to be disabled in prd mode")
	}
	if len(jobs) != 2 {
		t.Fatalf("expected one batch per task in prd mode, got %d", len(jobs))
	}
	for _, job := range jobs {
		if len(job.CodeFiles) != 1 {
			t.Fatalf("expected single-file jobs in prd mode, got %#v", job.CodeFiles)
		}
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

func TestPrepareAllowsReviewRoundsWithoutPR(t *testing.T) {
	t.Parallel()

	reviewDir := filepath.Join(t.TempDir(), model.TasksBaseDir(), "review-without-pr", "reviews-007")
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
		ReviewsDir: reviewDir,
		IDE:        model.IDECodex,
		DryRun:     true,
		Mode:       model.ExecutionModePRReview,
	}

	prep, err := Prepare(context.Background(), cfg)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
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

func captureStandardOutput(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	os.Stdout = writePipe
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := writePipe.Close(); err != nil {
		t.Fatalf("close write pipe: %v", err)
	}
	output, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("read captured output: %v", err)
	}
	if err := readPipe.Close(); err != nil {
		t.Fatalf("close read pipe: %v", err)
	}

	return string(output)
}
