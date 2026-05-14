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

func TestPrepareCreatesDistinctMemoryFilesForNestedRelpath(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()

	authCtx, err := Prepare(tasksDir, "features/auth/task_01.md")
	if err != nil {
		t.Fatalf("prepare auth memory: %v", err)
	}
	payCtx, err := Prepare(tasksDir, "features/payment/task_01.md")
	if err != nil {
		t.Fatalf("prepare payment memory: %v", err)
	}
	rootCtx, err := Prepare(tasksDir, "task_01.md")
	if err != nil {
		t.Fatalf("prepare root memory: %v", err)
	}

	wantAuth := filepath.Join(tasksDir, DirName, "features", "auth", "task_01.md")
	wantPay := filepath.Join(tasksDir, DirName, "features", "payment", "task_01.md")
	wantRoot := filepath.Join(tasksDir, DirName, "task_01.md")
	if authCtx.Task.Path != wantAuth {
		t.Fatalf("auth memory path = %q, want %q", authCtx.Task.Path, wantAuth)
	}
	if payCtx.Task.Path != wantPay {
		t.Fatalf("payment memory path = %q, want %q", payCtx.Task.Path, wantPay)
	}
	if rootCtx.Task.Path != wantRoot {
		t.Fatalf("root memory path = %q, want %q", rootCtx.Task.Path, wantRoot)
	}

	if authCtx.Task.Path == payCtx.Task.Path {
		t.Fatal("expected nested tasks with the same basename to use distinct memory files")
	}
	if authCtx.Task.Path == rootCtx.Task.Path {
		t.Fatal("expected nested and root tasks to use distinct memory files")
	}

	authBody, err := os.ReadFile(authCtx.Task.Path)
	if err != nil {
		t.Fatalf("read auth memory: %v", err)
	}
	if !strings.Contains(string(authBody), taskHeaderPrefix+"task_01.md") {
		t.Fatalf("expected auth memory header to use basename, got %q", string(authBody))
	}
}

func TestPrepareNormalizesBackslashRelpath(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	ctx, err := Prepare(tasksDir, `features\auth\task_01.md`)
	if err != nil {
		t.Fatalf("prepare memory: %v", err)
	}
	want := filepath.Join(tasksDir, DirName, "features", "auth", "task_01.md")
	if ctx.Task.Path != want {
		t.Fatalf("task memory path = %q, want %q", ctx.Task.Path, want)
	}
}

func TestPrepareRejectsUnsafeRelpaths(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input  string
		needle string
	}{
		{"", "task file name is required"},
		{"   ", "task file name is required"},
		{"/task_01.md", "leading slash not allowed"},
		{"../task_01.md", `".." segment not allowed`},
		{"features/../task_01.md", `".." segment not allowed`},
		{"features//task_01.md", "empty path segment"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			_, err := Prepare(t.TempDir(), tc.input)
			if err == nil {
				t.Fatalf("expected error for input %q", tc.input)
			}
			if !strings.Contains(err.Error(), tc.needle) {
				t.Fatalf("error %v should contain %q", err, tc.needle)
			}
		})
	}
}

func TestWriteDocumentCreatesNestedDirectories(t *testing.T) {
	t.Parallel()

	tasksDir := t.TempDir()
	doc, _, err := WriteDocument(tasksDir, "features/auth/task_01.md", "hello\n", WriteModeReplace)
	if err != nil {
		t.Fatalf("write document: %v", err)
	}
	want := filepath.Join(tasksDir, DirName, "features", "auth", "task_01.md")
	if doc.Path != want {
		t.Fatalf("document path = %q, want %q", doc.Path, want)
	}
	body, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read nested memory: %v", err)
	}
	if string(body) != "hello\n" {
		t.Fatalf("unexpected body: %q", string(body))
	}
}
