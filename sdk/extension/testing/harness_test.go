package exttesting

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/pkg/compozy/events"
	extension "github.com/compozy/compozy/sdk/extension"
)

func TestTestHarnessExercisesLifecycleAndDefaults(t *testing.T) {
	t.Parallel()

	var (
		mu         sync.Mutex
		received   []events.EventKind
		shutdownCh = make(chan extension.ShutdownRequest, 1)
	)

	ext := extension.New("sdk-ext", "1.0.0").
		WithCapabilities(extension.CapabilityTasksRead).
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
				PromptText: extension.Ptr(payload.PromptText + "\ncount=" + string(rune('0'+len(tasks)))),
			}, nil
		}).
		OnEvent(func(_ context.Context, event extension.Event) error {
			mu.Lock()
			defer mu.Unlock()
			received = append(received, event.Kind)
			return nil
		}, events.EventKindRunCompleted).
		OnHealthCheck(func(_ context.Context, _ extension.HealthCheckRequest) (extension.HealthCheckResponse, error) {
			return extension.HealthCheckResponse{
				Healthy: true,
				Message: "ok",
				Details: map[string]any{"custom": true},
			}, nil
		}).
		OnShutdown(func(_ context.Context, request extension.ShutdownRequest) error {
			shutdownCh <- request
			return nil
		})

	harness := NewTestHarness(HarnessOptions{
		GrantedCapabilities: []extension.Capability{
			extension.CapabilityPromptMutate,
			extension.CapabilityTasksRead,
			extension.CapabilityEventsRead,
		},
	})
	harness.HandleHostMethod("host.events.subscribe", func(_ context.Context, _ json.RawMessage) (any, error) {
		return extension.EventSubscribeResult{SubscriptionID: "sub-1"}, nil
	})
	harness.HandleHostMethod("host.tasks.list", func(_ context.Context, _ json.RawMessage) (any, error) {
		return []extension.Task{{Workflow: "demo", Number: 1}}, nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := harness.Run(ctx, ext)
	initializeResponse, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
		Name:    "sdk-ext",
		Version: "1.0.0",
		Source:  "workspace",
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if initializeResponse.ProtocolVersion != extension.ProtocolVersion {
		t.Fatalf("protocol_version = %q, want %q", initializeResponse.ProtocolVersion, extension.ProtocolVersion)
	}

	requireEventually(t, func() bool {
		return slices.Equal(SortedHostCalls(harness.HostCalls()), []string{"host.events.subscribe"})
	})

	health, err := harness.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if !health.Healthy || health.Message != "ok" {
		t.Fatalf("health response = %#v, want healthy custom response", health)
	}
	if got := health.Details["custom"]; got != true {
		t.Fatalf("health details custom = %#v, want true", got)
	}
	if _, ok := health.Details["active_requests"]; !ok {
		t.Fatal("health details missing active_requests")
	}

	if err := harness.SendEvent(ctx, extension.Event{Kind: events.EventKindRunCompleted}); err != nil {
		t.Fatalf("SendEvent() error = %v", err)
	}
	requireEventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 1
	})

	hookResponse, err := harness.DispatchHook(
		ctx,
		"hook-001",
		extension.HookInfo{Name: "prompt.post_build", Event: extension.HookPromptPostBuild, Mutable: true},
		extension.PromptPostBuildPayload{PromptText: "hello"},
	)
	if err != nil {
		t.Fatalf("DispatchHook() error = %v", err)
	}

	var patch extension.PromptTextPatch
	if err := json.Unmarshal(hookResponse.Patch, &patch); err != nil {
		t.Fatalf("unmarshal patch: %v", err)
	}
	if patch.PromptText == nil || *patch.PromptText != "hello\ncount=1" {
		t.Fatalf("patch prompt_text = %#v, want %q", patch.PromptText, "hello\ncount=1")
	}

	calls := SortedHostCalls(harness.HostCalls())
	if !slices.Equal(calls, []string{"host.events.subscribe", "host.tasks.list"}) {
		t.Fatalf("host calls = %#v, want subscribe then task list", calls)
	}

	if _, err := harness.Shutdown(
		ctx,
		extension.ShutdownRequest{Reason: "run_completed", DeadlineMS: 1000},
	); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	select {
	case request := <-shutdownCh:
		if request.Reason != "run_completed" {
			t.Fatalf("shutdown reason = %q, want run_completed", request.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for shutdown handler")
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() terminal error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for extension shutdown")
	}
}

func TestTestHarnessHostMethodErrorsAreSurfaced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		handlerErr error
		wantCode   int
	}{
		{
			name:       "method not found",
			handlerErr: nil,
			wantCode:   -32601,
		},
		{
			name:       "internal error",
			handlerErr: errors.New("boom"),
			wantCode:   -32603,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ext := extension.New("sdk-ext", "1.0.0").
				WithCapabilities(extension.CapabilityTasksRead).
				OnPromptPostBuild(func(
					ctx context.Context,
					hook extension.HookContext,
					_ extension.PromptPostBuildPayload,
				) (extension.PromptTextPatch, error) {
					_, err := hook.Host.Tasks.List(ctx, extension.TaskListRequest{Workflow: "demo"})
					return extension.PromptTextPatch{}, err
				})

			harness := NewTestHarness(HarnessOptions{
				GrantedCapabilities: []extension.Capability{
					extension.CapabilityPromptMutate,
					extension.CapabilityTasksRead,
				},
			})
			if tc.handlerErr != nil {
				harness.HandleHostMethod("host.tasks.list", func(_ context.Context, _ json.RawMessage) (any, error) {
					return nil, tc.handlerErr
				})
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			errCh := harness.Run(ctx, ext)
			if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{
				Name:    "sdk-ext",
				Version: "1.0.0",
				Source:  "workspace",
			}); err != nil {
				t.Fatalf("Initialize() error = %v", err)
			}

			_, err := harness.DispatchHook(
				ctx,
				"hook-002",
				extension.HookInfo{Name: "prompt.post_build", Event: extension.HookPromptPostBuild, Mutable: true},
				extension.PromptPostBuildPayload{PromptText: "hello"},
			)
			assertRPCErrorCode(t, err, tc.wantCode)

			if _, err := harness.Shutdown(
				ctx,
				extension.ShutdownRequest{Reason: "run_completed", DeadlineMS: 1000},
			); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
			select {
			case err := <-errCh:
				if err != nil {
					t.Fatalf("Run() terminal error = %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for extension shutdown")
			}
		})
	}
}

func TestTestHarnessNilMethodsReturnErrors(t *testing.T) {
	t.Parallel()

	var harness *TestHarness
	ctx := context.Background()

	if _, err := harness.Initialize(ctx, extension.InitializeRequestIdentity{}); err == nil {
		t.Fatal("Initialize() error = nil, want error")
	}
	if _, err := harness.DispatchHook(ctx, "hook", extension.HookInfo{}, nil); err == nil {
		t.Fatal("DispatchHook() error = nil, want error")
	}
	if err := harness.SendEvent(ctx, extension.Event{}); err == nil {
		t.Fatal("SendEvent() error = nil, want error")
	}
	if _, err := harness.HealthCheck(ctx); err == nil {
		t.Fatal("HealthCheck() error = nil, want error")
	}
	if _, err := harness.Shutdown(ctx, extension.ShutdownRequest{}); err == nil {
		t.Fatal("Shutdown() error = nil, want error")
	}
	if got := harness.HostCalls(); got != nil {
		t.Fatalf("HostCalls() = %#v, want nil", got)
	}
}

func assertRPCErrorCode(t *testing.T, err error, code int) {
	t.Helper()

	var requestErr *extension.Error
	if !errors.As(err, &requestErr) {
		t.Fatalf("error type = %T, want *extension.Error", err)
	}
	if requestErr.Code != code {
		t.Fatalf("rpc error code = %d, want %d", requestErr.Code, code)
	}
}

func requireEventually(t *testing.T, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true before timeout")
}
