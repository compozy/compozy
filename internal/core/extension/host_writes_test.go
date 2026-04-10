package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/memory"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type runStartCaptureHandler struct {
	commands []commands.RunStartCommand
}

func (h *runStartCaptureHandler) Handle(
	_ context.Context,
	cmd commands.RunStartCommand,
) (commands.RunStartResult, error) {
	h.commands = append(h.commands, cmd)
	return commands.RunStartResult{
		RunID:        cmd.Runtime.RunID,
		ArtifactsDir: model.NewRunArtifacts(cmd.Runtime.WorkspaceRoot, cmd.Runtime.RunID).RunDir,
		Status:       "succeeded",
	}, nil
}

func testRunStartDispatcher(t *testing.T) (*kernel.Dispatcher, *runStartCaptureHandler) {
	t.Helper()

	dispatcher := kernel.NewDispatcher()
	handler := &runStartCaptureHandler{}
	kernel.Register(dispatcher, handler)
	return dispatcher, handler
}

func TestHostTasksCreateReturnsSequentialNumberAndEmitsTaskFileUpdated(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksCreate}, nil, "")
	writeTaskFixture(
		t,
		rt.root,
		"extensibility",
		1,
		"pending",
		"Existing task",
		"chore",
		"# Task 01: Existing task\n\nExisting body.\n",
	)
	_, ch, _ := rt.bus.Subscribe()

	result, err := rt.router.Handle(context.Background(), "ext", "host.tasks.create", mustJSON(t, TaskCreateRequest{
		Workflow: "extensibility",
		Title:    "Post-run verification",
		Body:     "Run make verify and attach the output.",
		Frontmatter: TaskFrontmatter{
			Status: "pending",
			Type:   "chore",
		},
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	task, ok := result.(*Task)
	if !ok {
		t.Fatalf("result type = %T, want *Task", result)
	}
	if task.Number != 2 {
		t.Fatalf("task.Number = %d, want 2", task.Number)
	}
	if task.Status != "pending" {
		t.Fatalf("task.Status = %q, want %q", task.Status, "pending")
	}
	if task.Path != ".compozy/tasks/extensibility/task_02.md" {
		t.Fatalf("task.Path = %q, want .compozy/tasks/extensibility/task_02.md", task.Path)
	}

	closeJournalAndWait(t, rt.journal)
	event := awaitEvent(t, ch, events.EventKindTaskFileUpdated)

	var payload kinds.TaskFileUpdatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal(event.Payload) error = %v", err)
	}
	if payload.TaskName != "task_02.md" {
		t.Fatalf("payload.TaskName = %q, want %q", payload.TaskName, "task_02.md")
	}
}

func TestHostTasksCreateNormalizesFrontmatterAndTaskBody(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksCreate}, nil, "")
	result, err := rt.router.Handle(context.Background(), "ext", "host.tasks.create", mustJSON(t, TaskCreateRequest{
		Workflow: "extensibility",
		Title:    "  Normalize metadata  ",
		Body:     "  Verify trimmed metadata is persisted.  ",
		Frontmatter: TaskFrontmatter{
			Type:         "backend",
			Complexity:   "high",
			Dependencies: []string{" task_01.md ", "", "task_02.md "},
		},
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	task, ok := result.(*Task)
	if !ok {
		t.Fatalf("result type = %T, want *Task", result)
	}
	if task.Status != "pending" {
		t.Fatalf("task.Status = %q, want %q", task.Status, "pending")
	}
	if task.Complexity != "high" {
		t.Fatalf("task.Complexity = %q, want %q", task.Complexity, "high")
	}
	if len(task.Dependencies) != 2 || task.Dependencies[0] != "task_01.md" || task.Dependencies[1] != "task_02.md" {
		t.Fatalf("task.Dependencies = %#v, want trimmed dependencies", task.Dependencies)
	}

	content, err := os.ReadFile(filepath.Join(rt.root, ".compozy", "tasks", "extensibility", "task_01.md"))
	if err != nil {
		t.Fatalf("ReadFile(created task) error = %v", err)
	}
	if !strings.Contains(string(content), "# Task 01: Normalize metadata\n\nVerify trimmed metadata is persisted.\n") {
		t.Fatalf("created task content = %q, want normalized task body", string(content))
	}
}

func TestHostTasksCreateRejectsInvalidFrontmatter(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksCreate}, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.tasks.create", mustJSON(t, TaskCreateRequest{
		Workflow: "extensibility",
		Title:    "Reject invalid status",
		Body:     "Do not persist this task.",
		Frontmatter: TaskFrontmatter{
			Status: "invalid",
			Type:   "backend",
		},
	}))
	assertRequestErrorCode(t, err, -32602)
}

func TestHostTasksCreateRejectsEmptyTitle(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksCreate}, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.tasks.create", mustJSON(t, TaskCreateRequest{
		Workflow: "extensibility",
		Title:    "   ",
		Body:     "Body",
		Frontmatter: TaskFrontmatter{
			Type: "backend",
		},
	}))
	assertRequestErrorCode(t, err, -32602)
}

func TestHostRunsStartReturnsRunIDAndIncrementsParentChain(t *testing.T) {
	t.Parallel()

	dispatcher, handler := testRunStartDispatcher(t)
	rt := newHostRuntime(t, []Capability{CapabilityRunsStart}, dispatcher, "")

	result, err := rt.router.Handle(context.Background(), "ext", "host.runs.start", mustJSON(t, RunStartRequest{
		Runtime: RunConfig{
			Mode:       model.ExecutionModeExec,
			PromptText: "nested run prompt",
			IDE:        model.IDECodex,
		},
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	runHandle, ok := result.(*RunHandle)
	if !ok {
		t.Fatalf("result type = %T, want *RunHandle", result)
	}
	if runHandle.RunID == "" {
		t.Fatal("runHandle.RunID is empty")
	}
	if runHandle.ParentRunID != rt.runID {
		t.Fatalf("runHandle.ParentRunID = %q, want %q", runHandle.ParentRunID, rt.runID)
	}
	if len(handler.commands) != 1 {
		t.Fatalf("captured commands = %d, want 1", len(handler.commands))
	}
	if got := handler.commands[0].Runtime.ParentRunID; got != rt.runID {
		t.Fatalf("captured ParentRunID = %q, want %q", got, rt.runID)
	}
}

func TestHostRunsStartRejectsRecursionDepthExceeded(t *testing.T) {
	t.Parallel()

	dispatcher, _ := testRunStartDispatcher(t)
	rt := newHostRuntime(t, []Capability{CapabilityRunsStart}, dispatcher, "run-a,run-b,run-c")

	_, err := rt.router.Handle(context.Background(), "ext", "host.runs.start", mustJSON(t, RunStartRequest{
		Runtime: RunConfig{
			Mode:       model.ExecutionModeExec,
			PromptText: "nested run prompt",
		},
	}))
	data := assertRequestErrorReason(t, err, capabilityDeniedCode, "recursion_depth_exceeded")
	if got := data["depth"]; got != 3 {
		t.Fatalf("recursion depth = %#v, want 3", got)
	}
}

func TestHostMemoryWriteAppendModeAppendsWithSingleNewlineSeparator(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityMemoryWrite}, nil, "")
	memoryPath := filepath.Join(model.TaskDirectoryForWorkspace(rt.root, "extensibility"), "memory", "task_03.md")
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(memory dir) error = %v", err)
	}
	if err := os.WriteFile(memoryPath, []byte("alpha\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(memoryPath) error = %v", err)
	}

	_, err := rt.router.Handle(context.Background(), "ext", "host.memory.write", mustJSON(t, MemoryWriteRequest{
		Workflow: "extensibility",
		TaskFile: "task_03.md",
		Content:  "\nbeta\n",
		Mode:     memory.WriteModeAppend,
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	content, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("ReadFile(memoryPath) error = %v", err)
	}
	if got := string(content); got != "alpha\nbeta\n" {
		t.Fatalf("memory content = %q, want %q", got, "alpha\nbeta\n")
	}
}

func TestHostMemoryWriteReplaceEmitsTaskMemoryUpdated(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityMemoryWrite}, nil, "")
	_, ch, _ := rt.bus.Subscribe()

	_, err := rt.router.Handle(context.Background(), "ext", "host.memory.write", mustJSON(t, MemoryWriteRequest{
		Workflow: "extensibility",
		TaskFile: "task_03.md",
		Content:  "# Task Memory: task_03.md\n\nUpdated.\n",
		Mode:     memory.WriteModeReplace,
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	closeJournalAndWait(t, rt.journal)
	event := awaitEvent(t, ch, events.EventKindTaskMemoryUpdated)

	var payload kinds.TaskMemoryUpdatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal(event.Payload) error = %v", err)
	}
	if payload.TaskFile != "task_03.md" {
		t.Fatalf("payload.TaskFile = %q, want %q", payload.TaskFile, "task_03.md")
	}
}

func TestHostMemoryWriteWithoutTaskFileUsesWorkflowMemory(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityMemoryWrite}, nil, "")
	result, err := rt.router.Handle(context.Background(), "ext", "host.memory.write", mustJSON(t, MemoryWriteRequest{
		Workflow: "extensibility",
		Content:  "# Workflow Memory\n\nKeep this durable.\n",
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	response, ok := result.(*MemoryWriteResult)
	if !ok {
		t.Fatalf("result type = %T, want *MemoryWriteResult", result)
	}
	if response.Path != ".compozy/tasks/extensibility/memory/MEMORY.md" {
		t.Fatalf("response.Path = %q, want %q", response.Path, ".compozy/tasks/extensibility/memory/MEMORY.md")
	}

	content, err := os.ReadFile(filepath.Join(rt.root, ".compozy", "tasks", "extensibility", "memory", "MEMORY.md"))
	if err != nil {
		t.Fatalf("ReadFile(workflow memory) error = %v", err)
	}
	if got := string(content); got != "# Workflow Memory\n\nKeep this durable.\n" {
		t.Fatalf("workflow memory content = %q, want persisted content", got)
	}
}

func TestHostArtifactsWriteRejectsAbsolutePathOutsideWorkspace(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityArtifactsWrite}, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.artifacts.write", mustJSON(t, ArtifactWriteRequest{
		Path:    "/tmp/outside-scope.txt",
		Content: []byte("blocked"),
	}))
	data := assertRequestErrorReason(t, err, capabilityDeniedCode, "path_out_of_scope")
	if _, ok := data["allowed_roots"]; !ok {
		t.Fatal("allowed_roots missing from error data")
	}
}

func TestHostArtifactsWriteRejectsTraversal(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityArtifactsWrite}, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.artifacts.write", mustJSON(t, ArtifactWriteRequest{
		Path:    "../escape.txt",
		Content: []byte("blocked"),
	}))
	assertRequestErrorReason(t, err, capabilityDeniedCode, "path_out_of_scope")
}

func TestHostArtifactsWriteWritesScopedFileAndEmitsArtifactUpdated(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityArtifactsWrite}, nil, "")
	_, ch, _ := rt.bus.Subscribe()

	result, err := rt.router.Handle(
		context.Background(),
		"ext",
		"host.artifacts.write",
		mustJSON(t, ArtifactWriteRequest{
			Path:    ".compozy/artifacts/note.txt",
			Content: []byte("hello host api"),
		}),
	)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	response, ok := result.(*ArtifactWriteResult)
	if !ok {
		t.Fatalf("result type = %T, want *ArtifactWriteResult", result)
	}
	if response.Path != ".compozy/artifacts/note.txt" {
		t.Fatalf("response.Path = %q, want %q", response.Path, ".compozy/artifacts/note.txt")
	}
	if response.BytesWritten != len("hello host api") {
		t.Fatalf("response.BytesWritten = %d, want %d", response.BytesWritten, len("hello host api"))
	}

	content, err := os.ReadFile(filepath.Join(rt.root, ".compozy", "artifacts", "note.txt"))
	if err != nil {
		t.Fatalf("ReadFile(artifact) error = %v", err)
	}
	if got := string(content); got != "hello host api" {
		t.Fatalf("artifact content = %q, want %q", got, "hello host api")
	}

	closeJournalAndWait(t, rt.journal)
	event := awaitEvent(t, ch, events.EventKindArtifactUpdated)

	var payload kinds.ArtifactUpdatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal(event.Payload) error = %v", err)
	}
	if payload.Path != ".compozy/artifacts/note.txt" {
		t.Fatalf("payload.Path = %q, want %q", payload.Path, ".compozy/artifacts/note.txt")
	}
}

func TestHostTasksCreateThenGetReturnsCreatedTaskContent(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksCreate, CapabilityTasksRead}, nil, "")

	createResult, err := rt.router.Handle(
		context.Background(),
		"ext",
		"host.tasks.create",
		mustJSON(t, TaskCreateRequest{
			Workflow: "extensibility",
			Title:    "Generated follow-up",
			Body:     "Capture the verification output.",
			Frontmatter: TaskFrontmatter{
				Status: "pending",
				Type:   "backend",
			},
		}),
	)
	if err != nil {
		t.Fatalf("create Handle() error = %v", err)
	}
	created := createResult.(*Task)
	closeJournalAndWait(t, rt.journal)

	getResult, err := rt.router.Handle(context.Background(), "ext", "host.tasks.get", mustJSON(t, struct {
		Workflow string `json:"workflow"`
		Number   int    `json:"number"`
	}{
		Workflow: "extensibility",
		Number:   created.Number,
	}))
	if err != nil {
		t.Fatalf("get Handle() error = %v", err)
	}

	got := getResult.(*Task)
	if got.Title != "Generated follow-up" {
		t.Fatalf("got.Title = %q, want %q", got.Title, "Generated follow-up")
	}
	if !strings.Contains(got.Body, "Capture the verification output.") {
		t.Fatalf("got.Body = %q, want created task content", got.Body)
	}
}

func TestHostRunsStartRecursionGuardAllowsThreeNestedRunsThenRejectsFourth(t *testing.T) {
	t.Parallel()

	dispatcher, _ := testRunStartDispatcher(t)

	rt1 := newHostRuntime(t, []Capability{CapabilityRunsStart}, dispatcher, "")
	res1, err := rt1.router.Handle(context.Background(), "ext", "host.runs.start", mustJSON(t, RunStartRequest{
		Runtime: RunConfig{Mode: model.ExecutionModeExec, PromptText: "one"},
	}))
	if err != nil {
		t.Fatalf("level 1 Handle() error = %v", err)
	}
	run1 := res1.(*RunHandle)

	rt2 := newHostRuntimeWithRunID(t, []Capability{CapabilityRunsStart}, dispatcher, run1.ParentRunID, run1.RunID)
	res2, err := rt2.router.Handle(context.Background(), "ext", "host.runs.start", mustJSON(t, RunStartRequest{
		Runtime: RunConfig{Mode: model.ExecutionModeExec, PromptText: "two"},
	}))
	if err != nil {
		t.Fatalf("level 2 Handle() error = %v", err)
	}
	run2 := res2.(*RunHandle)

	rt3 := newHostRuntimeWithRunID(t, []Capability{CapabilityRunsStart}, dispatcher, run2.ParentRunID, run2.RunID)
	res3, err := rt3.router.Handle(context.Background(), "ext", "host.runs.start", mustJSON(t, RunStartRequest{
		Runtime: RunConfig{Mode: model.ExecutionModeExec, PromptText: "three"},
	}))
	if err != nil {
		t.Fatalf("level 3 Handle() error = %v", err)
	}
	run3 := res3.(*RunHandle)

	rt4 := newHostRuntimeWithRunID(t, []Capability{CapabilityRunsStart}, dispatcher, run3.ParentRunID, run3.RunID)
	_, err = rt4.router.Handle(context.Background(), "ext", "host.runs.start", mustJSON(t, RunStartRequest{
		Runtime: RunConfig{Mode: model.ExecutionModeExec, PromptText: "four"},
	}))
	assertRequestErrorReason(t, err, capabilityDeniedCode, "recursion_depth_exceeded")
}
