package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
)

func TestMigrateConvertsLegacyArtifactsAndIgnoresLegacyGroupedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	workflowDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workflowDir, "task_1.md"), []byte(strings.Join([]string{
		"## status: pending",
		"",
		"<task_context>",
		"  <domain>backend</domain>",
		"  <type>feature</type>",
		"  <scope>small</scope>",
		"  <complexity>low</complexity>",
		"</task_context>",
		"",
		"# Task 1: Demo",
		"",
		"Legacy task body.",
		"",
	}, "\n")), 0o600); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}

	reviewDir := filepath.Join(workflowDir, "reviews-001")
	if err := os.MkdirAll(filepath.Join(reviewDir, "grouped"), 0o755); err != nil {
		t.Fatalf("mkdir review dir: %v", err)
	}
	if err := reviews.WriteRoundMeta(reviewDir, model.RoundMeta{
		Provider:  "coderabbit",
		PR:        "259",
		Round:     1,
		CreatedAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("write round meta: %v", err)
	}
	legacyIssuePath := filepath.Join(reviewDir, "issue_001.md")
	legacyIssue := strings.Join([]string{
		"# Issue 001: Demo issue",
		"",
		"## Status: pending",
		"",
		"<review_context>",
		"  <file>internal/app/service.go</file>",
		"  <line>42</line>",
		"  <severity>high</severity>",
		"  <author>review-bot</author>",
		"  <provider_ref>thread:1</provider_ref>",
		"</review_context>",
		"",
		"## Review Comment",
		"",
		"Legacy review body.",
		"",
		"## Triage",
		"",
		"- Decision: `UNREVIEWED`",
		"- Notes:",
		"",
	}, "\n")
	if err := os.WriteFile(legacyIssuePath, []byte(legacyIssue), 0o600); err != nil {
		t.Fatalf("write legacy issue: %v", err)
	}
	groupedPath := filepath.Join(reviewDir, "grouped", "group_internal_app_service.go.md")
	if err := os.WriteFile(groupedPath, []byte("stale grouped content\n"), 0o600); err != nil {
		t.Fatalf("write grouped placeholder: %v", err)
	}

	result, err := Migrate(context.Background(), MigrationConfig{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result.FilesMigrated != 2 {
		t.Fatalf("expected 2 migrated files, got %d", result.FilesMigrated)
	}
	if result.FilesAlreadyFrontmatter != 1 {
		t.Fatalf("expected 1 already-frontmatter file, got %d", result.FilesAlreadyFrontmatter)
	}

	taskContent, err := os.ReadFile(filepath.Join(workflowDir, "task_1.md"))
	if err != nil {
		t.Fatalf("read migrated task: %v", err)
	}
	if strings.Contains(string(taskContent), "<task_context>") {
		t.Fatalf("expected migrated task to remove XML metadata, got:\n%s", string(taskContent))
	}
	if !strings.Contains(string(taskContent), "status: pending") {
		t.Fatalf("expected migrated task to include front matter status, got:\n%s", string(taskContent))
	}
	if strings.Contains(string(taskContent), "domain:") || strings.Contains(string(taskContent), "scope:") {
		t.Fatalf("expected migrated task to drop v1-only fields, got:\n%s", string(taskContent))
	}

	issueContent, err := os.ReadFile(legacyIssuePath)
	if err != nil {
		t.Fatalf("read migrated issue: %v", err)
	}
	if strings.Contains(string(issueContent), "<review_context>") {
		t.Fatalf("expected migrated issue to remove XML metadata, got:\n%s", string(issueContent))
	}
	if !strings.Contains(string(issueContent), "status: pending") {
		t.Fatalf("expected migrated issue to include front matter status, got:\n%s", string(issueContent))
	}

	groupedContent, err := os.ReadFile(groupedPath)
	if err != nil {
		t.Fatalf("read legacy grouped file: %v", err)
	}
	if string(groupedContent) != "stale grouped content\n" {
		t.Fatalf("expected migrate to leave legacy grouped file untouched, got:\n%s", string(groupedContent))
	}
}

func TestMigrateDryRunLeavesLegacyArtifactsUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	workflowDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	taskPath := filepath.Join(workflowDir, "task_1.md")
	legacyTask := strings.Join([]string{
		"## status: pending",
		"<task_context><domain>backend</domain><type>feature</type><scope>small</scope><complexity>low</complexity></task_context>",
		"# Task 1: Demo",
		"",
	}, "\n")
	if err := os.WriteFile(taskPath, []byte(legacyTask), 0o600); err != nil {
		t.Fatalf("write legacy task: %v", err)
	}

	result, err := Migrate(context.Background(), MigrationConfig{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run migrate: %v", err)
	}
	if result.FilesMigrated != 1 {
		t.Fatalf("expected 1 planned migration, got %d", result.FilesMigrated)
	}

	content, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("read dry-run task: %v", err)
	}
	if string(content) != legacyTask {
		t.Fatalf("expected dry-run to leave file unchanged\nwant:\n%s\ngot:\n%s", legacyTask, string(content))
	}
}

func TestMigrateTreatsV1TaskArtifactsAsAlreadyFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	workflowDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	taskPath := filepath.Join(workflowDir, "task_1.md")
	v1Task := strings.Join([]string{
		"---",
		"status: pending",
		"domain: backend",
		"type: backend",
		"scope: small",
		"complexity: low",
		"---",
		"",
		"# Task 1: Demo",
		"",
	}, "\n")
	if err := os.WriteFile(taskPath, []byte(v1Task), 0o600); err != nil {
		t.Fatalf("write v1 task: %v", err)
	}

	result, err := Migrate(context.Background(), MigrationConfig{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result.FilesMigrated != 0 {
		t.Fatalf("expected no migrated files, got %d", result.FilesMigrated)
	}
	if result.FilesAlreadyFrontmatter != 1 {
		t.Fatalf("expected v1 task to be deferred as already-frontmatter, got %#v", result)
	}

	content, readErr := os.ReadFile(taskPath)
	if readErr != nil {
		t.Fatalf("read v1 task: %v", readErr)
	}
	if string(content) != v1Task {
		t.Fatalf("expected v1 task to be left unchanged\nwant:\n%s\ngot:\n%s", v1Task, string(content))
	}
}

func TestMigrateRejectsInvalidArtifactsWithoutWriting(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	workflowDir := filepath.Join(tmpDir, ".compozy", "tasks", "demo")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	taskPath := filepath.Join(workflowDir, "task_1.md")
	invalidTask := "# Task 1: Missing metadata\n"
	if err := os.WriteFile(taskPath, []byte(invalidTask), 0o600); err != nil {
		t.Fatalf("write invalid task: %v", err)
	}

	result, err := Migrate(context.Background(), MigrationConfig{})
	if err == nil {
		t.Fatal("expected migrate to fail on invalid artifacts")
	}
	if result == nil {
		t.Fatal("expected migration result on error")
	}
	if result.FilesInvalid != 1 {
		t.Fatalf("expected 1 invalid file, got %d", result.FilesInvalid)
	}

	content, readErr := os.ReadFile(taskPath)
	if readErr != nil {
		t.Fatalf("read invalid task after failed migrate: %v", readErr)
	}
	if string(content) != invalidTask {
		t.Fatalf("expected failed migrate to leave file unchanged\nwant:\n%s\ngot:\n%s", invalidTask, string(content))
	}
}
