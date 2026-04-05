package core

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	taskscore "github.com/compozy/compozy/internal/core/tasks"
)

func TestExtractTaskBodyTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "task prefix with colon",
			body: "# Task 1: ACP Agent Layer\n\nBody.\n",
			want: "ACP Agent Layer",
		},
		{
			name: "task prefix with dash",
			body: "# Task 10 - Cleanup\n\nBody.\n",
			want: "Cleanup",
		},
		{
			name: "plain title",
			body: "# Plain Title\n\nBody.\n",
			want: "Plain Title",
		},
		{
			name: "missing h1",
			body: "## Not H1\n\nBody.\n",
			want: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := taskscore.ExtractTaskBodyTitle(tt.body); got != tt.want {
				t.Fatalf("unexpected title\nwant: %q\ngot:  %q", tt.want, got)
			}
		})
	}
}

func TestMigrateV1ToV2RemapsTypesAndExtractsTitle(t *testing.T) {
	t.Parallel()

	registry := mustMigrationRegistry(t)
	tests := []struct {
		name          string
		rawType       string
		bodyTitle     string
		wantTitle     string
		wantType      string
		wantEmptyType bool
	}{
		{
			name:      "bug fix remaps to bugfix",
			rawType:   "Bug Fix",
			bodyTitle: "# Task 1: ACP Agent Layer",
			wantTitle: "ACP Agent Layer",
			wantType:  "bugfix",
		},
		{
			name:      "refactor remaps to refactor",
			rawType:   "Refactor",
			bodyTitle: "# Task 1: Cleanup",
			wantTitle: "Cleanup",
			wantType:  "refactor",
		},
		{
			name:      "documentation remaps to docs",
			rawType:   "Documentation",
			bodyTitle: "# Task 1: Docs",
			wantTitle: "Docs",
			wantType:  "docs",
		},
		{
			name:          "feature implementation stays empty",
			rawType:       "Feature Implementation",
			bodyTitle:     "# Task 1: Needs Human Classification",
			wantTitle:     "Needs Human Classification",
			wantType:      "",
			wantEmptyType: true,
		},
		{
			name:      "registry match is case insensitive",
			rawType:   "Frontend",
			bodyTitle: "# Task 1: UI",
			wantTitle: "UI",
			wantType:  "frontend",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := taskMarkdown(
				[]string{
					"status: pending",
					"domain: backend",
					"type: " + tt.rawType,
					"scope: full",
					"complexity: low",
					"dependencies: []",
				},
				tt.bodyTitle,
				"Body.",
			)

			migrated, outcome, err := migrateV1ToV2("/tmp/task_01.md", content, registry)
			if err != nil {
				t.Fatalf("migrateV1ToV2: %v", err)
			}
			if outcome != migrationOutcomeV1ToV2 {
				t.Fatalf("expected migration outcome %v, got %v", migrationOutcomeV1ToV2, outcome)
			}
			if migrated == nil {
				t.Fatal("expected migrated file")
			}
			if strings.Contains(migrated.content, "domain:") || strings.Contains(migrated.content, "scope:") {
				t.Fatalf("expected migrated content to drop v1-only keys, got:\n%s", migrated.content)
			}

			var meta model.TaskFileMeta
			body, err := frontmatter.Parse(migrated.content, &meta)
			if err != nil {
				t.Fatalf("parse migrated frontmatter: %v", err)
			}
			if meta.Title != tt.wantTitle {
				t.Fatalf("unexpected title\nwant: %q\ngot:  %q", tt.wantTitle, meta.Title)
			}
			if meta.TaskType != tt.wantType {
				t.Fatalf("unexpected type\nwant: %q\ngot:  %q", tt.wantType, meta.TaskType)
			}
			if !strings.Contains(body, "Body.") {
				t.Fatalf("expected body to be preserved, got:\n%s", body)
			}
			if tt.wantEmptyType && !strings.Contains(migrated.content, "type: \"\"") {
				t.Fatalf("expected explicit empty type in migrated output, got:\n%s", migrated.content)
			}
		})
	}
}

func TestMigrateConvertsLegacyArtifactsAndIgnoresLegacyGroupedDirectory(t *testing.T) {
	t.Parallel()

	workspaceRoot, workflowDir := makeMigrationWorkspace(t, "demo")
	taskPath := filepath.Join(workflowDir, "task_1.md")
	writeMigrationFile(t, taskPath, strings.Join([]string{
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
	}, "\n"))

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
	writeMigrationFile(t, legacyIssuePath, strings.Join([]string{
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
	}, "\n"))
	groupedPath := filepath.Join(reviewDir, "grouped", "group_internal_app_service.go.md")
	writeMigrationFile(t, groupedPath, "stale grouped content\n")

	result, err := Migrate(context.Background(), MigrationConfig{WorkspaceRoot: workspaceRoot})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result.FilesMigrated != 2 {
		t.Fatalf("expected 2 migrated files, got %d", result.FilesMigrated)
	}
	if result.V1ToV2Migrated != 1 {
		t.Fatalf("expected 1 task to pass through v1->v2, got %d", result.V1ToV2Migrated)
	}
	if result.FilesAlreadyFrontmatter != 1 {
		t.Fatalf("expected 1 already-frontmatter file, got %d", result.FilesAlreadyFrontmatter)
	}
	if !slices.Equal(result.UnmappedTypeFiles, []string{taskPath}) {
		t.Fatalf("unexpected unmapped type files\nwant: %#v\ngot:  %#v", []string{taskPath}, result.UnmappedTypeFiles)
	}

	taskContent := readMigrationFile(t, taskPath)
	if strings.Contains(taskContent, "<task_context>") {
		t.Fatalf("expected migrated task to remove XML metadata, got:\n%s", taskContent)
	}
	if strings.Contains(taskContent, "domain:") || strings.Contains(taskContent, "scope:") {
		t.Fatalf("expected migrated task to drop v1-only fields, got:\n%s", taskContent)
	}
	if !strings.Contains(taskContent, "title: Demo") {
		t.Fatalf("expected migrated task to include extracted title, got:\n%s", taskContent)
	}
	if !strings.Contains(taskContent, "type: \"\"") {
		t.Fatalf("expected migrated task to record unmapped type explicitly, got:\n%s", taskContent)
	}

	issueContent := readMigrationFile(t, legacyIssuePath)
	if strings.Contains(issueContent, "<review_context>") {
		t.Fatalf("expected migrated issue to remove XML metadata, got:\n%s", issueContent)
	}

	groupedContent := readMigrationFile(t, groupedPath)
	if groupedContent != "stale grouped content\n" {
		t.Fatalf("expected migrate to leave grouped file untouched, got:\n%s", groupedContent)
	}
}

func TestMigrateDryRunLeavesLegacyArtifactsUnchanged(t *testing.T) {
	t.Parallel()

	workspaceRoot, workflowDir := makeMigrationWorkspace(t, "demo")
	taskPath := filepath.Join(workflowDir, "task_1.md")
	legacyTask := strings.Join([]string{
		"## status: pending",
		"<task_context><domain>backend</domain><type>feature</type><scope>small</scope><complexity>low</complexity></task_context>",
		"# Task 1: Demo",
		"",
	}, "\n")
	writeMigrationFile(t, taskPath, legacyTask)

	result, err := Migrate(context.Background(), MigrationConfig{WorkspaceRoot: workspaceRoot, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run migrate: %v", err)
	}
	if result.FilesMigrated != 1 {
		t.Fatalf("expected 1 planned migration, got %d", result.FilesMigrated)
	}
	if result.V1ToV2Migrated != 1 {
		t.Fatalf("expected 1 v1->v2 migration, got %d", result.V1ToV2Migrated)
	}

	content := readMigrationFile(t, taskPath)
	if content != legacyTask {
		t.Fatalf("expected dry-run to leave file unchanged\nwant:\n%s\ngot:\n%s", legacyTask, content)
	}
}

func TestMigrateTreatsMissingTitleAsV1Metadata(t *testing.T) {
	t.Parallel()

	workspaceRoot, workflowDir := makeMigrationWorkspace(t, "demo")
	taskPath := filepath.Join(workflowDir, "task_01.md")
	writeMigrationFile(t, taskPath, taskMarkdown(
		[]string{
			"status: pending",
			"type: backend",
			"complexity: low",
			"dependencies: []",
		},
		"# Task 1: Demo",
		"Body.",
	))

	result, err := Migrate(context.Background(), MigrationConfig{WorkspaceRoot: workspaceRoot})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result.FilesMigrated != 1 {
		t.Fatalf("expected 1 migrated file, got %d", result.FilesMigrated)
	}
	if result.V1ToV2Migrated != 1 {
		t.Fatalf("expected 1 v1->v2 migration, got %d", result.V1ToV2Migrated)
	}
	if result.FilesAlreadyFrontmatter != 0 {
		t.Fatalf("expected no already-frontmatter files, got %#v", result)
	}

	content := readMigrationFile(t, taskPath)
	if !strings.Contains(content, "title: Demo") {
		t.Fatalf("expected migrated file to extract title, got:\n%s", content)
	}
	if !strings.Contains(content, "type: backend") {
		t.Fatalf("expected migrated file to preserve already-valid type, got:\n%s", content)
	}
}

func TestMigrateTreatsV2TaskArtifactsAsAlreadyFrontmatter(t *testing.T) {
	t.Parallel()

	workspaceRoot, workflowDir := makeMigrationWorkspace(t, "demo")
	taskPath := filepath.Join(workflowDir, "task_01.md")
	v2Task := taskMarkdown(
		[]string{
			"status: pending",
			"title: Demo",
			"type: backend",
			"complexity: low",
			"dependencies: []",
		},
		"# Task 1: Demo",
		"Body.",
	)
	writeMigrationFile(t, taskPath, v2Task)

	result, err := Migrate(context.Background(), MigrationConfig{WorkspaceRoot: workspaceRoot})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result.FilesMigrated != 0 {
		t.Fatalf("expected no migrated files, got %d", result.FilesMigrated)
	}
	if result.V1ToV2Migrated != 0 {
		t.Fatalf("expected no v1->v2 migrations, got %d", result.V1ToV2Migrated)
	}
	if result.FilesAlreadyFrontmatter != 1 {
		t.Fatalf("expected v2 task to be already frontmatter, got %#v", result)
	}

	content := readMigrationFile(t, taskPath)
	if content != v2Task {
		t.Fatalf("expected v2 task to be left unchanged\nwant:\n%s\ngot:\n%s", v2Task, content)
	}
}

func TestMigrateMixedDirectoryCountsV1ToV2AndTracksUnmappedTypes(t *testing.T) {
	t.Parallel()

	workspaceRoot, workflowDir := makeMigrationWorkspace(t, "demo")
	writeMigrationFile(t, filepath.Join(workflowDir, "_meta.md"), strings.Join([]string{
		"---",
		"created_at: 2026-04-04T23:19:02Z",
		"updated_at: 2026-04-04T23:54:13Z",
		"---",
		"",
		"## Summary",
		"- Total: 3",
		"",
	}, "\n"))
	memoryDir := filepath.Join(workflowDir, "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	writeMigrationFile(t, filepath.Join(memoryDir, "task_01.md"), "# Task memory that must be ignored\n")

	v1Path := filepath.Join(workflowDir, "task_01.md")
	writeMigrationFile(t, v1Path, taskMarkdown(
		[]string{
			"status: pending",
			"domain: backend",
			"type: Bug Fix",
			"scope: full",
			"complexity: low",
			"dependencies: []",
		},
		"# Task 1: Bug Fix",
		"Body.",
	))
	legacyPath := filepath.Join(workflowDir, "task_02.md")
	writeMigrationFile(t, legacyPath, strings.Join([]string{
		"## status: pending",
		"",
		"<task_context>",
		"  <domain>backend</domain>",
		"  <type>Feature Implementation</type>",
		"  <scope>full</scope>",
		"  <complexity>medium</complexity>",
		"</task_context>",
		"",
		"# Task 2: Needs Classification",
		"",
		"Body.",
		"",
	}, "\n"))
	v2Path := filepath.Join(workflowDir, "task_03.md")
	v2Content := taskMarkdown(
		[]string{
			"status: pending",
			"title: Already V2",
			"type: docs",
			"complexity: low",
			"dependencies: []",
		},
		"# Task 3: Already V2",
		"Body.",
	)
	writeMigrationFile(t, v2Path, v2Content)

	result, err := Migrate(context.Background(), MigrationConfig{WorkspaceRoot: workspaceRoot, TasksDir: workflowDir})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if result.FilesMigrated != 2 {
		t.Fatalf("expected 2 migrated files, got %d", result.FilesMigrated)
	}
	if result.V1ToV2Migrated != 2 {
		t.Fatalf("expected 2 files to pass through v1->v2, got %d", result.V1ToV2Migrated)
	}
	if result.FilesAlreadyFrontmatter != 1 {
		t.Fatalf("expected 1 already-frontmatter file, got %d", result.FilesAlreadyFrontmatter)
	}
	if result.FilesInvalid != 0 {
		t.Fatalf("expected no invalid files, got %d", result.FilesInvalid)
	}
	if !slices.Equal(result.UnmappedTypeFiles, []string{legacyPath}) {
		t.Fatalf("unexpected unmapped paths\nwant: %#v\ngot:  %#v", []string{legacyPath}, result.UnmappedTypeFiles)
	}

	if got := readMigrationFile(t, v2Path); got != v2Content {
		t.Fatalf("expected v2 file to remain unchanged\nwant:\n%s\ngot:\n%s", v2Content, got)
	}
	if got := readMigrationFile(t, v1Path); !strings.Contains(got, "type: bugfix") {
		t.Fatalf("expected v1 file to remap type, got:\n%s", got)
	}
	if got := readMigrationFile(t, legacyPath); !strings.Contains(got, "type: \"\"") {
		t.Fatalf("expected legacy file to keep empty type for manual follow-up, got:\n%s", got)
	}
}

func TestMigrateRejectsInvalidArtifactsWithoutWriting(t *testing.T) {
	t.Parallel()

	workspaceRoot, workflowDir := makeMigrationWorkspace(t, "demo")
	taskPath := filepath.Join(workflowDir, "task_1.md")
	invalidTask := "# Task 1: Missing metadata\n"
	writeMigrationFile(t, taskPath, invalidTask)

	result, err := Migrate(context.Background(), MigrationConfig{WorkspaceRoot: workspaceRoot})
	if err == nil {
		t.Fatal("expected migrate to fail on invalid artifacts")
	}
	if result == nil {
		t.Fatal("expected migration result on error")
	}
	if result.FilesInvalid != 1 {
		t.Fatalf("expected 1 invalid file, got %d", result.FilesInvalid)
	}

	content := readMigrationFile(t, taskPath)
	if content != invalidTask {
		t.Fatalf("expected failed migrate to leave file unchanged\nwant:\n%s\ngot:\n%s", invalidTask, content)
	}
}

func mustMigrationRegistry(t *testing.T) *taskscore.TypeRegistry {
	t.Helper()

	registry, err := taskscore.NewRegistry(nil)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	return registry
}

func makeMigrationWorkspace(t *testing.T, name string) (string, string) {
	t.Helper()

	root := t.TempDir()
	workflowDir := filepath.Join(root, ".compozy", "tasks", name)
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}
	return root, workflowDir
}

func writeMigrationFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readMigrationFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func taskMarkdown(frontMatter []string, h1 string, bodyLines ...string) string {
	lines := []string{"---"}
	lines = append(lines, frontMatter...)
	lines = append(lines, "---", "", h1, "")
	lines = append(lines, bodyLines...)
	return strings.Join(lines, "\n") + "\n"
}
