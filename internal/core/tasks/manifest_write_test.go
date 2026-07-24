package tasks

import (
	"context"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/frontmatter"
)

func TestIsTaskGraphManifestContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "Should detect v2 manifest with nodes",
			content: taskGraphManifestMarkdown("demo", nil),
			want:    true,
		},
		{
			name:    "Should detect v2 manifest by schema version alone",
			content: "---\nschema_version: \"compozy.tasks/v2\"\nworkflow: demo\ngraph:\n  nodes: []\n  edges: []\n---\n",
			want:    true,
		},
		{
			name:    "Should reject legacy markdown table without front matter",
			content: "# Legacy - Task List\n\n| # | Title | Status | Complexity | Dependencies |\n",
			want:    false,
		},
		{
			name: "Should reject a plain task file",
			content: taskMarkdown(
				[]string{"status: pending", "title: Task 1", "type: backend", "complexity: low"},
				"# Task 1",
			),
			want: false,
		},
		{
			name:    "Should reject empty content",
			content: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsTaskGraphManifestContent(tt.content); got != tt.want {
				t.Fatalf("IsTaskGraphManifestContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderTaskGraphTaskFile(t *testing.T) {
	t.Parallel()

	t.Run("Should omit dependencies and round-trip through ParseTaskFile", func(t *testing.T) {
		t.Parallel()

		content, err := RenderTaskGraphTaskFile("pending", "QA plan", "docs", "high", "# Task 04: QA plan\n\nBody.\n")
		if err != nil {
			t.Fatalf("RenderTaskGraphTaskFile() error = %v", err)
		}
		if taskFrontMatterHasKey(content, "dependencies") {
			t.Fatalf("rendered file must not contain a dependencies key:\n%s", content)
		}
		for _, want := range []string{"status: pending", "title: QA plan", "type: docs", "complexity: high"} {
			if !strings.Contains(content, want) {
				t.Fatalf("rendered file missing %q:\n%s", want, content)
			}
		}
		entry, err := ParseTaskFile(content)
		if err != nil {
			t.Fatalf("ParseTaskFile() error = %v", err)
		}
		if entry.Status != "pending" || entry.Title != "QA plan" || entry.TaskType != "docs" {
			t.Fatalf("parsed entry = %#v", entry)
		}
		if len(entry.Dependencies) != 0 {
			t.Fatalf("entry.Dependencies = %#v, want empty", entry.Dependencies)
		}
	})

	t.Run("Should omit complexity when empty", func(t *testing.T) {
		t.Parallel()

		content, err := RenderTaskGraphTaskFile("pending", "QA plan", "docs", "", "# Task\n")
		if err != nil {
			t.Fatalf("RenderTaskGraphTaskFile() error = %v", err)
		}
		if strings.Contains(content, "complexity:") {
			t.Fatalf("rendered file must omit empty complexity:\n%s", content)
		}
	})
}

func TestAppendTaskGraphNode(t *testing.T) {
	t.Parallel()

	t.Run("Should append node and dependency edges then validate", func(t *testing.T) {
		t.Parallel()

		tasksDir := t.TempDir()
		base := taskGraphManifestMarkdown("demo", []string{
			"    - from: task_01",
			"      to: task_03",
		})
		updated, err := AppendTaskGraphNode(base, "task_04", "task_04.md", []string{"task_01", "task_02"})
		if err != nil {
			t.Fatalf("AppendTaskGraphNode() error = %v", err)
		}
		writeTaskManifestTestFile(t, tasksDir, "_tasks.md", updated)
		for _, name := range []string{"task_01", "task_02", "task_03"} {
			writeTaskManifestTestFile(t, tasksDir, name+".md", taskMarkdown(
				[]string{"status: pending", "title: " + name, "type: backend", "complexity: low"},
				"# "+name,
			))
		}
		task04, err := RenderTaskGraphTaskFile("pending", "Task 4", "docs", "high", "# Task 04: Task 4\n\nBody.\n")
		if err != nil {
			t.Fatalf("RenderTaskGraphTaskFile() error = %v", err)
		}
		writeTaskManifestTestFile(t, tasksDir, "task_04.md", task04)

		manifest, taskFiles, err := LoadValidatedTaskGraphManifest(context.Background(), tasksDir, "demo")
		if err != nil {
			t.Fatalf("LoadValidatedTaskGraphManifest() error = %v", err)
		}
		if len(taskFiles) != 4 {
			t.Fatalf("task file count = %d, want 4", len(taskFiles))
		}
		if !manifestHasEdge(manifest, "task_01", "task_04") || !manifestHasEdge(manifest, "task_02", "task_04") {
			t.Fatalf("manifest edges = %#v, want task_01->task_04 and task_02->task_04", manifest.Graph.Edges)
		}
		if manifestHasEdge(manifest, "task_03", "task_04") {
			t.Fatalf("unexpected edge task_03->task_04 in %#v", manifest.Graph.Edges)
		}
	})

	t.Run("Should deduplicate existing node and edges", func(t *testing.T) {
		t.Parallel()

		base := taskGraphManifestMarkdown("demo", []string{
			"    - from: task_01",
			"      to: task_02",
		})
		updated, err := AppendTaskGraphNode(base, "task_02", "task_02.md", []string{"task_01"})
		if err != nil {
			t.Fatalf("AppendTaskGraphNode() error = %v", err)
		}
		var manifest TaskGraphManifest
		if _, err := frontmatter.Parse(updated, &manifest); err != nil {
			t.Fatalf("parse updated manifest error = %v", err)
		}
		if countNodes(manifest, "task_02") != 1 {
			t.Fatalf("task_02 node count = %d, want 1", countNodes(manifest, "task_02"))
		}
		if countEdges(manifest, "task_01", "task_02") != 1 {
			t.Fatalf("task_01->task_02 edge count = %d, want 1", countEdges(manifest, "task_01", "task_02"))
		}
	})

	t.Run("Should reject a dependency that is not a graph node", func(t *testing.T) {
		t.Parallel()

		base := taskGraphManifestMarkdown("demo", nil)
		_, err := AppendTaskGraphNode(base, "task_04", "task_04.md", []string{"task_99"})
		if err == nil {
			t.Fatal("AppendTaskGraphNode() error = nil, want dangling dependency error")
		}
		if !strings.Contains(err.Error(), "task_99") {
			t.Fatalf("error = %v, want mention of task_99", err)
		}
	})

	t.Run("Should preserve the manifest body", func(t *testing.T) {
		t.Parallel()

		base := taskGraphManifestMarkdown("demo", nil)
		updated, err := AppendTaskGraphNode(base, "task_04", "task_04.md", nil)
		if err != nil {
			t.Fatalf("AppendTaskGraphNode() error = %v", err)
		}
		if !strings.Contains(updated, "# demo Tasks") {
			t.Fatalf("updated manifest lost body:\n%s", updated)
		}
	})
}

func manifestHasEdge(manifest TaskGraphManifest, from, to string) bool {
	return countEdges(manifest, from, to) > 0
}

func countEdges(manifest TaskGraphManifest, from, to string) int {
	count := 0
	for _, edge := range manifest.Graph.Edges {
		if edge.From == from && edge.To == to {
			count++
		}
	}
	return count
}

func countNodes(manifest TaskGraphManifest, id string) int {
	count := 0
	for _, node := range manifest.Graph.Nodes {
		if node.ID == id {
			count++
		}
	}
	return count
}
