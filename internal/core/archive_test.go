package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
)

func TestArchiveTaskWorkflowsRootScanArchivesOnlyEligibleWorkflows(t *testing.T) {
	t.Parallel()

	rootDir := filepath.Join(t.TempDir(), ".compozy", "tasks")
	eligibleDir := filepath.Join(rootDir, "alpha")
	ineligibleDir := filepath.Join(rootDir, "beta")
	archivedDir := filepath.Join(rootDir, model.ArchivedWorkflowDirName, "old-run")
	for _, dir := range []string{eligibleDir, ineligibleDir, archivedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	writeArchiveTaskFile(t, eligibleDir, "task_001.md", "completed")
	if _, err := tasks.RefreshTaskMeta(eligibleDir); err != nil {
		t.Fatalf("refresh task meta for eligible workflow: %v", err)
	}

	writeArchiveTaskFile(t, ineligibleDir, "task_001.md", "pending")
	if _, err := tasks.RefreshTaskMeta(ineligibleDir); err != nil {
		t.Fatalf("refresh task meta for ineligible workflow: %v", err)
	}

	result, err := Archive(context.Background(), ArchiveConfig{RootDir: rootDir})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if result.WorkflowsScanned != 2 {
		t.Fatalf("expected 2 active workflows scanned, got %d", result.WorkflowsScanned)
	}
	if result.Archived != 1 || result.Skipped != 1 {
		t.Fatalf("unexpected archive counts: %#v", result)
	}
	if got := result.SkippedReasons[ineligibleDir]; got != "task workflow not fully completed" {
		t.Fatalf("unexpected skip reason for beta: %q", got)
	}
	if _, err := os.Stat(eligibleDir); !os.IsNotExist(err) {
		t.Fatalf("expected eligible workflow to move out of active root, got err=%v", err)
	}
	if _, err := os.Stat(archivedDir); err != nil {
		t.Fatalf("expected pre-existing archived content to remain untouched: %v", err)
	}
	if len(result.ArchivedPaths) != 1 {
		t.Fatalf("expected one archived path, got %#v", result.ArchivedPaths)
	}
	archivedPath := result.ArchivedPaths[0]
	if filepath.Dir(archivedPath) != filepath.Join(rootDir, model.ArchivedWorkflowDirName) {
		t.Fatalf("unexpected archive parent: %s", archivedPath)
	}
	if matched, err := regexp.MatchString(
		`/\d{8}-\d{6}-alpha$`,
		filepath.ToSlash(archivedPath),
	); err != nil ||
		!matched {
		t.Fatalf("unexpected archived path format: %s", archivedPath)
	}
}

func TestArchiveTaskWorkflowsRequiresExistingTaskMeta(t *testing.T) {
	t.Parallel()

	rootDir := filepath.Join(t.TempDir(), ".compozy", "tasks")
	workflowDir := filepath.Join(rootDir, "alpha")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "completed")

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if result.Archived != 0 || result.Skipped != 1 {
		t.Fatalf("unexpected archive counts: %#v", result)
	}
	if got := result.SkippedReasons[workflowDir]; got != "missing task _meta.md" {
		t.Fatalf("unexpected skip reason: %q", got)
	}
	if _, err := os.Stat(tasks.MetaPath(workflowDir)); !os.IsNotExist(err) {
		t.Fatalf("expected archive to avoid bootstrapping task meta, got err=%v", err)
	}
}

func TestArchiveTaskWorkflowsRequiresExistingReviewMeta(t *testing.T) {
	t.Parallel()

	rootDir := filepath.Join(t.TempDir(), ".compozy", "tasks")
	workflowDir := filepath.Join(rootDir, "alpha")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "completed")
	if _, err := tasks.RefreshTaskMeta(workflowDir); err != nil {
		t.Fatalf("refresh task meta: %v", err)
	}
	writeArchiveReviewRound(t, workflowDir, 1, []string{"resolved"}, false)

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if result.Archived != 0 || result.Skipped != 1 {
		t.Fatalf("unexpected archive counts: %#v", result)
	}
	if got := result.SkippedReasons[workflowDir]; got != "missing review _meta.md" {
		t.Fatalf("unexpected skip reason: %q", got)
	}
}

func TestArchiveTaskWorkflowsRequiresFullyResolvedReviewRounds(t *testing.T) {
	t.Parallel()

	rootDir := filepath.Join(t.TempDir(), ".compozy", "tasks")
	workflowDir := filepath.Join(rootDir, "alpha")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow: %v", err)
	}
	writeArchiveTaskFile(t, workflowDir, "task_001.md", "completed")
	if _, err := tasks.RefreshTaskMeta(workflowDir); err != nil {
		t.Fatalf("refresh task meta: %v", err)
	}
	writeArchiveReviewRound(t, workflowDir, 1, []string{"resolved"}, true)
	writeArchiveReviewRound(t, workflowDir, 2, []string{"pending"}, true)

	result, err := Archive(context.Background(), ArchiveConfig{TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if result.Archived != 0 || result.Skipped != 1 {
		t.Fatalf("unexpected archive counts: %#v", result)
	}
	if got := result.SkippedReasons[workflowDir]; got != "review rounds not fully resolved" {
		t.Fatalf("unexpected skip reason: %q", got)
	}
}

func TestArchiveTaskWorkflowsRejectsArchivedTargets(t *testing.T) {
	t.Parallel()

	rootDir := filepath.Join(t.TempDir(), ".compozy", "tasks")
	target := filepath.Join(rootDir, model.ArchivedWorkflowDirName, "already-archived")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir archived target: %v", err)
	}

	_, err := Archive(context.Background(), ArchiveConfig{TasksDir: target})
	if err == nil {
		t.Fatal("expected archive to reject archived targets")
	}
	if !strings.Contains(err.Error(), model.ArchivedWorkflowDirName) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeArchiveTaskFile(t *testing.T, workflowDir, name, status string) {
	t.Helper()

	content := strings.Join([]string{
		"---",
		"status: " + status,
		"title: " + name,
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# " + name,
		"",
	}, "\n")

	if err := os.WriteFile(filepath.Join(workflowDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func writeArchiveReviewRound(t *testing.T, workflowDir string, round int, statuses []string, withMeta bool) {
	t.Helper()

	reviewDir := reviews.ReviewDirectory(workflowDir, round)
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("mkdir review dir: %v", err)
	}

	resolvedCount := 0
	for idx, status := range statuses {
		if status == "resolved" {
			resolvedCount++
		}
		content := strings.Join([]string{
			"---",
			"status: " + status,
			"file: internal/app/service.go",
			"line: 42",
			"severity: medium",
			"author: coderabbitai[bot]",
			"provider_ref: thread:PRT_1,comment:RC_1",
			"---",
			"",
			"Review body",
			"",
		}, "\n")
		name := filepath.Join(reviewDir, "issue_"+formatArchiveIssueNumber(idx+1)+".md")
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatalf("write review issue: %v", err)
		}
	}

	if !withMeta {
		return
	}

	if err := reviews.WriteRoundMeta(reviewDir, model.RoundMeta{
		Provider:   "coderabbit",
		PR:         "259",
		Round:      round,
		CreatedAt:  time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		Total:      len(statuses),
		Resolved:   resolvedCount,
		Unresolved: len(statuses) - resolvedCount,
	}); err != nil {
		t.Fatalf("write review meta: %v", err)
	}
}

func formatArchiveIssueNumber(n int) string {
	return fmt.Sprintf("%03d", n)
}
