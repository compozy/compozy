// Suite: transient bridge control runtime
// Invariant: admitted control calls use one restricted process joined to the manager lifecycle and reap it unconditionally.
// Boundary IN: daemon-to-extension control transport.
// Boundary OUT: provider check semantics, owned by provider suites.
package extensionpkg

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	redactpkg "github.com/compozy/agh/internal/redact"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/toolruntime"
)

func TestManagerCheckBridgeUsesRestrictedTransientRuntimeAndCleansUp(t *testing.T) {
	t.Run("Should launch one restricted registered process and clean it up", func(t *testing.T) {
		t.Parallel()

		process := newFakeProcess(4101)
		process.initResp = controlInitializeResponse("telegram-adapter", bridgepkg.ControlMethodCheck)
		process.callFn = func(_ context.Context, method string, params any, result any) error {
			if got, want := method, string(bridgepkg.ControlMethodCheck); got != want {
				t.Fatalf("Call() method = %q, want %q", got, want)
			}
			request, ok := params.(bridgepkg.BridgeCheckRequest)
			if !ok || request.BridgeInstanceID != "brg-control" {
				t.Fatalf("Call() params = %#v, want bridge check request", params)
			}
			response := result.(*bridgepkg.BridgeCheckResponse)
			*response = bridgepkg.BridgeCheckResponse{
				Checks: []bridgepkg.BridgeCheckRecord{{
					Check: "provider.identity", Status: bridgepkg.BridgeCheckStatusPass,
				}},
			}
			return nil
		}
		resolver := &stubBridgeControlRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				"brg-control": controlBridgeRuntime("brg-control", bridgepkg.ControlMethodCheck),
			},
		}
		launcher := &fakeLauncher{queue: []*fakeProcess{process}}
		manager := newBridgeControlTestManager(t, launcher, resolver)

		response, err := manager.CheckBridge(
			testutil.Context(t),
			"telegram-adapter",
			bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-control"},
		)
		if err != nil {
			t.Fatalf("CheckBridge() error = %v", err)
		}
		if err := response.Validate(); err != nil {
			t.Fatalf("CheckBridge() response error = %v", err)
		}

		requests := process.initRequests()
		if len(requests) != 1 {
			t.Fatalf("Initialize() requests = %d, want 1", len(requests))
		}
		request := requests[0]
		if request.Runtime.Bridge == nil || request.Runtime.Bridge.Purpose != subprocess.BridgeRuntimePurposeControl {
			t.Fatalf("Initialize() bridge runtime = %#v, want control purpose", request.Runtime.Bridge)
		}
		gotMethods := request.Methods.ExtensionServices
		wantMethods := []string{string(bridgepkg.ControlMethodCheck)}
		if !slices.Equal(gotMethods, wantMethods) {
			t.Fatalf("ExtensionServices = %#v, want %#v", gotMethods, wantMethods)
		}
		if len(request.Capabilities.Provides) != 0 || len(request.Capabilities.GrantedActions) != 0 ||
			len(request.Capabilities.GrantedSecurity) != 0 || len(request.Capabilities.GrantedResourceKinds) != 0 ||
			len(request.Capabilities.GrantedResourceScopes) != 0 {
			t.Fatalf("Initialize() capabilities = %#v, want zero grants", request.Capabilities)
		}
		process.mu.Lock()
		handlerCount := len(process.handlers)
		shutdownCount := process.shutdownCnt
		closed := process.closed
		process.mu.Unlock()
		if handlerCount != 0 {
			t.Fatalf("registered Host API handlers = %d, want zero", handlerCount)
		}
		if shutdownCount != 1 || !closed {
			t.Fatalf("process cleanup = shutdown %d closed %v, want 1/true", shutdownCount, closed)
		}
		if got := launcher.launchCount(); got != 1 {
			t.Fatalf("launch count = %d, want 1", got)
		}
		launcher.mu.Lock()
		launchConfig := launcher.config[0]
		launcher.mu.Unlock()
		if launchConfig.ProcessRegistry == nil ||
			launchConfig.ProcessRecord.Source != toolruntime.ProcessSourceExtension ||
			launchConfig.ProcessRecord.Owner.ExtensionName != "telegram-adapter" {
			t.Fatalf("process registration = %#v, want transient extension ownership", launchConfig.ProcessRecord)
		}
	})
}

func TestManagerBridgeControlCleansUpAfterProviderFailure(t *testing.T) {
	t.Run("Should redact provider failure and remove transient secret registration", func(t *testing.T) {
		t.Parallel()

		secret := "dynamic-control-secret-4102"
		wantErr := errors.New("provider control failed with " + secret)
		process := newFakeProcess(4102)
		process.initResp = controlInitializeResponse("telegram-adapter", bridgepkg.ControlMethodCheck)
		process.callFn = func(context.Context, string, any, any) error { return wantErr }
		runtime := controlBridgeRuntime("brg-control-failure", bridgepkg.ControlMethodCheck)
		runtime.ManagedInstances[0].BoundSecrets[0].Value = secret
		resolver := &stubBridgeControlRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				"brg-control-failure": runtime,
			},
		}
		manager := newBridgeControlTestManager(
			t,
			&fakeLauncher{queue: []*fakeProcess{process}},
			resolver,
		)

		_, err := manager.CheckBridge(
			testutil.Context(t),
			"telegram-adapter",
			bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-control-failure"},
		)
		if !errors.Is(err, wantErr) {
			t.Fatalf("CheckBridge() error = %v, want %v", err, wantErr)
		}
		if strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), redactpkg.Marker) {
			t.Fatalf("CheckBridge() error = %q, want redacted secret", err.Error())
		}
		if got := redactpkg.String(secret); got != secret {
			t.Fatalf("redactor after cleanup = %q, want dynamic secret unregistered", got)
		}
		process.mu.Lock()
		shutdownCount := process.shutdownCnt
		closed := process.closed
		process.mu.Unlock()
		if shutdownCount != 1 || !closed {
			t.Fatalf("process cleanup = shutdown %d closed %v, want 1/true", shutdownCount, closed)
		}
	})
}

func TestManagerBridgeControlReapsAfterCallerCancellation(t *testing.T) {
	t.Run("Should reap the process after caller cancellation", func(t *testing.T) {
		t.Parallel()

		entered := make(chan struct{})
		process := newFakeProcess(4105)
		process.initResp = controlInitializeResponse("telegram-adapter", bridgepkg.ControlMethodCheck)
		process.callFn = func(ctx context.Context, _ string, _ any, _ any) error {
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		}
		resolver := &stubBridgeControlRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				"brg-control-cancel": controlBridgeRuntime("brg-control-cancel", bridgepkg.ControlMethodCheck),
			},
		}
		manager := newBridgeControlTestManager(
			t,
			&fakeLauncher{queue: []*fakeProcess{process}},
			resolver,
		)
		ctx, cancel := context.WithCancel(testutil.Context(t))
		done := make(chan error, 1)
		go func() {
			_, err := manager.CheckBridge(
				ctx,
				"telegram-adapter",
				bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-control-cancel"},
			)
			done <- err
		}()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("control call did not reach provider")
		}
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("CheckBridge() error = %v, want context canceled", err)
		}
		process.mu.Lock()
		shutdownCount := process.shutdownCnt
		closed := process.closed
		process.mu.Unlock()
		if shutdownCount != 1 || !closed {
			t.Fatalf("process cleanup after cancel = shutdown %d closed %v, want 1/true", shutdownCount, closed)
		}
	})
}

func TestManagerStopWaitsForBridgeControlProcessReap(t *testing.T) {
	t.Run("Should cancel an admitted control call and observe process reap before returning", func(t *testing.T) {
		t.Parallel()

		const (
			processReaped = "process_reaped"
			stopReturned  = "stop_returned"
		)
		events := make(chan string, 2)
		entered := make(chan struct{})
		process := newFakeProcess(4107)
		process.initResp = controlInitializeResponse("telegram-adapter", bridgepkg.ControlMethodCheck)
		process.callFn = func(ctx context.Context, _ string, _ any, _ any) error {
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		}
		process.shutdownFn = func(context.Context) error {
			process.close(nil)
			events <- processReaped
			return nil
		}
		resolver := &stubBridgeControlRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				"brg-control-stop": controlBridgeRuntime("brg-control-stop", bridgepkg.ControlMethodCheck),
			},
		}
		manager := newBridgeControlTestManager(
			t,
			&fakeLauncher{queue: []*fakeProcess{process}},
			resolver,
		)
		callCtx, cancelCall := context.WithCancel(testutil.Context(t))
		t.Cleanup(cancelCall)
		callDone := make(chan error, 1)
		go func() {
			_, err := manager.CheckBridge(
				callCtx,
				"telegram-adapter",
				bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-control-stop"},
			)
			callDone <- err
		}()

		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("control call did not reach provider")
		}

		stopDone := make(chan error, 1)
		go func() {
			err := manager.Stop(testutil.Context(t))
			events <- stopReturned
			stopDone <- err
		}()

		select {
		case event := <-events:
			if event != processReaped {
				t.Fatalf("first lifecycle event = %q, want %q", event, processReaped)
			}
		case <-time.After(time.Second):
			t.Fatal("manager stop did not cancel and reap the control process")
		}
		select {
		case event := <-events:
			if event != stopReturned {
				t.Fatalf("second lifecycle event = %q, want %q", event, stopReturned)
			}
		case <-time.After(time.Second):
			t.Fatal("manager stop did not return after control process reap")
		}
		if err := <-stopDone; err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if err := <-callDone; !errors.Is(err, context.Canceled) {
			t.Fatalf("CheckBridge() error = %v, want lifecycle cancellation", err)
		}
	})
}

func TestManagerBridgeControlRequiresStartedRegisteredEnabledAdapter(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Manager)
	}{
		{
			name: "Should reject a control call before manager start",
			mutate: func(manager *Manager) {
				manager.started = false
				manager.lifecycleCtx = nil
				manager.cancel = nil
			},
		},
		{
			name: "Should reject a control call while manager stop is in progress",
			mutate: func(manager *Manager) {
				manager.stopping = true
			},
		},
		{
			name: "Should reject a control call for a disabled adapter",
			mutate: func(manager *Manager) {
				manager.extensions["telegram-adapter"].info.Enabled = false
			},
		},
		{
			name: "Should reject a control call for an unregistered adapter",
			mutate: func(manager *Manager) {
				manager.extensions["telegram-adapter"].registered = false
			},
		},
		{
			name: "Should reject a control call before adapter activation",
			mutate: func(manager *Manager) {
				manager.extensions["telegram-adapter"].phase = ExtensionPhaseInitialize
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			launcher := &fakeLauncher{queue: []*fakeProcess{newFakeProcess(4108)}}
			resolver := &stubBridgeControlRuntimeResolver{
				runtimes: map[string]*subprocess.InitializeBridgeRuntime{
					"brg-control-lifecycle": controlBridgeRuntime(
						"brg-control-lifecycle",
						bridgepkg.ControlMethodCheck,
					),
				},
			}
			manager := newBridgeControlTestManager(t, launcher, resolver)
			manager.mu.Lock()
			tt.mutate(manager)
			manager.mu.Unlock()

			_, err := manager.CheckBridge(
				testutil.Context(t),
				"telegram-adapter",
				bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-control-lifecycle"},
			)
			if !errors.Is(err, bridgepkg.ErrBridgeControlTransportUnavailable) {
				t.Fatalf("CheckBridge() error = %v, want control transport unavailable", err)
			}
			var redactedErr *bridgeControlRedactedError
			if !errors.As(err, &redactedErr) {
				t.Fatalf("CheckBridge() error type = %T, want *bridgeControlRedactedError", err)
			}
			if got := launcher.launchCount(); got != 0 {
				t.Fatalf("launch count = %d, want 0", got)
			}
		})
	}
}

func TestManagerBridgeControlReapsAfterInitializeFailure(t *testing.T) {
	t.Run("Should reap the process after initialize failure", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("initialize control failed")
		process := newFakeProcess(4106)
		process.initErr = wantErr
		resolver := &stubBridgeControlRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				"brg-control-init-failure": controlBridgeRuntime(
					"brg-control-init-failure",
					bridgepkg.ControlMethodCheck,
				),
			},
		}
		manager := newBridgeControlTestManager(
			t,
			&fakeLauncher{queue: []*fakeProcess{process}},
			resolver,
		)
		_, err := manager.CheckBridge(
			testutil.Context(t),
			"telegram-adapter",
			bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-control-init-failure"},
		)
		if !errors.Is(err, wantErr) {
			t.Fatalf("CheckBridge() error = %v, want initialize failure", err)
		}
		process.mu.Lock()
		shutdownCount := process.shutdownCnt
		closed := process.closed
		process.mu.Unlock()
		if shutdownCount != 1 || !closed {
			t.Fatalf(
				"process cleanup after initialize failure = shutdown %d closed %v, want 1/true",
				shutdownCount,
				closed,
			)
		}
	})
}

func TestManagerBridgeControlSerializesCallsForOneInstance(t *testing.T) {
	t.Run("Should serialize concurrent calls for one bridge instance", func(t *testing.T) {
		t.Parallel()

		entered := make(chan struct{})
		release := make(chan struct{})
		first := newFakeProcess(4103)
		first.initResp = controlInitializeResponse("telegram-adapter", bridgepkg.ControlMethodCheck)
		first.callFn = func(_ context.Context, _ string, _ any, result any) error {
			close(entered)
			<-release
			response := result.(*bridgepkg.BridgeCheckResponse)
			*response = bridgepkg.BridgeCheckResponse{
				Checks: []bridgepkg.BridgeCheckRecord{{
					Check: "provider.identity", Status: bridgepkg.BridgeCheckStatusPass,
				}},
			}
			return nil
		}
		second := newFakeProcess(4104)
		second.initResp = controlInitializeResponse("telegram-adapter", bridgepkg.ControlMethodCheck)
		second.callFn = func(_ context.Context, _ string, _ any, result any) error {
			response := result.(*bridgepkg.BridgeCheckResponse)
			*response = bridgepkg.BridgeCheckResponse{
				Checks: []bridgepkg.BridgeCheckRecord{{
					Check: "provider.identity", Status: bridgepkg.BridgeCheckStatusPass,
				}},
			}
			return nil
		}
		resolver := &stubBridgeControlRuntimeResolver{
			runtimes: map[string]*subprocess.InitializeBridgeRuntime{
				"brg-control": controlBridgeRuntime("brg-control", bridgepkg.ControlMethodCheck),
			},
		}
		launcher := &fakeLauncher{queue: []*fakeProcess{first, second}}
		manager := newBridgeControlTestManager(t, launcher, resolver)

		errCh := make(chan error, 2)
		secondAttempted := make(chan struct{})
		call := func() {
			_, err := manager.CheckBridge(
				testutil.Context(t),
				"telegram-adapter",
				bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-control"},
			)
			errCh <- err
		}
		go call()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("first control call did not enter provider RPC")
		}
		go func() {
			close(secondAttempted)
			call()
		}()
		<-secondAttempted
		select {
		case err := <-errCh:
			t.Fatalf("second CheckBridge() completed before first release: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
		if got := launcher.launchCount(); got != 1 {
			t.Fatalf("launch count while first call active = %d, want 1", got)
		}
		close(release)
		for range 2 {
			if err := <-errCh; err != nil {
				t.Fatalf("CheckBridge() error = %v", err)
			}
		}
		if got := launcher.launchCount(); got != 2 {
			t.Fatalf("final launch count = %d, want 2", got)
		}
	})
}

func TestBridgeControlGlobalConcurrencyIsBounded(t *testing.T) {
	t.Run("Should reject acquisition beyond the global process bound", func(t *testing.T) {
		// Intentionally serial because this case owns every process-wide control slot.
		releases := make([]func(), 0, maxConcurrentBridgeControls)
		for range maxConcurrentBridgeControls {
			release, err := acquireBridgeControlGlobal(testutil.Context(t))
			if err != nil {
				t.Fatalf("acquireBridgeControlGlobal() error = %v", err)
			}
			releases = append(releases, release)
		}
		defer func() {
			for _, release := range releases {
				release()
			}
		}()

		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		if _, err := acquireBridgeControlGlobal(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("acquireBridgeControlGlobal(full) error = %v, want context canceled", err)
		}
	})
}

func newBridgeControlTestManager(
	t *testing.T,
	launcher *fakeLauncher,
	resolver *stubBridgeControlRuntimeResolver,
) *Manager {
	t.Helper()

	manager := NewManager(
		nil,
		WithBridgeRuntimeResolver(resolver),
		WithProcessRegistry(toolruntime.NewRegistry(toolruntime.NewMemoryStore())),
		withProcessLauncher(launcher.launch),
		WithInitializeTimeout(time.Second),
	)
	lifecycleCtx, cancelLifecycle := context.WithCancel(context.Background())
	manager.lifecycleCtx = lifecycleCtx
	manager.cancel = cancelLifecycle
	manager.started = true
	manager.extensions = map[string]*managedExtension{
		"telegram-adapter": {
			info: ExtensionInfo{
				Name:    "telegram-adapter",
				Version: "1.0.0",
				Source:  SourceBundled,
				Enabled: true,
			},
			rootDir: t.TempDir(),
			manifest: &Manifest{
				Name:    "telegram-adapter",
				Version: "1.0.0",
				Capabilities: CapabilitiesConfig{
					Provides: []string{"bridge.adapter"},
				},
				Subprocess: SubprocessConfig{Command: "/bin/echo"},
				Bridge:     BridgeConfig{Platform: "telegram", DisplayName: "Telegram"},
			},
			registered: true,
			phase:      ExtensionPhaseActivate,
		},
	}
	t.Cleanup(func() {
		cancelLifecycle()
		manager.wg.Wait()
	})
	return manager
}

type stubBridgeControlRuntimeResolver struct {
	mu       sync.Mutex
	runtimes map[string]*subprocess.InitializeBridgeRuntime
}

func (r *stubBridgeControlRuntimeResolver) ResolveBridgeRuntime(
	context.Context,
	string,
) (*subprocess.InitializeBridgeRuntime, error) {
	return nil, ErrBridgeRuntimeDeferred
}

func (r *stubBridgeControlRuntimeResolver) ResolveBridgeControlRuntime(
	_ context.Context,
	extensionName string,
	bridgeInstanceID string,
	method bridgepkg.ControlMethod,
) (*subprocess.InitializeBridgeRuntime, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	runtime := subprocess.CloneInitializeBridgeRuntime(r.runtimes[strings.TrimSpace(bridgeInstanceID)])
	if runtime == nil {
		return nil, bridgepkg.ErrBridgeInstanceNotFound
	}
	if runtime.Provider != strings.TrimSpace(extensionName) {
		return nil, errors.New("resolver extension mismatch")
	}
	if len(runtime.AllowedMethods) != 1 || runtime.AllowedMethods[0] != string(method) {
		return nil, errors.New("resolver control method mismatch")
	}
	return runtime, nil
}

func controlBridgeRuntime(
	instanceID string,
	method bridgepkg.ControlMethod,
) *subprocess.InitializeBridgeRuntime {
	instance := testBridgeRuntimeInstance("telegram-adapter", instanceID)
	instance.Enabled = false
	instance.Status = bridgepkg.BridgeStatusDisabled
	return &subprocess.InitializeBridgeRuntime{
		RuntimeVersion: subprocess.InitializeBridgeRuntimeVersion2,
		Purpose:        subprocess.BridgeRuntimePurposeControl,
		Provider:       "telegram-adapter",
		Platform:       "telegram",
		AllowedMethods: []string{string(method)},
		ManagedInstances: []subprocess.InitializeBridgeManagedInstance{{
			Instance: bridgepkg.BridgeInstanceToContract(instance),
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{{
				BindingName: "bot_token",
				Kind:        "token",
				Value:       "test-secret",
			}},
		}},
	}
}

func controlInitializeResponse(
	extensionName string,
	method bridgepkg.ControlMethod,
) subprocess.InitializeResponse {
	return subprocess.InitializeResponse{
		ProtocolVersion: "1",
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    extensionName,
			Version: "1.0.0",
		},
		ImplementedMethods: []string{string(method), "shutdown"},
	}
}
