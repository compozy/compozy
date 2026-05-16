package tasks

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestReadTaskEntriesRecursiveDiscoversNestedAndSortsByDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := map[string]string{
		"task_01.md":                  pendingTaskBody("Root 01"),
		"features/auth/task_01.md":    pendingTaskBody("Auth 01"),
		"features/auth/task_02.md":    pendingTaskBody("Auth 02"),
		"features/payment/task_01.md": pendingTaskBody("Payment 01"),
	}
	writeNestedFiles(t, dir, files)

	entries, err := ReadTaskEntriesRecursive(dir, false)
	if err != nil {
		t.Fatalf("ReadTaskEntriesRecursive: %v", err)
	}

	gotNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		gotNames = append(gotNames, entry.Name)
	}
	wantNames := []string{
		"task_01.md",
		"features/auth/task_01.md",
		"features/auth/task_02.md",
		"features/payment/task_01.md",
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("unexpected entry order\nwant: %#v\ngot:  %#v", wantNames, gotNames)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name, "\\") {
			t.Fatalf("entry name uses native separator: %q", entry.Name)
		}
		wantCodeFile := strings.TrimSuffix(entry.Name, ".md")
		if entry.CodeFile != wantCodeFile {
			t.Fatalf("entry %q has CodeFile %q; want %q", entry.Name, entry.CodeFile, wantCodeFile)
		}
		wantAbs := filepath.Join(dir, filepath.FromSlash(entry.Name))
		if entry.AbsPath != wantAbs {
			t.Fatalf("entry %q has AbsPath %q; want %q", entry.Name, entry.AbsPath, wantAbs)
		}
	}
}

func TestReadTaskEntriesRecursiveSkipsHiddenAndReviewDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := map[string]string{
		".cache/task_01.md":      pendingTaskBody("Cache"),
		"_drafts/task_01.md":     pendingTaskBody("Drafts"),
		"reviews-001/task_01.md": pendingTaskBody("Reviews"),
		"adrs/task_01.md":        pendingTaskBody("Adrs"),
		"memory/task_01.md":      pendingTaskBody("Memory"),
	}
	writeNestedFiles(t, dir, files)

	entries, err := ReadTaskEntriesRecursive(dir, false)
	if err != nil {
		t.Fatalf("ReadTaskEntriesRecursive: %v", err)
	}
	if len(entries) != 0 {
		got := make([]string, 0, len(entries))
		for _, entry := range entries {
			got = append(got, entry.Name)
		}
		t.Fatalf("expected no entries; got %#v", got)
	}
}

func TestReadTaskEntriesRecursiveIgnoresNonMatchingFilenames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("ignored\n"), 0o600); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "task_bad.md"), []byte("ignored\n"), 0o600); err != nil {
		t.Fatalf("write task_bad.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "task_01.txt"), []byte("ignored\n"), 0o600); err != nil {
		t.Fatalf("write task_01.txt: %v", err)
	}

	entries, err := ReadTaskEntriesRecursive(dir, false)
	if err != nil {
		t.Fatalf("ReadTaskEntriesRecursive: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty result; got %d entries", len(entries))
	}
}

func TestReadTaskEntriesNonRecursiveUnchanged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := map[string]string{
		"task_01.md":               pendingTaskBody("Root 01"),
		"task_02.md":               pendingTaskBody("Root 02"),
		"features/auth/task_01.md": pendingTaskBody("Auth 01"),
	}
	writeNestedFiles(t, dir, files)

	entries, err := ReadTaskEntries(dir, false)
	if err != nil {
		t.Fatalf("ReadTaskEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected flat mode to ignore nested files; got %d entries", len(entries))
	}
	got := []string{entries[0].Name, entries[1].Name}
	want := []string{"task_01.md", "task_02.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("flat mode names changed\nwant: %#v\ngot:  %#v", want, got)
	}
	for _, entry := range entries {
		if entry.Name != entry.CodeFile+".md" {
			t.Fatalf("expected basename CodeFile in flat mode, got entry %#v", entry)
		}
	}
}

func TestShouldSkipDirMatchesAdr002Predicate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		skip bool
	}{
		{".cache", true},
		{".git", true},
		{"_archived", true},
		{"_meta", true},
		{"reviews-001", true},
		{"reviews-", true},
		{"adrs", true},
		{"memory", true},
		{"features", false},
		{"auth", false},
		{"payment", false},
		{"adrs-extras", false},
		{"reviews", false},
		{"memories", false},
	}
	for _, tc := range cases {
		got := shouldSkipDir(tc.name)
		if got != tc.skip {
			t.Errorf("shouldSkipDir(%q) = %v; want %v", tc.name, got, tc.skip)
		}
	}
}

func pendingTaskBody(title string) string {
	return strings.Join([]string{
		"---",
		"status: pending",
		"title: " + title,
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# " + title,
		"",
	}, "\n")
}

func completedTaskBody(title string) string {
	return strings.Join([]string{
		"---",
		"status: completed",
		"title: " + title,
		"type: backend",
		"complexity: low",
		"---",
		"",
		"# " + title,
		"",
	}, "\n")
}

func writeNestedFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}
