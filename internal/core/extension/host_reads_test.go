package extensions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/memory"
	"github.com/compozy/compozy/internal/core/model"
)

func TestHostTasksListReturnsFilenameSortedOrder(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksRead}, nil, "")
	writeTaskFixture(
		t,
		rt.root,
		"extensibility",
		2,
		"pending",
		"Second task",
		"backend",
		"# Task 02: Second task\n\nSecond body.\n",
	)
	writeTaskFixture(
		t,
		rt.root,
		"extensibility",
		1,
		"pending",
		"First task",
		"backend",
		"# Task 01: First task\n\nFirst body.\n",
	)

	result, err := rt.router.Handle(context.Background(), "ext", "host.tasks.list", mustJSON(t, struct {
		Workflow string `json:"workflow"`
	}{Workflow: "extensibility"}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	got, ok := result.([]Task)
	if !ok {
		t.Fatalf("result type = %T, want []Task", result)
	}
	if len(got) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(got))
	}
	if got[0].Number != 1 || got[1].Number != 2 {
		t.Fatalf("task order = %#v, want numbers [1 2]", []int{got[0].Number, got[1].Number})
	}
}

func TestHostTasksListReturnsEmptyWhenWorkflowDirMissing(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksRead}, nil, "")
	result, err := rt.router.Handle(context.Background(), "ext", "host.tasks.list", mustJSON(t, struct {
		Workflow string `json:"workflow"`
	}{Workflow: "extensibility"}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	got, ok := result.([]Task)
	if !ok {
		t.Fatalf("result type = %T, want []Task", result)
	}
	if len(got) != 0 {
		t.Fatalf("len(tasks) = %d, want 0", len(got))
	}
}

func TestHostTasksGetReturnsParsedFrontmatterAndBody(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksRead}, nil, "")
	writeTaskFixture(
		t,
		rt.root,
		"extensibility",
		3,
		"pending",
		"Fetch task",
		"backend",
		"# Task 03: Fetch task\n\nBody text.\n",
	)

	result, err := rt.router.Handle(context.Background(), "ext", "host.tasks.get", mustJSON(t, struct {
		Workflow string `json:"workflow"`
		Number   int    `json:"number"`
	}{
		Workflow: "extensibility",
		Number:   3,
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	task, ok := result.(*Task)
	if !ok {
		t.Fatalf("result type = %T, want *Task", result)
	}
	if task.Title != "Fetch task" {
		t.Fatalf("task.Title = %q, want %q", task.Title, "Fetch task")
	}
	if !strings.Contains(task.Body, "Body text.") {
		t.Fatalf("task.Body = %q, want body text", task.Body)
	}
}

func TestHostTasksGetRejectsNonPositiveNumber(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksRead}, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.tasks.get", mustJSON(t, struct {
		Workflow string `json:"workflow"`
		Number   int    `json:"number"`
	}{
		Workflow: "extensibility",
		Number:   0,
	}))
	assertRequestErrorCode(t, err, -32602)
}

func TestHostMemoryReadReturnsAbsentWhenMissing(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityMemoryRead}, nil, "")
	result, err := rt.router.Handle(context.Background(), "ext", "host.memory.read", mustJSON(t, MemoryReadRequest{
		Workflow: "extensibility",
		TaskFile: "task_03.md",
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	document, ok := result.(*MemoryReadResult)
	if !ok {
		t.Fatalf("result type = %T, want *MemoryReadResult", result)
	}
	if document.Exists {
		t.Fatal("document.Exists = true, want false")
	}
	if document.Content != "" {
		t.Fatalf("document.Content = %q, want empty", document.Content)
	}
}

func TestHostMemoryReadReturnsNeedsCompactionWhenThresholdExceeded(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityMemoryRead}, nil, "")
	tasksDir := model.TaskDirectoryForWorkspace(rt.root, "extensibility")
	if err := os.MkdirAll(filepath.Join(tasksDir, memory.DirName), 0o755); err != nil {
		t.Fatalf("MkdirAll(memory dir) error = %v", err)
	}

	var builder strings.Builder
	for i := 0; i < 180; i++ {
		builder.WriteString("line\n")
	}
	if err := os.WriteFile(
		filepath.Join(tasksDir, memory.DirName, memory.WorkflowFileName),
		[]byte(builder.String()),
		0o600,
	); err != nil {
		t.Fatalf("WriteFile(workflow memory) error = %v", err)
	}

	result, err := rt.router.Handle(context.Background(), "ext", "host.memory.read", mustJSON(t, MemoryReadRequest{
		Workflow: "extensibility",
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	document := result.(*MemoryReadResult)
	if !document.NeedsCompaction {
		t.Fatal("document.NeedsCompaction = false, want true")
	}
}

func TestHostArtifactsReadReturnsBytesForScopedFile(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityArtifactsRead}, nil, "")
	artifactPath := filepath.Join(rt.root, ".compozy", "artifacts", "note.txt")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(artifact dir) error = %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile(artifactPath) error = %v", err)
	}

	result, err := rt.router.Handle(context.Background(), "ext", "host.artifacts.read", mustJSON(t, ArtifactReadRequest{
		Path: ".compozy/artifacts/note.txt",
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	artifact, ok := result.(*ArtifactReadResult)
	if !ok {
		t.Fatalf("result type = %T, want *ArtifactReadResult", result)
	}
	if got := string(artifact.Content); got != "hello" {
		t.Fatalf("artifact content = %q, want %q", got, "hello")
	}
}

func TestHostArtifactsReadRejectsTraversal(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityArtifactsRead}, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.artifacts.read", mustJSON(t, ArtifactReadRequest{
		Path: "../escape.txt",
	}))
	assertRequestErrorReason(t, err, capabilityDeniedCode, "path_out_of_scope")
}
