package extension_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	extension "github.com/compozy/compozy/sdk/extension"
	exttesting "github.com/compozy/compozy/sdk/extension/testing"
)

func TestHostAPITasksCreateRoundTripsMethodAndParams(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version).WithCapabilities(extension.CapabilityTasksCreate)
	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: []extension.Capability{extension.CapabilityTasksCreate},
	})
	defer cancel()

	harness.HandleHostMethod("host.tasks.create", func(_ context.Context, params json.RawMessage) (any, error) {
		var req extension.TaskCreateRequest
		if err := json.Unmarshal(params, &req); err != nil {
			t.Fatalf("unmarshal params: %v", err)
		}
		if req.Workflow != "demo" || req.Title != "Hello" {
			t.Fatalf("unexpected create request: %#v", req)
		}
		if !req.UpdateIndex {
			t.Fatalf("req.UpdateIndex = false, want true")
		}
		return extension.Task{
			Workflow: "demo",
			Number:   7,
			Path:     ".compozy/tasks/demo/task_07.md",
			Status:   "pending",
		}, nil
	})

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	task, err := ext.Host().Tasks.Create(ctx, extension.TaskCreateRequest{
		Workflow: "demo",
		Title:    "Hello",
		Body:     "Body",
		Frontmatter: extension.TaskFrontmatter{
			Status: "pending",
			Type:   "backend",
		},
		UpdateIndex: true,
	})
	if err != nil {
		t.Fatalf("Tasks.Create() error = %v", err)
	}
	if task.Number != 7 {
		t.Fatalf("task.Number = %d, want 7", task.Number)
	}

	waitForHostMethod(t, harness, "host.tasks.create")
	shutdownHarness(ctx, t, harness, errCh)
}

func TestHostAPIRunsStartReturnsRunHandle(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version).WithCapabilities(extension.CapabilityRunsStart)
	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: []extension.Capability{extension.CapabilityRunsStart},
	})
	defer cancel()

	harness.HandleHostMethod("host.runs.start", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.RunHandle{
			RunID:       "run-child-001",
			ParentRunID: "run-parent-001",
		}, nil
	})

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	handle, err := ext.Host().Runs.Start(ctx, extension.RunStartRequest{
		Runtime: extension.RunConfig{Name: "child"},
	})
	if err != nil {
		t.Fatalf("Runs.Start() error = %v", err)
	}
	if handle.RunID != "run-child-001" || handle.ParentRunID != "run-parent-001" {
		t.Fatalf("handle = %#v, want run ids", handle)
	}

	shutdownHarness(ctx, t, harness, errCh)
}

func TestHostAPIMemoryReadReturnsAbsentDocument(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version).WithCapabilities(extension.CapabilityMemoryRead)
	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: []extension.Capability{extension.CapabilityMemoryRead},
	})
	defer cancel()

	harness.HandleHostMethod("host.memory.read", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.MemoryReadResult{
			Path:            ".compozy/tasks/demo/memory/task_03.md",
			Content:         "",
			Exists:          false,
			NeedsCompaction: false,
		}, nil
	})

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	result, err := ext.Host().Memory.Read(ctx, extension.MemoryReadRequest{
		Workflow: "demo",
		TaskFile: "task_03.md",
	})
	if err != nil {
		t.Fatalf("Memory.Read() error = %v", err)
	}
	if result.Exists || result.Content != "" {
		t.Fatalf("result = %#v, want exists=false content=\"\"", result)
	}

	shutdownHarness(ctx, t, harness, errCh)
}

func TestHostAPIArtifactsWriteReturnsPathOutOfScope(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version).WithCapabilities(extension.CapabilityArtifactsWrite)
	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: []extension.Capability{extension.CapabilityArtifactsWrite},
	})
	defer cancel()

	harness.HandleHostMethod("host.artifacts.write", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, &extension.Error{
			Code:    -32001,
			Message: "Capability denied",
			Data:    extension.MustJSON(map[string]any{"reason": "path_out_of_scope"}),
		}
	})

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	_, err := ext.Host().Artifacts.Write(ctx, extension.ArtifactWriteRequest{
		Path:    "../bad.txt",
		Content: []byte("nope"),
	})
	requestErr := assertRPCErrorCode(t, err, -32001)
	var data struct {
		Reason string `json:"reason"`
	}
	if err := requestErr.DecodeData(&data); err != nil {
		t.Fatalf("DecodeData() error = %v", err)
	}
	if data.Reason != "path_out_of_scope" {
		t.Fatalf("reason = %q, want path_out_of_scope", data.Reason)
	}

	shutdownHarness(ctx, t, harness, errCh)
}

func TestHostAPICallCorrelatesOutOfOrderResponses(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version).
		WithCapabilities(extension.CapabilityTasksRead, extension.CapabilityMemoryRead)
	extensionSide, hostSide := exttesting.NewMockTransportPair()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ext.WithTransport(extensionSide).Start(ctx)
	}()

	if err := hostSide.WriteMessage(extension.Message{
		ID:     json.RawMessage("1"),
		Method: "initialize",
		Params: extension.MustJSON(extension.InitializeRequest{
			ProtocolVersion:           extension.ProtocolVersion,
			SupportedProtocolVersions: []string{extension.ProtocolVersion},
			CompozyVersion:            "dev",
			Extension: extension.InitializeRequestIdentity{
				Name:    name,
				Version: version,
				Source:  "workspace",
			},
			GrantedCapabilities: []extension.Capability{
				extension.CapabilityTasksRead,
				extension.CapabilityMemoryRead,
			},
			Runtime: extension.InitializeRuntime{
				RunID:                "run-001",
				WorkspaceRoot:        ".",
				InvokingCommand:      "start",
				ShutdownTimeoutMS:    1000,
				DefaultHookTimeoutMS: 5000,
			},
		}),
	}); err != nil {
		t.Fatalf("write initialize: %v", err)
	}

	initializeResponse, err := hostSide.Receive(ctx)
	if err != nil {
		t.Fatalf("receive initialize response: %v", err)
	}
	if initializeResponse.Error != nil {
		t.Fatalf("initialize error = %v", initializeResponse.Error)
	}

	taskCh := make(chan error, 1)
	memCh := make(chan error, 1)
	var (
		listResult []extension.Task
		memResult  *extension.MemoryReadResult
	)

	go func() {
		result, err := ext.Host().Tasks.List(ctx, extension.TaskListRequest{Workflow: "demo"})
		listResult = result
		taskCh <- err
	}()
	go func() {
		result, err := ext.Host().Memory.Read(ctx, extension.MemoryReadRequest{Workflow: "demo"})
		memResult = result
		memCh <- err
	}()

	first, err := hostSide.Receive(ctx)
	if err != nil {
		t.Fatalf("receive first host call: %v", err)
	}
	second, err := hostSide.Receive(ctx)
	if err != nil {
		t.Fatalf("receive second host call: %v", err)
	}
	if first.Method == second.Method {
		t.Fatalf("expected distinct methods, got %q and %q", first.Method, second.Method)
	}

	requests := map[string]json.RawMessage{
		first.Method:  append(json.RawMessage(nil), first.ID...),
		second.Method: append(json.RawMessage(nil), second.ID...),
	}

	if err := hostSide.WriteMessage(extension.Message{
		ID: requests["host.memory.read"],
		Result: extension.MustJSON(
			extension.MemoryReadResult{Path: ".compozy/tasks/demo/memory/MEMORY.md", Exists: false},
		),
	}); err != nil {
		t.Fatalf("write second response: %v", err)
	}
	if err := hostSide.WriteMessage(extension.Message{
		ID:     requests["host.tasks.list"],
		Result: extension.MustJSON([]extension.Task{{Workflow: "demo", Number: 1}}),
	}); err != nil {
		t.Fatalf("write first response: %v", err)
	}

	if err := <-taskCh; err != nil {
		t.Fatalf("Tasks.List() error = %v", err)
	}
	if err := <-memCh; err != nil {
		t.Fatalf("Memory.Read() error = %v", err)
	}
	if len(listResult) != 1 || listResult[0].Number != 1 {
		t.Fatalf("listResult = %#v, want one task", listResult)
	}
	if memResult == nil || memResult.Exists {
		t.Fatalf("memResult = %#v, want exists=false", memResult)
	}

	if err := hostSide.WriteMessage(extension.Message{
		ID:     json.RawMessage("99"),
		Method: "shutdown",
		Params: extension.MustJSON(extension.ShutdownRequest{Reason: "run_completed", DeadlineMS: 1000}),
	}); err != nil {
		t.Fatalf("write shutdown: %v", err)
	}
	shutdownResponse, err := hostSide.Receive(ctx)
	if err != nil {
		t.Fatalf("receive shutdown response: %v", err)
	}
	if shutdownResponse.Error != nil {
		t.Fatalf("shutdown error = %v", shutdownResponse.Error)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() terminal error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for extension shutdown")
	}
}

func TestHostAPICallBeforeInitializeFails(t *testing.T) {
	t.Parallel()

	ext := extension.New("sdk-ext", "1.0.0")
	_, err := ext.Host().Tasks.Create(context.Background(), extension.TaskCreateRequest{})
	assertRPCErrorCode(t, err, -32003)
}

func TestHostAPICallWithoutAcceptedCapabilityFails(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version)
	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{})
	defer cancel()

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	_, err := ext.Host().Tasks.Create(ctx, extension.TaskCreateRequest{})
	assertRPCErrorCode(t, err, -32001)

	shutdownHarness(ctx, t, harness, errCh)
}
