package tasks

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

func TestWrapParseErrorProvidesMigrationGuidance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "legacy xml metadata", err: ErrLegacyTaskMetadata},
		{name: "v1 front matter", err: ErrV1TaskMetadata},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := WrapParseError("/tmp/task_01.md", tt.err)
			if !strings.Contains(err.Error(), "run `compozy migrate`") {
				t.Fatalf("expected migrate guidance, got %v", err)
			}
			if !errors.Is(err, tt.err) {
				t.Fatalf("errors.Is(%v, %v) = false, want true", err, tt.err)
			}
		})
	}
}
