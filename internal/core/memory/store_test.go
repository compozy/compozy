package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareBootstrapsWorkflowAndTaskMemory(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	ctx, err := Prepare(tasksDir, "task_07.md")
	if err != nil {
		t.Fatalf("prepare workflow memory: %v", err)
	}

	if ctx.Directory != filepath.Join(tasksDir, DirName) {
		t.Fatalf("unexpected memory dir: %q", ctx.Directory)
	}
	if ctx.Workflow.Path != filepath.Join(tasksDir, DirName, WorkflowFileName) {
		t.Fatalf("unexpected workflow path: %q", ctx.Workflow.Path)
	}
	if ctx.Task.Path != filepath.Join(tasksDir, DirName, "task_07.md") {
		t.Fatalf("unexpected task path: %q", ctx.Task.Path)
	}

	workflowBody, err := os.ReadFile(ctx.Workflow.Path)
	if err != nil {
		t.Fatalf("read workflow memory: %v", err)
	}
	if !strings.Contains(string(workflowBody), workflowHeader) {
		t.Fatalf("expected workflow template header, got %q", string(workflowBody))
	}

	taskBody, err := os.ReadFile(ctx.Task.Path)
	if err != nil {
		t.Fatalf("read task memory: %v", err)
	}
	if !strings.Contains(string(taskBody), taskHeaderPrefix+"task_07.md") {
		t.Fatalf("expected task template header, got %q", string(taskBody))
	}
}

func TestPreparePreservesExistingMemoryFiles(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	memoryDir := filepath.Join(tasksDir, DirName)
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}

	workflowPath := filepath.Join(memoryDir, WorkflowFileName)
	taskPath := filepath.Join(memoryDir, "task_02.md")
	workflowBody := "custom workflow memory\n"
	taskBody := "custom task memory\n"
	if err := os.WriteFile(workflowPath, []byte(workflowBody), 0o600); err != nil {
		t.Fatalf("write workflow memory: %v", err)
	}
	if err := os.WriteFile(taskPath, []byte(taskBody), 0o600); err != nil {
		t.Fatalf("write task memory: %v", err)
	}

	ctx, err := Prepare(tasksDir, "task_02.md")
	if err != nil {
		t.Fatalf("prepare workflow memory: %v", err)
	}

	gotWorkflow, err := os.ReadFile(ctx.Workflow.Path)
	if err != nil {
		t.Fatalf("read workflow memory: %v", err)
	}
	if string(gotWorkflow) != workflowBody {
		t.Fatalf("expected workflow memory to remain unchanged\nwant: %q\ngot:  %q", workflowBody, string(gotWorkflow))
	}

	gotTask, err := os.ReadFile(ctx.Task.Path)
	if err != nil {
		t.Fatalf("read task memory: %v", err)
	}
	if string(gotTask) != taskBody {
		t.Fatalf("expected task memory to remain unchanged\nwant: %q\ngot:  %q", taskBody, string(gotTask))
	}
}

func TestPrepareFlagsCompactionForOversizedFiles(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	memoryDir := filepath.Join(tasksDir, DirName)
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}

	workflowBody := strings.Repeat("workflow line\n", workflowLineLimit+1)
	taskBody := strings.Repeat("task line\n", taskLineLimit+1)
	if err := os.WriteFile(filepath.Join(memoryDir, WorkflowFileName), []byte(workflowBody), 0o600); err != nil {
		t.Fatalf("write workflow memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "task_09.md"), []byte(taskBody), 0o600); err != nil {
		t.Fatalf("write task memory: %v", err)
	}

	ctx, err := Prepare(tasksDir, "task_09.md")
	if err != nil {
		t.Fatalf("prepare workflow memory: %v", err)
	}
	if !ctx.Workflow.NeedsCompaction {
		t.Fatalf("expected workflow memory to require compaction")
	}
	if !ctx.Task.NeedsCompaction {
		t.Fatalf("expected task memory to require compaction")
	}
}
