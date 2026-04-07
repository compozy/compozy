package tasks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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

	entries, err := ReadTaskEntries(dir, false)
	if err != nil {
		t.Fatalf("ReadTaskEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected completed tasks to be filtered, got %d entries", len(entries))
	}

	gotNames := []string{entries[0].Name, entries[1].Name}
	wantNames := []string{"task_2.md", "task_10.md"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected task order\nwant: %#v\ngot:  %#v", wantNames, gotNames)
	}
}
