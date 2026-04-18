package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/subprocess"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

type hostRuntime struct {
	root      string
	bus       *events.Bus[events.Event]
	journal   *journal.Journal
	router    *HostAPIRouter
	extension *RuntimeExtension
	runID     string
}

func newHostRuntime(
	t *testing.T,
	capabilities []Capability,
	dispatcher *kernel.Dispatcher,
	parentRunID string,
) *hostRuntime {
	t.Helper()
	return newHostRuntimeWithOptions(t, capabilities, dispatcher, parentRunID, "run-host-root", nil, nil)
}

func newHostRuntimeWithRunID(
	t *testing.T,
	capabilities []Capability,
	dispatcher *kernel.Dispatcher,
	parentRunID string,
	runID string,
) *hostRuntime {
	t.Helper()
	return newHostRuntimeWithOptions(t, capabilities, dispatcher, parentRunID, runID, nil, nil)
}

func newHostRuntimeWithRunIDAndManager(
	t *testing.T,
	capabilities []Capability,
	dispatcher *kernel.Dispatcher,
	parentRunID string,
	runID string,
	runtimeManager model.RuntimeManager,
) *hostRuntime {
	t.Helper()
	return newHostRuntimeWithOptions(t, capabilities, dispatcher, parentRunID, runID, runtimeManager, nil)
}

func newHostRuntimeWithDaemonBridge(
	t *testing.T,
	capabilities []Capability,
	dispatcher *kernel.Dispatcher,
	parentRunID string,
	runID string,
	daemonBridge DaemonHostBridge,
) *hostRuntime {
	t.Helper()
	return newHostRuntimeWithOptions(t, capabilities, dispatcher, parentRunID, runID, nil, daemonBridge)
}

func newHostRuntimeWithOptions(
	t *testing.T,
	capabilities []Capability,
	dispatcher *kernel.Dispatcher,
	parentRunID string,
	runID string,
	runtimeManager model.RuntimeManager,
	daemonBridge DaemonHostBridge,
) *hostRuntime {
	t.Helper()

	root := t.TempDir()
	runArtifacts := model.NewRunArtifacts(root, runID)
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(run dir) error = %v", err)
	}

	bus := events.New[events.Event](32)
	j, err := journal.Open(runArtifacts.EventsPath, bus, 32)
	if err != nil {
		t.Fatalf("journal.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = j.Close(context.Background())
		_ = bus.Close(context.Background())
	})

	ops, err := NewDefaultKernelOps(DefaultKernelOpsConfig{
		WorkspaceRoot:  root,
		RunID:          runID,
		ParentRunID:    parentRunID,
		Dispatcher:     dispatcher,
		EventBus:       bus,
		Journal:        j,
		RuntimeManager: runtimeManager,
		DaemonBridge:   daemonBridge,
	})
	if err != nil {
		t.Fatalf("NewDefaultKernelOps() error = %v", err)
	}

	extension := newHostAPIExtension("ext", capabilities, ExtensionStateReady)
	router := NewHostAPIRouter(mustRegistry(t, extension), &auditSpy{})
	if err := RegisterHostServices(router, ops); err != nil {
		t.Fatalf("RegisterHostServices() error = %v", err)
	}

	return &hostRuntime{
		root:      root,
		bus:       bus,
		journal:   j,
		router:    router,
		extension: extension,
		runID:     runID,
	}
}

type stubDaemonHostBridge struct {
	token     string
	runtime   *model.RuntimeConfig
	runHandle *RunHandle
	startErr  error
}

func (b *stubDaemonHostBridge) HostCapabilityToken() string {
	if b == nil {
		return ""
	}
	return strings.TrimSpace(b.token)
}

func (b *stubDaemonHostBridge) StartRun(
	_ context.Context,
	runtimeCfg *model.RuntimeConfig,
) (*RunHandle, error) {
	if b == nil {
		return nil, errors.New("missing daemon host bridge")
	}
	if runtimeCfg != nil {
		b.runtime = runtimeCfg.Clone()
	}
	if b.startErr != nil {
		return nil, b.startErr
	}
	if b.runHandle != nil {
		handle := *b.runHandle
		return &handle, nil
	}
	return &RunHandle{RunID: "daemon-run", ParentRunID: strings.TrimSpace(runtimeCfg.ParentRunID)}, nil
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return payload
}

func writeTaskFixture(
	t *testing.T,
	root string,
	workflow string,
	number int,
	status string,
	title string,
	taskType string,
	body string,
) string {
	t.Helper()

	tasksDir := model.TaskDirectoryForWorkspace(root, workflow)
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(tasks dir) error = %v", err)
	}

	content, err := frontmatter.Format(model.TaskFileMeta{
		Status:       status,
		Title:        title,
		TaskType:     taskType,
		Dependencies: []string{},
	}, strings.TrimSpace(body)+"\n")
	if err != nil {
		t.Fatalf("frontmatter.Format() error = %v", err)
	}

	path := filepath.Join(tasksDir, filepath.Base(filepath.Join(tasksDir, taskFileName(number))))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
	return path
}

func taskFileName(number int) string {
	return fmt.Sprintf("task_%02d.md", number)
}

func awaitEvent(t *testing.T, ch <-chan events.Event, kind events.EventKind) events.Event {
	t.Helper()

	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case evt := <-ch:
			if evt.Kind == kind {
				return evt
			}
		case <-timeout.C:
			t.Fatalf("timed out waiting for event kind %q", kind)
		}
	}
}

func closeJournalAndWait(t *testing.T, j *journal.Journal) {
	t.Helper()
	if err := j.Close(context.Background()); err != nil {
		t.Fatalf("journal.Close() error = %v", err)
	}
}

func assertRequestErrorReason(t *testing.T, err error, wantCode int, wantReason string) map[string]any {
	t.Helper()

	var requestErr *subprocess.RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("error type = %T, want *subprocess.RequestError", err)
	}
	if requestErr.Code != wantCode {
		t.Fatalf("request error code = %d, want %d", requestErr.Code, wantCode)
	}

	data, ok := requestErr.Data.(map[string]any)
	if !ok {
		t.Fatalf("request error data type = %T, want map[string]any", requestErr.Data)
	}
	if got := data["reason"]; got != wantReason {
		t.Fatalf("request error reason = %#v, want %q", got, wantReason)
	}
	return data
}

func TestRegisterHostServicesRegistersAllNamespaces(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityEventsRead}, nil, "")
	for _, namespace := range []string{"tasks", "runs", "memory", "artifacts", "prompts", "events"} {
		if _, ok := rt.router.service(namespace); !ok {
			t.Fatalf("router.service(%q) found = false, want true", namespace)
		}
	}
}

func TestHostPromptsRenderReturnsRenderedPrompt(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, nil, nil, "")
	result, err := rt.router.Handle(context.Background(), "ext", "host.prompts.render", mustJSON(t, PromptRenderRequest{
		Template: hostPromptTemplateSystemAddendum,
		Params: PromptRenderParams{
			Mode: model.ExecutionModePRDTasks,
			Memory: &PromptMemoryContext{
				WorkflowPath: "/repo/.compozy/tasks/extensibility/memory/MEMORY.md",
				TaskPath:     "/repo/.compozy/tasks/extensibility/memory/task_06.md",
			},
		},
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	rendered, ok := result.(*PromptRenderResult)
	if !ok {
		t.Fatalf("result type = %T, want *PromptRenderResult", result)
	}
	if !strings.Contains(rendered.Rendered, "<workflow_memory>") {
		t.Fatalf("rendered prompt = %q, want workflow memory block", rendered.Rendered)
	}
}

func TestHostPromptsRenderBuildTemplate(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, nil, nil, "")
	result, err := rt.router.Handle(context.Background(), "ext", "host.prompts.render", mustJSON(t, PromptRenderRequest{
		Template: hostPromptTemplateBuild,
		Params: PromptRenderParams{
			Mode:       model.ExecutionModePRDTasks,
			AutoCommit: true,
			BatchGroups: map[string][]PromptIssueRef{
				"task_06": {
					{
						Name:    "task_06.md",
						AbsPath: filepath.Join(rt.root, ".compozy", "tasks", "extensibility", "task_06.md"),
						Content: strings.Join([]string{
							"---",
							"status: pending",
							"title: Host API services",
							"type: backend",
							"complexity: high",
							"dependencies:",
							"  - task_05.md",
							"---",
							"",
							"# Task 06: Host API services",
							"",
							"Implement the Host API layer.",
						}, "\n"),
					},
				},
			},
		},
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	rendered, ok := result.(*PromptRenderResult)
	if !ok {
		t.Fatalf("result type = %T, want *PromptRenderResult", result)
	}
	if !strings.Contains(rendered.Rendered, "# Implementation Task: task_06.md") {
		t.Fatalf("rendered prompt = %q, want task header", rendered.Rendered)
	}
	if !strings.Contains(rendered.Rendered, "- **Dependencies**: task_05.md") {
		t.Fatalf("rendered prompt = %q, want dependency list", rendered.Rendered)
	}
}

func TestHostPromptsRenderRejectsUnknownTemplate(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, nil, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.prompts.render", mustJSON(t, PromptRenderRequest{
		Template: "unknown",
	}))
	assertRequestErrorCode(t, err, -32601)
}

func TestHostPromptsUnknownVerbReturnsMethodNotFound(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, nil, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.prompts.preview", mustJSON(t, map[string]any{}))
	assertRequestErrorCode(t, err, -32601)
}

func TestHostEventsSubscribeReturnsSubscriptionAndStoresFilter(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityEventsRead}, nil, "")
	result, err := rt.router.Handle(
		context.Background(),
		"ext",
		"host.events.subscribe",
		mustJSON(t, EventSubscribeRequest{
			Kinds: []events.EventKind{events.EventKindRunCompleted, events.EventKindJobFailed},
		}),
	)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	response, ok := result.(*EventSubscribeResult)
	if !ok {
		t.Fatalf("result type = %T, want *EventSubscribeResult", result)
	}
	if response.SubscriptionID == "" {
		t.Fatal("subscription id is empty")
	}

	gotID, gotKinds := rt.extension.EventSubscription()
	if gotID != response.SubscriptionID {
		t.Fatalf("stored subscription id = %q, want %q", gotID, response.SubscriptionID)
	}
	if len(gotKinds) != 2 || gotKinds[0] != events.EventKindRunCompleted || gotKinds[1] != events.EventKindJobFailed {
		t.Fatalf("stored filter = %#v, want two explicit kinds", gotKinds)
	}
}

func TestHostEventsPublishEmitsExtensionEvent(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityEventsPublish}, nil, "")
	_, ch, _ := rt.bus.Subscribe()

	result, err := rt.router.Handle(context.Background(), "ext", "host.events.publish", mustJSON(t, EventPublishRequest{
		Kind:    "custom.signal",
		Payload: json.RawMessage(`{"ok":true}`),
	}))
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	response, ok := result.(*EventPublishResult)
	if !ok {
		t.Fatalf("result type = %T, want *EventPublishResult", result)
	}
	if response.Seq == 0 {
		t.Fatal("response.Seq = 0, want assigned sequence")
	}

	closeJournalAndWait(t, rt.journal)
	event := awaitEvent(t, ch, events.EventKindExtensionEvent)

	var payload kinds.ExtensionEventPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal(event.Payload) error = %v", err)
	}
	if payload.Extension != "ext" {
		t.Fatalf("payload.Extension = %q, want %q", payload.Extension, "ext")
	}
	if payload.Kind != "custom.signal" {
		t.Fatalf("payload.Kind = %q, want %q", payload.Kind, "custom.signal")
	}
}

func TestHostEventsPublishRejectsMissingKind(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityEventsPublish}, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.events.publish", mustJSON(t, EventPublishRequest{}))
	assertRequestErrorCode(t, err, -32602)
}

func TestRuntimeExtensionEventFiltersAndNameFallbacks(t *testing.T) {
	t.Parallel()

	extension := &RuntimeExtension{
		Ref:      Ref{Name: "ref-name"},
		Manifest: &Manifest{Extension: ExtensionInfo{Name: "manifest-name"}},
	}
	if got := extension.normalizedName(); got != "ref-name" {
		t.Fatalf("normalizedName() = %q, want %q", got, "ref-name")
	}
	extension.Name = "runtime-name"
	if got := extension.normalizedName(); got != "runtime-name" {
		t.Fatalf("normalizedName() with runtime name = %q, want %q", got, "runtime-name")
	}

	if !extension.WantsEvent(events.EventKindRunCompleted) {
		t.Fatal("WantsEvent() without explicit filter = false, want true")
	}

	extension.SetEventSubscription("sub-1", []events.EventKind{events.EventKindJobCompleted})
	if !extension.WantsEvent(events.EventKindJobCompleted) {
		t.Fatal("WantsEvent(job.completed) = false, want true")
	}
	if extension.WantsEvent(events.EventKindRunCompleted) {
		t.Fatal("WantsEvent(run.completed) = true, want false")
	}
}

func TestDefaultKernelOpsSubmitRuntimeEventPublishesToBus(t *testing.T) {
	t.Parallel()

	bus := events.New[events.Event](4)
	t.Cleanup(func() {
		_ = bus.Close(context.Background())
	})

	ops := &defaultKernelOps{
		runID:    "run-test",
		eventBus: bus,
	}
	_, ch, _ := bus.Subscribe()

	seq, err := ops.submitRuntimeEvent(
		context.Background(),
		events.EventKindExtensionEvent,
		kinds.ExtensionEventPayload{
			Extension: "ext",
			Kind:      "custom.signal",
		},
	)
	if err != nil {
		t.Fatalf("submitRuntimeEvent() error = %v", err)
	}
	if seq != 0 {
		t.Fatalf("submitRuntimeEvent() seq = %d, want 0 for bus-only publish", seq)
	}

	event := awaitEvent(t, ch, events.EventKindExtensionEvent)
	if event.RunID != "run-test" {
		t.Fatalf("event.RunID = %q, want %q", event.RunID, "run-test")
	}
}

func TestDefaultKernelOpsSubmitRuntimeEventRejectsNilReceiver(t *testing.T) {
	t.Parallel()

	var ops *defaultKernelOps
	if _, err := ops.submitRuntimeEvent(
		context.Background(),
		events.EventKindExtensionEvent,
		kinds.ExtensionEventPayload{},
	); err == nil {
		t.Fatal("submitRuntimeEvent(nil) error = nil, want failure")
	}
}

func TestDefaultKernelOpsTasksDirForWorkflowValidation(t *testing.T) {
	t.Parallel()

	ops := &defaultKernelOps{workspaceRoot: t.TempDir()}

	_, err := ops.tasksDirForWorkflow("")
	assertRequestErrorCode(t, err, -32602)

	_, err = ops.tasksDirForWorkflow("nested/workflow")
	assertRequestErrorCode(t, err, -32602)

	got, err := ops.tasksDirForWorkflow("extensibility")
	if err != nil {
		t.Fatalf("tasksDirForWorkflow(valid) error = %v", err)
	}
	if want := filepath.Join(ops.workspaceRoot, ".compozy", "tasks", "extensibility"); got != want {
		t.Fatalf("tasksDirForWorkflow(valid) = %q, want %q", got, want)
	}
}

func TestDefaultKernelOpsWorkspaceRelativeAndPathWithinRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ops := &defaultKernelOps{workspaceRoot: root}
	if got := ops.workspaceRelative(root); got != "." {
		t.Fatalf("workspaceRelative(root) = %q, want %q", got, ".")
	}

	nested := filepath.Join(root, ".compozy", "tasks", "extensibility", "task_06.md")
	if got := ops.workspaceRelative(nested); got != ".compozy/tasks/extensibility/task_06.md" {
		t.Fatalf("workspaceRelative(nested) = %q, want .compozy/tasks/extensibility/task_06.md", got)
	}

	if !pathWithinRoot(nested, root) {
		t.Fatal("pathWithinRoot(nested, root) = false, want true")
	}
	if pathWithinRoot(filepath.Join(root, "..", "escape.txt"), root) {
		t.Fatal("pathWithinRoot(escape, root) = true, want false")
	}
}

func TestRegisterHostServicesWorksWithRouterCapabilityChecks(t *testing.T) {
	t.Parallel()

	rt := newHostRuntime(t, []Capability{CapabilityTasksRead}, nil, "")
	_, err := rt.router.Handle(context.Background(), "ext", "host.tasks.create", mustJSON(t, TaskCreateRequest{
		Workflow: "extensibility",
		Title:    "Blocked create",
		Body:     "Blocked body",
		Frontmatter: TaskFrontmatter{
			Status: "pending",
			Type:   "chore",
		},
	}))
	assertRequestErrorCode(t, err, capabilityDeniedCode)
}
