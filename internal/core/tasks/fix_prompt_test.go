package tasks

import (
	"strings"
	"testing"
)

func TestFixPromptGolden(t *testing.T) {
	t.Parallel()

	registry := mustTaskRegistry(t)
	report := Report{
		TasksDir: "/tmp/tasks",
		Scanned:  2,
		Issues: []Issue{
			{
				Path:    "/tmp/tasks/task_01.md",
				Field:   "title",
				Message: "title is required",
			},
			{
				Path:    "/tmp/tasks/task_02.md",
				Field:   "type",
				Message: `type "nope" must be one of: ` + strings.Join(registry.Values(), ", "),
			},
		},
	}

	got := FixPrompt(report, registry)
	want := readGoldenFile(t, filepathJoin("testdata", "fix_prompt.golden"))
	if strings.TrimSuffix(got, "\n") != strings.TrimSuffix(want, "\n") {
		t.Fatalf("unexpected fix prompt\nwant:\n%s\ngot:\n%s", want, got)
	}

	for _, path := range []string{"/tmp/tasks/task_01.md", "/tmp/tasks/task_02.md"} {
		if !strings.Contains(got, path) {
			t.Fatalf("expected fix prompt to include %q", path)
		}
	}
	for _, message := range []string{"title is required", `type "nope" must be one of:`} {
		if !strings.Contains(got, message) {
			t.Fatalf("expected fix prompt to include %q", message)
		}
	}
	for _, value := range registry.Values() {
		if !strings.Contains(got, value) {
			t.Fatalf("expected fix prompt to include allowed type %q", value)
		}
	}
}

func filepathJoin(parts ...string) string {
	return strings.Join(parts, "/")
}
