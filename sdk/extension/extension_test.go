package extension_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
	extension "github.com/compozy/compozy/sdk/extension"
	exttesting "github.com/compozy/compozy/sdk/extension/testing"
)

func TestExtensionStartProcessesInitializeRequest(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version).
		WithCapabilities(extension.CapabilityPromptMutate).
		OnPromptPostBuild(func(
			_ context.Context,
			_ extension.HookContext,
			_ extension.PromptPostBuildPayload,
		) (extension.PromptTextPatch, error) {
			return extension.PromptTextPatch{}, nil
		})

	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: []extension.Capability{extension.CapabilityPromptMutate},
	})
	defer cancel()

	response, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if response.ProtocolVersion != extension.ProtocolVersion {
		t.Fatalf("protocol_version = %q, want %q", response.ProtocolVersion, extension.ProtocolVersion)
	}
	if got, want := response.AcceptedCapabilities, []extension.Capability{
		extension.CapabilityPromptMutate,
	}; len(got) != len(want) ||
		got[0] != want[0] {
		t.Fatalf("accepted_capabilities = %#v, want %#v", got, want)
	}
	if got, want := response.SupportedHookEvents, []extension.HookName{
		extension.HookPromptPostBuild,
	}; len(got) != len(want) ||
		got[0] != want[0] {
		t.Fatalf("supported_hook_events = %#v, want %#v", got, want)
	}

	request, ok := ext.InitializeRequest()
	if !ok {
		t.Fatal("InitializeRequest() reported not initialized")
	}
	if request.Extension.Name != name {
		t.Fatalf("initialize extension name = %q, want %q", request.Extension.Name, name)
	}
	if got, want := ext.AcceptedCapabilities(), []extension.Capability{
		extension.CapabilityPromptMutate,
	}; len(got) != len(want) ||
		got[0] != want[0] {
		t.Fatalf("AcceptedCapabilities() = %#v, want %#v", got, want)
	}

	shutdownHarness(ctx, t, harness, errCh)
}

func TestExtensionStartRejectsUnsupportedProtocolVersion(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version)

	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		ProtocolVersion:           "9",
		SupportedProtocolVersions: []string{"9"},
	})
	defer cancel()

	_, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	})
	assertRPCErrorCode(t, err, -32602)

	runErr := waitForRunError(t, errCh)
	assertRPCErrorCode(t, runErr, -32602)
}

func TestExtensionStartRejectsMissingGrantedCapabilities(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version).WithCapabilities(extension.CapabilityPromptMutate)

	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{})
	defer cancel()

	_, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	})
	requestErr := assertRPCErrorCode(t, err, -32001)

	var data struct {
		Required []extension.Capability `json:"required"`
		Granted  []extension.Capability `json:"granted"`
	}
	if err := requestErr.DecodeData(&data); err != nil {
		t.Fatalf("DecodeData() error = %v", err)
	}
	if len(data.Required) != 1 || data.Required[0] != extension.CapabilityPromptMutate {
		t.Fatalf("required = %#v, want [prompt.mutate]", data.Required)
	}
	if len(data.Granted) != 0 {
		t.Fatalf("granted = %#v, want empty", data.Granted)
	}

	runErr := waitForRunError(t, errCh)
	assertRPCErrorCode(t, runErr, -32001)
}

func TestOnPromptPostBuildReceivesPayloadAndReturnsPatch(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	seen := make(chan extension.PromptPostBuildPayload, 1)
	ext := extension.New(name, version).
		WithCapabilities(extension.CapabilityPromptMutate).
		OnPromptPostBuild(func(
			_ context.Context,
			_ extension.HookContext,
			payload extension.PromptPostBuildPayload,
		) (extension.PromptTextPatch, error) {
			seen <- payload
			return extension.PromptTextPatch{PromptText: extension.Ptr(payload.PromptText + "\npatched")}, nil
		})

	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: []extension.Capability{extension.CapabilityPromptMutate},
	})
	defer cancel()

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	response, err := harness.DispatchHook(
		ctx,
		"hook-001",
		extension.HookInfo{
			Name:      "prompt.post_build",
			Event:     extension.HookPromptPostBuild,
			Mutable:   true,
			Required:  false,
			Priority:  500,
			TimeoutMS: 5000,
		},
		extension.PromptPostBuildPayload{
			RunID:      "run-001",
			JobID:      "job-001",
			PromptText: "hello",
			BatchParams: extension.BatchParams{
				Name: "demo",
			},
		},
	)
	if err != nil {
		t.Fatalf("DispatchHook() error = %v", err)
	}

	var patch extension.PromptTextPatch
	if err := json.Unmarshal(response.Patch, &patch); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	if patch.PromptText == nil || *patch.PromptText != "hello\npatched" {
		t.Fatalf("patch prompt_text = %#v, want %q", patch.PromptText, "hello\npatched")
	}

	select {
	case payload := <-seen:
		if payload.PromptText != "hello" {
			t.Fatalf("handler payload prompt_text = %q, want hello", payload.PromptText)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for prompt hook payload")
	}

	shutdownHarness(ctx, t, harness, errCh)
}

func TestOnEventFilterReceivesOnlyDeclaredKinds(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	const name = "sdk-ext"
	const version = "1.0.0"
	received := make([]extension.EventKind, 0)
	ext := extension.New(name, version).OnEvent(func(_ context.Context, event extension.Event) error {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, event.Kind)
		return nil
	}, events.EventKindRunCompleted)

	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: []extension.Capability{extension.CapabilityEventsRead},
	})
	defer cancel()
	harness.HandleHostMethod("host.events.subscribe", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.EventSubscribeResult{SubscriptionID: "sub-1"}, nil
	})

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	waitForHostMethod(t, harness, "host.events.subscribe")

	if err := harness.SendEvent(ctx, extension.Event{Kind: events.EventKindRunCompleted}); err != nil {
		t.Fatalf("SendEvent(run.completed) error = %v", err)
	}
	if err := harness.SendEvent(ctx, extension.Event{Kind: events.EventKindJobFailed}); err != nil {
		t.Fatalf("SendEvent(job.failed) error = %v", err)
	}

	requireEventually(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 1
	})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 || received[0] != events.EventKindRunCompleted {
		t.Fatalf("received event kinds = %#v, want [run.completed]", received)
	}

	shutdownHarness(ctx, t, harness, errCh)
}

func TestTestHarnessRunsLifecycleAndRecordsHostCalls(t *testing.T) {
	t.Parallel()

	const name = "sdk-ext"
	const version = "1.0.0"
	ext := extension.New(name, version).
		WithCapabilities(extension.CapabilityPromptMutate, extension.CapabilityTasksRead).
		OnPromptPostBuild(func(
			ctx context.Context,
			hook extension.HookContext,
			payload extension.PromptPostBuildPayload,
		) (extension.PromptTextPatch, error) {
			tasks, err := hook.Host.Tasks.List(ctx, extension.TaskListRequest{Workflow: "demo"})
			if err != nil {
				return extension.PromptTextPatch{}, err
			}
			return extension.PromptTextPatch{
				PromptText: extension.Ptr(payload.PromptText + "\ncount=" + strconv.Itoa(len(tasks))),
			}, nil
		})

	harness, ctx, cancel, errCh := runHarnessedExtension(t, ext, exttesting.HarnessOptions{
		GrantedCapabilities: []extension.Capability{extension.CapabilityPromptMutate, extension.CapabilityTasksRead},
	})
	defer cancel()

	harness.HandleHostMethod("host.tasks.list", func(_ context.Context, _ json.RawMessage) (any, error) {
		return []extension.Task{{Workflow: "demo", Number: 1}}, nil
	})

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    name,
		Version: version,
		Source:  "workspace",
	}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	response, err := harness.DispatchHook(
		ctx,
		"hook-002",
		extension.HookInfo{Name: "prompt.post_build", Event: extension.HookPromptPostBuild, Mutable: true},
		extension.PromptPostBuildPayload{PromptText: "hello"},
	)
	if err != nil {
		t.Fatalf("DispatchHook() error = %v", err)
	}

	var patch extension.PromptTextPatch
	if err := json.Unmarshal(response.Patch, &patch); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	if patch.PromptText == nil || *patch.PromptText != "hello\ncount=1" {
		t.Fatalf("patch prompt_text = %#v, want hello\\ncount=1", patch.PromptText)
	}

	waitForHostMethod(t, harness, "host.tasks.list")
	shutdownHarness(ctx, t, harness, errCh)
}

func runHarnessedExtension(
	t *testing.T,
	ext *extension.Extension,
	options exttesting.HarnessOptions,
) (*exttesting.TestHarness, context.Context, context.CancelFunc, <-chan error) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	harness := exttesting.NewTestHarness(options)
	errCh := harness.Run(ctx, ext)
	return harness, ctx, cancel, errCh
}

func shutdownHarness(
	ctx context.Context,
	t *testing.T,
	harness *exttesting.TestHarness,
	errCh <-chan error,
) {
	t.Helper()

	if _, err := harness.Shutdown(
		ctx,
		extension.ShutdownRequest{Reason: "run_completed", DeadlineMS: 1000},
	); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := waitForRunError(t, errCh); err != nil {
		t.Fatalf("Start() terminal error = %v, want nil", err)
	}
}

func waitForRunError(t *testing.T, errCh <-chan error) error {
	t.Helper()

	select {
	case err := <-errCh:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for extension start to exit")
		return nil
	}
}

func waitForHostMethod(t *testing.T, harness *exttesting.TestHarness, method string) {
	t.Helper()

	requireEventually(t, 2*time.Second, func() bool {
		for _, call := range harness.HostCalls() {
			if call.Method == method {
				return true
			}
		}
		return false
	})
}

func assertRPCErrorCode(t *testing.T, err error, code int) *extension.Error {
	t.Helper()

	if err == nil {
		t.Fatalf("expected rpc error code %d, got nil", code)
	}

	var requestErr *extension.Error
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected rpc error code %d, got %T (%v)", code, err, err)
	}
	if requestErr.Code != code {
		t.Fatalf("rpc error code = %d, want %d", requestErr.Code, code)
	}
	return requestErr
}

func requireEventually(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true before timeout")
}
