package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestValidateRules(t *testing.T) {
	t.Parallel()

	registry := mustTaskRegistry(t)
	tests := []struct {
		name        string
		files       map[string]string
		wantFields  []string
		wantOK      bool
		wantScanned int
	}{
		{
			name: "reports missing title",
			files: map[string]string{
				"task_01.md": taskMarkdown(
					[]string{"status: pending", "type: backend", "complexity: low"},
					"# Task 1: Example Task",
				),
			},
			wantFields:  []string{"title"},
			wantScanned: 1,
		},
		{
			name: "reports title h1 mismatch",
			files: map[string]string{
				"task_01.md": taskMarkdown(
					[]string{"status: pending", "title: Example Task", "type: backend", "complexity: low"},
					"# Task 1: Different Title",
				),
			},
			wantFields:  []string{"title_h1_sync"},
			wantScanned: 1,
		},
		{
			name: "reports invalid type",
			files: map[string]string{
				"task_01.md": taskMarkdown(
					[]string{"status: pending", "title: Example Task", "type: nope", "complexity: low"},
					"# Task 1: Example Task",
				),
			},
			wantFields:  []string{"type"},
			wantScanned: 1,
		},
		{
			name: "reports invalid status",
			files: map[string]string{
				"task_01.md": taskMarkdown(
					[]string{"status: in-progress", "title: Example Task", "type: backend", "complexity: low"},
					"# Task 1: Example Task",
				),
			},
			wantFields:  []string{"status"},
			wantScanned: 1,
		},
		{
			name: "reports invalid complexity",
			files: map[string]string{
				"task_01.md": taskMarkdown(
					[]string{"status: pending", "title: Example Task", "type: backend", "complexity: extreme"},
					"# Task 1: Example Task",
				),
			},
			wantFields:  []string{"complexity"},
			wantScanned: 1,
		},
		{
			name: "reports missing dependency",
			files: map[string]string{
				"task_01.md": taskMarkdown(
					[]string{
						"status: pending",
						"title: Example Task",
						"type: backend",
						"complexity: low",
						"dependencies:",
						"  - task_99",
					},
					"# Task 1: Example Task",
				),
			},
			wantFields:  []string{"dependencies"},
			wantScanned: 1,
		},
		{
			name: "reports legacy keys",
			files: map[string]string{
				"task_01.md": taskMarkdown(
					[]string{
						"status: pending",
						"title: Example Task",
						"domain: backend",
						"type: backend",
						"scope: full",
						"complexity: low",
					},
					"# Task 1: Example Task",
				),
			},
			wantFields:  []string{"domain", "scope"},
			wantScanned: 1,
		},
		{
			name: "accepts clean v2 task",
			files: map[string]string{
				"task_01.md": taskMarkdown(
					[]string{"status: pending", "title: Example Task", "type: backend", "complexity: low"},
					"# Task 1: Example Task",
				),
			},
			wantOK:      true,
			wantScanned: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tasksDir := t.TempDir()
			for name, content := range tt.files {
				writeRawTaskFile(t, tasksDir, name, content)
			}

			report, err := Validate(context.Background(), tasksDir, registry)
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if report.Scanned != tt.wantScanned {
				t.Fatalf("unexpected scanned count\nwant: %d\ngot:  %d", tt.wantScanned, report.Scanned)
			}
			if report.OK() != tt.wantOK {
				t.Fatalf("unexpected OK state\nwant: %v\ngot:  %v\nissues: %#v", tt.wantOK, report.OK(), report.Issues)
			}

			gotFields := make([]string, 0, len(report.Issues))
			for _, issue := range report.Issues {
				gotFields = append(gotFields, issue.Field)
			}
			slices.Sort(gotFields)
			slices.Sort(tt.wantFields)
			if !slices.Equal(gotFields, tt.wantFields) {
				t.Fatalf("unexpected issue fields\nwant: %#v\ngot:  %#v", tt.wantFields, gotFields)
			}
		})
	}
}

func TestValidateTypeIssueListsAllowedValues(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeRawTaskFile(t, tasksDir, "task_01.md", taskMarkdown(
		[]string{"status: pending", "title: Example Task", "type: nope", "complexity: low"},
		"# Task 1: Example Task",
	))

	registry := mustTaskRegistry(t)
	report, err := Validate(context.Background(), tasksDir, registry)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(report.Issues) != 1 {
		t.Fatalf("expected exactly one issue, got %#v", report.Issues)
	}
	if report.Issues[0].Field != "type" {
		t.Fatalf("expected type issue, got %#v", report.Issues[0])
	}
	for _, value := range registry.Values() {
		if !strings.Contains(report.Issues[0].Message, value) {
			t.Fatalf("expected type issue to mention %q, got %q", value, report.Issues[0].Message)
		}
	}
}

func TestValidateAcceptsTitleH1Prefixes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		h1   string
	}{
		{name: "task prefix with colon", h1: "# Task 1: ACP Agent Layer"},
		{name: "task prefix with hyphen", h1: "# Task 1 - ACP Agent Layer"},
		{name: "plain title", h1: "# ACP Agent Layer"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tasksDir := t.TempDir()
			writeRawTaskFile(t, tasksDir, "task_01.md", taskMarkdown(
				[]string{"status: pending", "title: ACP Agent Layer", "type: backend", "complexity: medium"},
				tt.h1,
			))

			report, err := Validate(context.Background(), tasksDir, mustTaskRegistry(t))
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if !report.OK() {
				t.Fatalf("expected clean report, got %#v", report.Issues)
			}
		})
	}
}

func TestValidateTitleH1Golden(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeRawTaskFile(t, tasksDir, "task_01.md", taskMarkdown(
		[]string{"status: pending", "title: ACP Agent Layer", "type: backend", "complexity: medium"},
		"# Task 1: ACP Agent Layer",
	))
	writeRawTaskFile(t, tasksDir, "task_02.md", taskMarkdown(
		[]string{"status: pending", "title: ACP Agent Layer", "type: backend", "complexity: medium"},
		"# Task 2 - ACP Agent Layer",
	))
	writeRawTaskFile(t, tasksDir, "task_03.md", taskMarkdown(
		[]string{"status: pending", "title: ACP Agent Layer", "type: backend", "complexity: medium"},
		"# ACP Agent Layer",
	))
	writeRawTaskFile(t, tasksDir, "task_04.md", taskMarkdown(
		[]string{"status: pending", "title: ACP Agent Layer", "type: backend", "complexity: medium"},
		"# Task 4: Different Title",
	))

	report, err := Validate(context.Background(), tasksDir, mustTaskRegistry(t))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	goldenPath := filepath.Join("testdata", "validate_title_h1.golden")
	want := readGoldenFile(t, goldenPath)
	got := renderIssueSummary(report.Issues)
	if got != want {
		t.Fatalf("unexpected title/H1 golden output\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestValidateEmptyDirectory(t *testing.T) {
	t.Parallel()

	report, err := Validate(context.Background(), t.TempDir(), mustTaskRegistry(t))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if report.Scanned != 0 {
		t.Fatalf("expected zero scanned tasks, got %d", report.Scanned)
	}
	if !report.OK() {
		t.Fatalf("expected empty directory to be valid, got %#v", report.Issues)
	}
}

func TestValidateRequiresRegistry(t *testing.T) {
	t.Parallel()

	_, err := Validate(context.Background(), t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected registry error")
	}
	if !strings.Contains(err.Error(), "task type registry is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRecursesIntoNestedDirectories(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeRawTaskFile(t, tasksDir, "task_01.md", taskMarkdown(
		[]string{"status: pending", "title: Root", "type: backend", "complexity: low"},
		"# Root",
	))
	nestedDir := filepath.Join(tasksDir, "features", "auth")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	writeRawTaskFile(t, nestedDir, "task_01.md", taskMarkdown(
		[]string{"status: pending", "type: backend", "complexity: low"},
		"# Nested",
	))

	report, err := Validate(context.Background(), tasksDir, mustTaskRegistry(t))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if report.Scanned != 2 {
		t.Fatalf("expected recursive scan to find 2 files, got %d", report.Scanned)
	}
	if report.OK() {
		t.Fatalf("expected nested file to surface a title issue, got clean report")
	}
	nestedPath := filepath.Join(nestedDir, "task_01.md")
	found := false
	for _, issue := range report.Issues {
		if issue.Path == nestedPath && issue.Field == "title" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected nested title issue at %q, got %#v", nestedPath, report.Issues)
	}
}

func TestValidateWithOptionsFlatIgnoresNestedTasks(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeRawTaskFile(t, tasksDir, "task_01.md", taskMarkdown(
		[]string{"status: pending", "title: Root", "type: backend", "complexity: low"},
		"# Root",
	))
	nestedDir := filepath.Join(tasksDir, "features", "auth")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	writeRawTaskFile(t, nestedDir, "task_01.md", taskMarkdown(
		[]string{"status: pending", "type: backend", "complexity: low"},
		"# Nested",
	))

	report, err := ValidateWithOptions(
		context.Background(),
		tasksDir,
		mustTaskRegistry(t),
		ValidateOptions{Recursive: false},
	)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if report.Scanned != 1 {
		t.Fatalf("expected flat scan to find 1 file, got %d", report.Scanned)
	}
	if !report.OK() {
		t.Fatalf("expected flat scan to ignore nested draft, got %#v", report.Issues)
	}
}

func TestValidateSkipsHiddenAndReviewDirectories(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeRawTaskFile(t, tasksDir, "task_01.md", taskMarkdown(
		[]string{"status: pending", "title: Root", "type: backend", "complexity: low"},
		"# Root",
	))
	for _, sub := range []string{".cache", "_drafts", "reviews-001", "adrs", "memory"} {
		dir := filepath.Join(tasksDir, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
		writeRawTaskFile(t, dir, "task_01.md", "garbage")
	}

	report, err := Validate(context.Background(), tasksDir, mustTaskRegistry(t))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if report.Scanned != 1 {
		t.Fatalf("expected validator to skip hidden dirs; got scanned=%d", report.Scanned)
	}
	if !report.OK() {
		t.Fatalf("expected clean report, got %#v", report.Issues)
	}
}

func TestValidateRejectsLegacyTaskMetadata(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	writeRawTaskFile(t, tasksDir, "task_01.md", strings.Join([]string{
		"## status: pending",
		"<task_context><domain>backend</domain><type>backend</type><scope>full</scope><complexity>low</complexity></task_context>",
		"# Task 1: Legacy Task",
		"",
	}, "\n"))

	_, err := Validate(context.Background(), tasksDir, mustTaskRegistry(t))
	if err == nil {
		t.Fatal("expected legacy metadata error")
	}
	if !strings.Contains(err.Error(), "legacy XML task metadata detected") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustTaskRegistry(t *testing.T) *TypeRegistry {
	t.Helper()

	registry, err := NewRegistry(nil)
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	return registry
}

func taskMarkdown(frontMatter []string, h1 string) string {
	lines := []string{"---"}
	lines = append(lines, frontMatter...)
	lines = append(lines, "---", "", h1, "", "Body.")
	return strings.Join(lines, "\n") + "\n"
}

func writeRawTaskFile(t *testing.T, tasksDir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(tasksDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readGoldenFile(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file %s: %v", path, err)
	}
	return string(body)
}

func renderIssueSummary(issues []Issue) string {
	if len(issues) == 0 {
		return ""
	}
	lines := make([]string, 0, len(issues))
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("%s|%s|%s", filepath.Base(issue.Path), issue.Field, issue.Message))
	}
	return strings.Join(lines, "\n") + "\n"
}
