package plan

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/compozy/looper/internal/looper/model"
)

func TestReadTaskEntriesSortsNumericallyAndFiltersCompleted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := map[string]string{
		"task_10.md": "## status: pending\n<task_context><domain>x</domain><type>feature</type><scope>s</scope><complexity>low</complexity></task_context>\n",
		"task_2.md":  "## status: pending\n<task_context><domain>x</domain><type>feature</type><scope>s</scope><complexity>low</complexity></task_context>\n",
		"task_3.md":  "## status: completed\n<task_context><domain>x</domain><type>feature</type><scope>s</scope><complexity>low</complexity></task_context>\n",
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
	if err := os.MkdirAll(filepath.Join("tasks", "prd-demo"), 0o755); err != nil {
		t.Fatalf("mkdir prd dir: %v", err)
	}

	prValue, inputDir, resolved, err := resolveInputs(&model.RuntimeConfig{
		PR:   "demo",
		Mode: model.ExecutionModePRDTasks,
	})
	if err != nil {
		t.Fatalf("resolveInputs: %v", err)
	}
	if prValue != "demo" {
		t.Fatalf("unexpected pr value: %q", prValue)
	}
	if inputDir != "tasks/prd-demo" {
		t.Fatalf("unexpected input dir: %q", inputDir)
	}
	wantResolved := filepath.Join(tmp, "tasks", "prd-demo")
	if resolved != wantResolved {
		t.Fatalf("unexpected resolved dir\nwant: %q\ngot:  %q", wantResolved, resolved)
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
				Content:  "## status: pending\n<task_context><domain>backend</domain><type>feature</type><scope>small</scope><complexity>low</complexity></task_context>\n",
				CodeFile: "task_1",
			},
		},
		"task_2": {
			{
				Name:     "task_2.md",
				AbsPath:  filepath.Join(issuesDir, "task_2.md"),
				Content:  "## status: pending\n<task_context><domain>backend</domain><type>feature</type><scope>small</scope><complexity>low</complexity></task_context>\n",
				CodeFile: "task_2",
			},
		},
	}

	jobs, groupedWritten, err := prepareJobs(
		"demo",
		groups,
		promptRoot,
		issuesDir,
		5,
		true,
		false,
		model.ExecutionModePRDTasks,
	)
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
	}
}
