package extensions

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/subprocess"
	runtimeevents "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var (
	mockExtensionBuildOnce sync.Once
	mockExtensionBinary    string
	mockExtensionBuildErr  error
)

type mockRecord struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

type managerHarness struct {
	scope       *RunScope
	manager     *Manager
	events      <-chan runtimeevents.Event
	unsubscribe func()
	recordPath  string
}

func TestManagerStartInitializesExtensionsPublishesReadyAndDispatchesHooks(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:       binary,
		Mode:         "normal",
		Capabilities: []Capability{CapabilityPromptMutate},
		Hooks:        []HookDeclaration{{Event: HookPromptPostBuild}},
		Env: map[string]string{
			"COMPOZY_MOCK_SUPPORTED_HOOKS": "prompt.post_build",
			"COMPOZY_MOCK_PATCH_JSON":      `{"message":"patched"}`,
		},
		ParentRunID: "parent-001",
	})
	defer harness.Close(t)

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	loaded := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionLoaded)
	ready := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionReady)

	var readyPayload kinds.ExtensionReadyPayload
	decodeEventPayload(t, ready, &readyPayload)
	if readyPayload.Extension != "mock-ext" {
		t.Fatalf("ready payload extension = %q, want %q", readyPayload.Extension, "mock-ext")
	}

	records := waitForRecords(t, harness.recordPath, 2)
	envRecord := findRecord(t, records, "initialize_env")
	requestRecord := findRecord(t, records, "initialize_request")
	if got := envRecord.Payload["run_id"]; got != harness.scope.Artifacts.RunID {
		t.Fatalf("env run_id = %#v, want %q", got, harness.scope.Artifacts.RunID)
	}
	if got := envRecord.Payload["parent_run_id"]; got != "parent-001" {
		t.Fatalf("env parent_run_id = %#v, want %q", got, "parent-001")
	}
	if got := envRecord.Payload["extension_source"]; got != string(SourceWorkspace) {
		t.Fatalf("env extension_source = %#v, want %q", got, SourceWorkspace)
	}
	if got := requestRecord.Payload["invoking_command"]; got != "start" {
		t.Fatalf("initialize invoking_command = %#v, want %q", got, "start")
	}
	if readyPayload.ProtocolVersion != compozyProtocolVersion() {
		t.Fatalf("ready protocol_version = %q, want %q", readyPayload.ProtocolVersion, compozyProtocolVersion())
	}

	result, err := harness.manager.DispatchMutable(context.Background(), HookPromptPostBuild, map[string]any{
		"message": "original",
	})
	if err != nil {
		t.Fatalf("DispatchMutable() error = %v", err)
	}

	updated, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	if got := updated["message"]; got != "patched" {
		t.Fatalf("patched message = %#v, want %q", got, "patched")
	}

	if loaded.Kind != runtimeevents.EventKindExtensionLoaded {
		t.Fatalf("loaded event kind = %q", loaded.Kind)
	}
}

func TestManagerStartRejectsUnsupportedProtocolVersion(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:       binary,
		Mode:         "unsupported_protocol",
		Capabilities: []Capability{CapabilityPromptMutate},
		Hooks:        []HookDeclaration{{Event: HookPromptPostBuild}},
		Env:          map[string]string{"COMPOZY_MOCK_SUPPORTED_HOOKS": "prompt.post_build"},
	})
	defer harness.Close(t)

	err := harness.manager.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unsupported_protocol_version") {
		t.Fatalf("Start() error = %v, want unsupported protocol version", err)
	}

	failed := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionFailed)
	var payload kinds.ExtensionFailedPayload
	decodeEventPayload(t, failed, &payload)
	if payload.Phase != extensionFailurePhaseInitialize {
		t.Fatalf("failed phase = %q, want %q", payload.Phase, extensionFailurePhaseInitialize)
	}
}

func TestManagerStartRejectsCapabilityMismatch(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:       binary,
		Mode:         "capability_mismatch",
		Capabilities: []Capability{CapabilityPromptMutate},
		Hooks:        []HookDeclaration{{Event: HookPromptPostBuild}},
		Env:          map[string]string{"COMPOZY_MOCK_SUPPORTED_HOOKS": "prompt.post_build"},
	})
	defer harness.Close(t)

	err := harness.manager.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unsupported_capability_acceptance") {
		t.Fatalf("Start() error = %v, want capability mismatch", err)
	}
}

func TestManagerStartRejectsEventsReadWithoutOnEventSupport(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:       binary,
		Mode:         "events_without_on_event",
		Capabilities: []Capability{CapabilityEventsRead},
		Env:          map[string]string{},
	})
	defer harness.Close(t)

	err := harness.manager.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "inconsistent_event_contract") {
		t.Fatalf("Start() error = %v, want inconsistent event contract", err)
	}
}

func TestHealthFalseMarksExtensionUnhealthyAndPublishesFailure(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	restoreHealthTimeout := setHealthTimeoutForTest(t, 25*time.Millisecond)
	defer restoreHealthTimeout()

	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:       binary,
		Mode:         "health_false",
		Capabilities: []Capability{},
		HealthPeriod: 20 * time.Millisecond,
	})
	defer harness.Close(t)

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	failed := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionFailed)
	var payload kinds.ExtensionFailedPayload
	decodeEventPayload(t, failed, &payload)
	if payload.Phase != extensionFailurePhaseHealth {
		t.Fatalf("failed phase = %q, want %q", payload.Phase, extensionFailurePhaseHealth)
	}
	waitForExtensionState(t, harness.manager.registry.Extensions()[0], ExtensionStateStopped)
}

func TestHealthTimeoutMarksExtensionUnhealthyAfterTwoFailures(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	restoreHealthTimeout := setHealthTimeoutForTest(t, 20*time.Millisecond)
	defer restoreHealthTimeout()

	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:       binary,
		Mode:         "health_timeout",
		Capabilities: []Capability{},
		HealthPeriod: 15 * time.Millisecond,
		Env: map[string]string{
			"COMPOZY_MOCK_HEALTH_DELAY_MS": "120",
		},
	})
	defer harness.Close(t)

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	failed := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionFailed)
	var payload kinds.ExtensionFailedPayload
	decodeEventPayload(t, failed, &payload)
	if payload.Phase != extensionFailurePhaseHealth {
		t.Fatalf("failed phase = %q, want %q", payload.Phase, extensionFailurePhaseHealth)
	}
}

func TestManagerShutdownSendsShutdownWithoutForceWhenExtensionExitsCooperatively(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:          binary,
		Mode:            "normal",
		Capabilities:    []Capability{CapabilityPromptMutate},
		ShutdownTimeout: 120 * time.Millisecond,
		Env: map[string]string{
			"COMPOZY_MOCK_SHUTDOWN_DELAY_MS": "40",
		},
	})
	defer harness.Close(t)

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := harness.manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	process := harness.manager.subprocs["mock-ext"]
	if process == nil {
		t.Fatal("expected subprocess entry")
	}
	if process.Forced() {
		t.Fatal("expected cooperative shutdown without force")
	}

	records := waitForRecords(t, harness.recordPath, 3)
	findRecord(t, records, "shutdown")
}

func TestManagerShutdownEscalatesToForceKill(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:          binary,
		Mode:            "ignore_shutdown",
		Capabilities:    []Capability{},
		ShutdownTimeout: 20 * time.Millisecond,
	})
	defer harness.Close(t)

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := harness.manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	process := harness.manager.subprocs["mock-ext"]
	if process == nil {
		t.Fatal("expected subprocess entry")
	}
	if !process.Forced() {
		t.Fatal("expected forced shutdown path")
	}
}

func TestOnEventForwardingDropsWhenQueueIsFull(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	restoreQueueCap := setEventQueueCapForTest(t, 1)
	defer restoreQueueCap()

	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:       binary,
		Mode:         "normal",
		Capabilities: []Capability{CapabilityEventsRead},
		Env: map[string]string{
			"COMPOZY_MOCK_ON_EVENT_DELAY_MS": "80",
		},
	})
	defer harness.Close(t)

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	for i := 0; i < 5; i++ {
		harness.scope.EventBus.Publish(context.Background(), runtimeevents.Event{
			Kind:    runtimeevents.EventKindJobCompleted,
			Payload: json.RawMessage(`{}`),
		})
	}

	dropEvent := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionEvent)
	var payload kinds.ExtensionEventPayload
	decodeEventPayload(t, dropEvent, &payload)
	if payload.Kind != "delivery_dropped" {
		t.Fatalf("payload.Kind = %q, want %q", payload.Kind, "delivery_dropped")
	}
}

func TestHostTasksListIsRoutedThroughHostAPIRouter(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:       binary,
		Mode:         "host_tasks_list",
		Capabilities: []Capability{CapabilityTasksRead},
		Workflow:     "demo",
	})
	defer harness.Close(t)

	writeTaskFixture(t, harness.manager.workspaceRoot, "demo", 1, "pending", "Demo task", "backend", "body")

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	records := waitForRecords(t, harness.recordPath, 3)
	record := findRecord(t, records, "host_tasks_list")
	if got := record.Payload["count"]; got != float64(1) {
		t.Fatalf("host_tasks_list count = %#v, want 1", got)
	}
}

func TestValidateInitializeContractRejectsHookMismatch(t *testing.T) {
	extension, err := runtimeExtensionFromDiscovered(DiscoveredExtension{
		Ref: Ref{Name: "mock-ext", Source: SourceWorkspace},
		Manifest: &Manifest{
			Extension: ExtensionInfo{Name: "mock-ext", Version: "1.0.0"},
			Subprocess: &SubprocessConfig{
				Command: "mock-extension",
			},
			Hooks: []HookDeclaration{{Event: HookPromptPostBuild}},
		},
	})
	if err != nil {
		t.Fatalf("runtimeExtensionFromDiscovered() error = %v", err)
	}

	err = validateInitializeContract(
		extension,
		initializeRequest{
			ProtocolVersion:           "1",
			SupportedProtocolVersions: []string{"1"},
			GrantedCapabilities:       []Capability{CapabilityPromptMutate},
		},
		initializeResponse{
			ProtocolVersion:      "1",
			AcceptedCapabilities: []Capability{CapabilityPromptMutate},
			SupportedHookEvents:  nil,
		},
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported_hook_contract") {
		t.Fatalf("validateInitializeContract() error = %v, want hook contract failure", err)
	}
}

func TestExtensionSessionCallRoundTripAndClose(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	session := &extensionSession{
		transport: subprocess.NewTransport(left, left),
		pending:   make(map[string]chan sessionCallResult),
		done:      make(chan struct{}),
	}
	peer := subprocess.NewTransport(right, right)

	go func() {
		message, err := peer.ReadMessage()
		if err != nil {
			t.Errorf("peer.ReadMessage() error = %v", err)
			return
		}
		if message.Method != "ping" {
			t.Errorf("message.Method = %q, want %q", message.Method, "ping")
			return
		}
		session.deliverResponse(subprocess.Message{
			ID:     message.ID,
			Result: mustJSON(t, map[string]string{"status": "ok"}),
		})
	}()

	result := map[string]string{}
	if err := session.Call(context.Background(), "ping", map[string]string{"hello": "world"}, &result); err != nil {
		t.Fatalf("Call() error = %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("result status = %q, want %q", result["status"], "ok")
	}

	session.close(nil)
	if err := session.closedErr(); err == nil {
		t.Fatal("closedErr() = nil, want session closed error")
	}
}

func TestExtensionSessionHandleIncomingRequestWritesResponse(t *testing.T) {
	rt := newHostRuntime(t, []Capability{CapabilityEventsRead}, nil, "")
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	session := &extensionSession{
		manager:   &Manager{hostAPI: rt.router},
		runtime:   rt.extension,
		transport: subprocess.NewTransport(left, left),
		pending:   make(map[string]chan sessionCallResult),
		done:      make(chan struct{}),
	}
	peer := subprocess.NewTransport(right, right)
	requestID := json.RawMessage("1")

	type result struct {
		message subprocess.Message
		err     error
	}
	resultCh := make(chan result, 1)
	go func() {
		message, err := peer.ReadMessage()
		resultCh <- result{message: message, err: err}
	}()

	if err := session.handleIncomingRequest(subprocess.Message{
		ID:     &requestID,
		Method: "host.events.subscribe",
		Params: mustJSON(
			t,
			EventSubscribeRequest{Kinds: []runtimeevents.EventKind{runtimeevents.EventKindJobCompleted}},
		),
	}); err != nil {
		t.Fatalf("handleIncomingRequest() error = %v", err)
	}

	response := <-resultCh
	if response.err != nil {
		t.Fatalf("peer.ReadMessage() error = %v", response.err)
	}
	message := response.message
	if message.Error != nil {
		t.Fatalf("response.Error = %v, want nil", message.Error)
	}
	var payload EventSubscribeResult
	if err := json.Unmarshal(message.Result, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.SubscriptionID == "" {
		t.Fatal("expected subscription id")
	}
}

func TestManagerHelpersCoverNilAndEventSkippingPaths(t *testing.T) {
	(&Manager{}).DispatchObserver(context.Background(), HookJobPostExecute, map[string]any{"ok": true})

	if got := invokingCommandForMode(model.ExecutionModePRReview); got != "fix-reviews" {
		t.Fatalf("invokingCommandForMode(PRReview) = %q", got)
	}
	if got := invokingCommandForMode(model.ExecutionModeExec); got != executionModeLabelExec {
		t.Fatalf("invokingCommandForMode(Exec) = %q, want %q", got, executionModeLabelExec)
	}

	raw, err := marshalResult(json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatalf("marshalResult(raw) error = %v", err)
	}
	if string(raw) != `{"ok":true}` {
		t.Fatalf("marshalResult(raw) = %s", raw)
	}

	if !shouldSkipEventForwarding(runtimeevents.Event{
		Kind: runtimeevents.EventKindExtensionEvent,
		Payload: mustJSON(t, kinds.ExtensionEventPayload{
			Kind: "delivery_dropped",
		}),
	}) {
		t.Fatal("expected delivery_dropped event to be skipped")
	}
}

func TestManagerHelperErrorConversionAndTimeoutDetection(t *testing.T) {
	if requestErrorFromError(nil, "host.tasks.list") != nil {
		t.Fatal("requestErrorFromError(nil) should return nil")
	}

	requestErr := subprocess.NewInvalidParams(map[string]any{"reason": "bad"})
	if got := requestErrorFromError(requestErr, "host.tasks.list"); got != requestErr {
		t.Fatal("expected requestErrorFromError to preserve request errors")
	}

	internal := requestErrorFromError(errors.New("boom"), " host.tasks.list ")
	if internal == nil {
		t.Fatal("expected wrapped internal error")
	}
	if internal.Code != -32603 {
		t.Fatalf("internal.Code = %d, want -32603", internal.Code)
	}
	data, ok := internal.Data.(map[string]any)
	if !ok {
		t.Fatalf("internal.Data type = %T, want map[string]any", internal.Data)
	}
	if got := data["method"]; got != "host.tasks.list" {
		t.Fatalf("internal.Data[method] = %#v, want %q", got, "host.tasks.list")
	}
	if got := data["error"]; got != "boom" {
		t.Fatalf("internal.Data[error] = %#v, want %q", got, "boom")
	}

	if !isTimeoutError(context.DeadlineExceeded) {
		t.Fatal("expected context deadline exceeded to be detected as timeout")
	}
	if isTimeoutError(context.Canceled) {
		t.Fatal("did not expect context canceled to be treated as timeout")
	}
}

func TestManagerWaitForObserversHandlesCompletionAndTimeout(t *testing.T) {
	t.Run("completion", func(t *testing.T) {
		dispatcher := &HookDispatcher{}
		dispatcher.pending.Add(1)

		manager := &Manager{dispatcher: dispatcher}
		go func() {
			time.Sleep(20 * time.Millisecond)
			dispatcher.pending.Done()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		if err := manager.waitForObservers(ctx); err != nil {
			t.Fatalf("waitForObservers() error = %v", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		dispatcher := &HookDispatcher{}
		dispatcher.pending.Add(1)
		defer dispatcher.pending.Done()

		manager := &Manager{dispatcher: dispatcher}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		err := manager.waitForObservers(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waitForObservers() error = %v, want context deadline exceeded", err)
		}
	})
}

func TestExtensionSessionCallReturnsContextAndClosedErrors(t *testing.T) {
	t.Run("context timeout", func(t *testing.T) {
		left, right := net.Pipe()
		defer left.Close()
		defer right.Close()

		session := &extensionSession{
			transport: subprocess.NewTransport(left, left),
			pending:   make(map[string]chan sessionCallResult),
			done:      make(chan struct{}),
		}
		peer := subprocess.NewTransport(right, right)

		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			if _, err := peer.ReadMessage(); err != nil {
				t.Errorf("peer.ReadMessage() error = %v", err)
				return
			}
			<-session.done
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		err := session.Call(ctx, "ping", map[string]string{"hello": "world"}, &struct{}{})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Call() error = %v, want context deadline exceeded", err)
		}

		session.close(nil)
		<-readDone

		session.pendingMu.Lock()
		pending := len(session.pending)
		session.pendingMu.Unlock()
		if pending != 0 {
			t.Fatalf("pending call count = %d, want 0", pending)
		}
	})

	t.Run("closed session", func(t *testing.T) {
		session := &extensionSession{
			done:    make(chan struct{}),
			pending: make(map[string]chan sessionCallResult),
		}
		session.close(nil)

		err := session.Call(context.Background(), "ping", map[string]string{"hello": "world"}, &struct{}{})
		if !errors.Is(err, errExtensionSessionClosed) {
			t.Fatalf("Call() error = %v, want errExtensionSessionClosed", err)
		}
	})
}

func TestInitializeExtensionTransportRejectsUnexpectedRequest(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	hostTransport := subprocess.NewTransport(left, left)
	peerTransport := subprocess.NewTransport(right, right)

	go func() {
		message, err := peerTransport.ReadMessage()
		if err != nil {
			t.Errorf("peerTransport.ReadMessage() error = %v", err)
			return
		}
		if err := peerTransport.WriteMessage(subprocess.Message{
			ID:     message.ID,
			Method: "host.tasks.list",
			Params: mustJSON(t, map[string]any{"workflow": "demo"}),
		}); err != nil {
			t.Errorf("peerTransport.WriteMessage() error = %v", err)
		}
	}()

	_, err := initializeExtensionTransport(context.Background(), hostTransport, initializeRequest{
		ProtocolVersion:           "1",
		SupportedProtocolVersions: []string{"1"},
	})
	if err == nil || !strings.Contains(err.Error(), "unexpected_initialize_request") {
		t.Fatalf("initializeExtensionTransport() error = %v, want unexpected initialize request", err)
	}
}

func TestEmitLifecycleEventPublishesDirectlyToBus(t *testing.T) {
	bus := runtimeevents.New[runtimeevents.Event](1)
	defer bus.Close(context.Background())

	_, ch, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	manager := &Manager{
		runID:    "run-test",
		eventBus: bus,
	}
	if err := manager.emitLifecycleEvent(
		context.Background(),
		runtimeevents.EventKindExtensionLoaded,
		kinds.ExtensionLoadedPayload{
			Extension: "mock-ext",
		},
	); err != nil {
		t.Fatalf("emitLifecycleEvent() error = %v", err)
	}

	event := awaitEvent(t, ch, runtimeevents.EventKindExtensionLoaded)
	var payload kinds.ExtensionLoadedPayload
	decodeEventPayload(t, event, &payload)
	if payload.Extension != "mock-ext" {
		t.Fatalf("payload.Extension = %q, want %q", payload.Extension, "mock-ext")
	}
}

func TestResolveExtensionCommandUsesExtensionDirForRelativePath(t *testing.T) {
	extension := &RuntimeExtension{
		Name:         "mock-ext",
		ExtensionDir: filepath.Join(string(os.PathSeparator), "tmp", "mock-ext"),
		Manifest: &Manifest{
			Subprocess: &SubprocessConfig{
				Command: filepath.Join(".", "bin", "mock-extension"),
				Args:    []string{"--stdio"},
			},
		},
	}

	command, err := resolveExtensionCommand(extension)
	if err != nil {
		t.Fatalf("resolveExtensionCommand() error = %v", err)
	}
	if got, want := command[0], filepath.Join(extension.ExtensionDir, "bin", "mock-extension"); got != want {
		t.Fatalf("command[0] = %q, want %q", got, want)
	}
}

func TestExtensionWorkingDirFallsBackToBinaryDirectoryWhenExtensionDirIsMissing(t *testing.T) {
	binaryDir := t.TempDir()
	command := []string{filepath.Join(binaryDir, "mock-extension")}

	got := extensionWorkingDir(&RuntimeExtension{
		ExtensionDir: filepath.Join(binaryDir, "missing"),
	}, command)

	if got != binaryDir {
		t.Fatalf("extensionWorkingDir() = %q, want %q", got, binaryDir)
	}
}

func TestEmitLifecycleEventPublishesThroughJournal(t *testing.T) {
	rt := newHostRuntime(t, nil, nil, "")
	_, ch, unsubscribe := rt.bus.Subscribe()
	defer unsubscribe()

	manager := &Manager{
		runID:    rt.runID,
		journal:  rt.journal,
		eventBus: rt.bus,
	}
	if err := manager.emitLifecycleEvent(
		context.Background(),
		runtimeevents.EventKindExtensionReady,
		kinds.ExtensionReadyPayload{
			Extension:       "mock-ext",
			ProtocolVersion: "1",
		},
	); err != nil {
		t.Fatalf("emitLifecycleEvent() error = %v", err)
	}

	event := awaitEvent(t, ch, runtimeevents.EventKindExtensionReady)
	var payload kinds.ExtensionReadyPayload
	decodeEventPayload(t, event, &payload)
	if payload.Extension != "mock-ext" {
		t.Fatalf("payload.Extension = %q, want %q", payload.Extension, "mock-ext")
	}
}

func TestInitializeExtensionTransportRejectsErrorResponsesAndBadPayloads(t *testing.T) {
	t.Run("error response", func(t *testing.T) {
		left, right := net.Pipe()
		defer left.Close()
		defer right.Close()

		hostTransport := subprocess.NewTransport(left, left)
		peerTransport := subprocess.NewTransport(right, right)
		go func() {
			message, err := peerTransport.ReadMessage()
			if err != nil {
				t.Errorf("peerTransport.ReadMessage() error = %v", err)
				return
			}
			if err := peerTransport.WriteMessage(subprocess.Message{
				ID:    message.ID,
				Error: subprocess.NewInvalidParams(map[string]any{"reason": "bad"}),
			}); err != nil {
				t.Errorf("peerTransport.WriteMessage() error = %v", err)
			}
		}()

		_, err := initializeExtensionTransport(context.Background(), hostTransport, initializeRequest{
			ProtocolVersion:           "1",
			SupportedProtocolVersions: []string{"1"},
		})
		if err == nil || !strings.Contains(err.Error(), "Invalid params") {
			t.Fatalf("initializeExtensionTransport() error = %v, want invalid params", err)
		}
	})

	t.Run("bad payload", func(t *testing.T) {
		left, right := net.Pipe()
		defer left.Close()
		defer right.Close()

		hostTransport := subprocess.NewTransport(left, left)
		peerTransport := subprocess.NewTransport(right, right)
		go func() {
			message, err := peerTransport.ReadMessage()
			if err != nil {
				t.Errorf("peerTransport.ReadMessage() error = %v", err)
				return
			}
			if err := peerTransport.WriteMessage(subprocess.Message{
				ID:     message.ID,
				Result: json.RawMessage(`{"protocol_version": 1}`),
			}); err != nil {
				t.Errorf("peerTransport.WriteMessage() error = %v", err)
			}
		}()

		_, err := initializeExtensionTransport(context.Background(), hostTransport, initializeRequest{
			ProtocolVersion:           "1",
			SupportedProtocolVersions: []string{"1"},
		})
		if err == nil || !strings.Contains(err.Error(), "Internal error") {
			t.Fatalf("initializeExtensionTransport() error = %v, want internal error", err)
		}
	})
}

func TestStartExtensionReportsSpawnFailure(t *testing.T) {
	bus := runtimeevents.New[runtimeevents.Event](1)
	defer bus.Close(context.Background())
	_, ch, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	manager := &Manager{
		runID:           "run-test",
		workspaceRoot:   t.TempDir(),
		invokingCommand: "start",
		eventBus:        bus,
	}
	extension := &RuntimeExtension{
		Name: "broken",
		Ref:  Ref{Name: "broken", Source: SourceWorkspace},
		Manifest: &Manifest{
			Extension: ExtensionInfo{Name: "broken", Version: "1.0.0"},
			Subprocess: &SubprocessConfig{
				Command: "",
			},
		},
	}

	if err := manager.startExtension(context.Background(), extension); err == nil {
		t.Fatal("expected startExtension() to fail")
	}
	event := awaitEvent(t, ch, runtimeevents.EventKindExtensionFailed)
	var payload kinds.ExtensionFailedPayload
	decodeEventPayload(t, event, &payload)
	if payload.Phase != extensionFailurePhaseSpawn {
		t.Fatalf("payload.Phase = %q, want %q", payload.Phase, extensionFailurePhaseSpawn)
	}
}

func TestFormatProcessStderrIncludesCapturedOutput(t *testing.T) {
	process, err := subprocess.Launch(context.Background(), subprocess.LaunchConfig{
		Command: []string{"/bin/sh", "-c", "printf 'boom\\n' >&2"},
	})
	if err != nil {
		t.Fatalf("Launch() error = %v", err)
	}
	if err := process.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	if got := formatProcessStderr(process); got != "; stderr: boom" {
		t.Fatalf("formatProcessStderr() = %q, want %q", got, "; stderr: boom")
	}
	if got := formatProcessStderr(nil); got != "" {
		t.Fatalf("formatProcessStderr(nil) = %q, want empty string", got)
	}
}

func TestManagerStartPublishesFailureWhenProcessExitsAfterInitialize(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	harness := newManagerHarness(t, managerHarnessSpec{
		Binary:       binary,
		Mode:         "exit_after_init",
		Capabilities: []Capability{},
	})
	defer harness.Close(t)

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	failed := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionFailed)
	var payload kinds.ExtensionFailedPayload
	decodeEventPayload(t, failed, &payload)
	if payload.Phase != extensionFailurePhaseTransport {
		t.Fatalf("payload.Phase = %q, want %q", payload.Phase, extensionFailurePhaseTransport)
	}
}

type managerHarnessSpec struct {
	Name            string
	Binary          string
	Mode            string
	Capabilities    []Capability
	Hooks           []HookDeclaration
	Env             map[string]string
	HealthPeriod    time.Duration
	ShutdownTimeout time.Duration
	ParentRunID     string
	Workflow        string
}

func newManagerHarness(t *testing.T, spec managerHarnessSpec) *managerHarness {
	t.Helper()

	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = "mock-ext"
	}

	recordPath := filepath.Join(t.TempDir(), "records.jsonl")
	env := map[string]string{
		"COMPOZY_MOCK_MODE":        spec.Mode,
		"COMPOZY_MOCK_RECORD_PATH": recordPath,
		"COMPOZY_SDK_RECORD_PATH":  recordPath,
	}
	for key, value := range spec.Env {
		env[key] = value
	}
	if spec.Workflow != "" {
		env["COMPOZY_MOCK_WORKFLOW"] = spec.Workflow
		env["COMPOZY_SDK_WORKFLOW"] = spec.Workflow
	}

	discovered := DiscoveredExtension{
		Ref: Ref{Name: name, Source: SourceWorkspace},
		Manifest: &Manifest{
			Extension: ExtensionInfo{
				Name:              name,
				Version:           "1.0.0",
				MinCompozyVersion: "0.0.0",
			},
			Subprocess: &SubprocessConfig{
				Command:           spec.Binary,
				Env:               env,
				ShutdownTimeout:   spec.ShutdownTimeout,
				HealthCheckPeriod: spec.HealthPeriod,
			},
			Security: SecurityConfig{Capabilities: spec.Capabilities},
			Hooks:    spec.Hooks,
		},
		ExtensionDir: filepath.Dir(spec.Binary),
		ManifestPath: filepath.Join(filepath.Dir(spec.Binary), "extension.toml"),
		Enabled:      true,
	}

	cfg := runtimeConfigForTest(t)
	cfg.ParentRunID = spec.ParentRunID
	restoreDiscovery := stubRunScopeDiscovery(t, DiscoveryResult{Extensions: []DiscoveredExtension{discovered}})
	defer restoreDiscovery()

	scope, err := OpenRunScope(context.Background(), cfg, OpenRunScopeOptions{EnableExecutableExtensions: true})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}

	_, ch, unsubscribe := scope.EventBus.Subscribe()
	return &managerHarness{
		scope:       scope,
		manager:     scope.Manager,
		events:      ch,
		unsubscribe: unsubscribe,
		recordPath:  recordPath,
	}
}

func (h *managerHarness) Close(t *testing.T) {
	t.Helper()
	if h == nil {
		return
	}
	if h.unsubscribe != nil {
		h.unsubscribe()
	}
	if h.scope != nil {
		if err := h.scope.Close(context.Background()); err != nil {
			t.Fatalf("scope.Close() error = %v", err)
		}
	}
}

func buildMockExtensionBinary(t *testing.T) string {
	t.Helper()

	mockExtensionBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "compozy-mock-extension-*")
		if err != nil {
			mockExtensionBuildErr = err
			return
		}
		binary := filepath.Join(dir, "mock-extension")
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", binary, "./testdata/mock_extension")
		cmd.Dir = "."
		output, err := cmd.CombinedOutput()
		if err != nil {
			mockExtensionBuildErr = fmt.Errorf("go build mock extension: %w: %s", err, output)
			return
		}
		mockExtensionBinary = binary
	})

	if mockExtensionBuildErr != nil {
		t.Fatal(mockExtensionBuildErr)
	}
	return mockExtensionBinary
}

func waitForExtensionState(t *testing.T, extension *RuntimeExtension, want ExtensionState) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if extension.State() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("extension state = %q, want %q", extension.State(), want)
}

func readRecords(t *testing.T, path string) []mockRecord {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("open record file: %v", err)
	}
	defer file.Close()

	records := make([]mockRecord, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record mockRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("unmarshal record: %v", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan records: %v", err)
	}
	return records
}

func waitForRecords(t *testing.T, path string, minRecords int) []mockRecord {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		records := readRecords(t, path)
		if len(records) >= minRecords {
			return records
		}
		time.Sleep(10 * time.Millisecond)
	}
	return readRecords(t, path)
}

func findRecord(t *testing.T, records []mockRecord, kind string) mockRecord {
	t.Helper()

	for _, record := range records {
		if record.Type == kind {
			return record
		}
	}
	t.Fatalf("record %q not found in %#v", kind, records)
	return mockRecord{}
}

func decodeEventPayload(t *testing.T, event runtimeevents.Event, target any) {
	t.Helper()
	if err := json.Unmarshal(event.Payload, target); err != nil {
		t.Fatalf("decode %s payload: %v", event.Kind, err)
	}
}

func setHealthTimeoutForTest(t *testing.T, timeout time.Duration) func() {
	t.Helper()
	previous := extensionHealthTimeout
	extensionHealthTimeout = timeout
	return func() {
		extensionHealthTimeout = previous
	}
}

func setEventQueueCapForTest(t *testing.T, size int) func() {
	t.Helper()
	previous := extensionEventQueueCap
	extensionEventQueueCap = size
	return func() {
		extensionEventQueueCap = previous
	}
}
