package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestExecutorWaitsForUIQuitAfterJobsComplete(t *testing.T) {
	t.Parallel()

	ui := newFakeUISession()
	done := make(chan struct{})
	close(done)

	controller := &executorController{
		ctx: context.Background(),
		execCtx: &jobExecutionContext{
			ctx:   context.Background(),
			total: 1,
			ui:    ui,
			cfg:   &config{},
		},
		done: done,
	}

	resultCh := make(chan error, 1)
	go func() {
		_, _, _, err := controller.awaitCompletion()
		resultCh <- err
	}()

	ui.awaitWaitCall(t)
	select {
	case err := <-resultCh:
		t.Fatalf("awaitCompletion returned before explicit UI exit: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	ui.releaseWait(nil)
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("awaitCompletion returned unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("awaitCompletion did not finish after UI exit")
	}

	if ui.closeEventsCalls != 0 {
		t.Fatalf(
			"expected normal completion to keep events open until the UI exits, got %d close calls",
			ui.closeEventsCalls,
		)
	}
	if ui.shutdownCalls != 0 {
		t.Fatalf("expected normal completion not to force UI shutdown, got %d calls", ui.shutdownCalls)
	}
}

func TestEnsureRuntimeEventBusCreatesFallbackBusForUI(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "ui-fallback")
	if err := os.MkdirAll(filepath.Dir(runArtifacts.EventsPath), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	runJournal, err := journal.Open(runArtifacts.EventsPath, nil, 8)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runJournal.Close(closeCtx); err != nil {
			t.Fatalf("close journal: %v", err)
		}
	}()

	cfg := &config{
		OutputFormat: model.OutputFormatText,
		TUI:          true,
	}
	bus := ensureRuntimeEventBus(cfg, runJournal, nil)
	if bus == nil {
		t.Fatal("expected fallback bus for UI-enabled execution")
	}
	defer func() {
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}()

	_, ch, unsub := bus.Subscribe()
	defer unsub()

	ev, err := newRuntimeEvent(
		runArtifacts.RunID,
		eventspkg.EventKindRunStarted,
		kinds.RunStartedPayload{JobsTotal: 1},
	)
	if err != nil {
		t.Fatalf("new runtime event: %v", err)
	}
	if err := runJournal.Submit(context.Background(), ev); err != nil {
		t.Fatalf("submit event: %v", err)
	}

	select {
	case got := <-ch:
		if got.Kind != eventspkg.EventKindRunStarted {
			t.Fatalf("expected fallback bus to receive run.started, got %s", got.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fallback bus event")
	}
}

func TestEnsureRuntimeEventBusCreatesFallbackBusForWorkflowJSONStream(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "json-fallback")
	if err := os.MkdirAll(filepath.Dir(runArtifacts.EventsPath), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	runJournal, err := journal.Open(runArtifacts.EventsPath, nil, 8)
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runJournal.Close(closeCtx); err != nil {
			t.Fatalf("close journal: %v", err)
		}
	}()

	cfg := &config{OutputFormat: model.OutputFormatJSON}
	bus := ensureRuntimeEventBus(cfg, runJournal, nil)
	if bus == nil {
		t.Fatal("expected fallback bus for event-stream-enabled execution")
	}
	defer func() {
		if err := bus.Close(context.Background()); err != nil {
			t.Fatalf("close bus: %v", err)
		}
	}()

	_, ch, unsub := bus.Subscribe()
	defer unsub()

	ev, err := newRuntimeEvent(
		runArtifacts.RunID,
		eventspkg.EventKindRunStarted,
		kinds.RunStartedPayload{JobsTotal: 1},
	)
	if err != nil {
		t.Fatalf("new runtime event: %v", err)
	}
	if err := runJournal.Submit(context.Background(), ev); err != nil {
		t.Fatalf("submit event: %v", err)
	}

	select {
	case got := <-ch:
		if got.Kind != eventspkg.EventKindRunStarted {
			t.Fatalf("expected fallback bus to receive run.started, got %s", got.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fallback bus event")
	}
}

func TestExecutorControllerUIQuitEntersDrainingPath(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg, 4)}
	execCtx := &jobExecutionContext{
		ctx:           context.Background(),
		total:         2,
		cfg:           &config{RunArtifacts: model.RunArtifacts{RunID: runID}},
		ui:            ui,
		journal:       runJournal,
		activeClients: make(map[agent.Client]struct{}),
	}
	done := make(chan struct{})
	cancelCalls := 0
	controller := &executorController{
		ctx:              context.Background(),
		execCtx:          execCtx,
		cancelJobs:       func(error) { cancelCalls++ },
		done:             done,
		shutdownRequests: make(chan shutdownRequest, 4),
	}

	resultCh := make(chan error, 1)
	go func() {
		_, _, _, err := controller.awaitCompletion()
		resultCh <- err
	}()

	controller.requestShutdown(uiQuitRequestDrain)

	events := collectRuntimeEvents(t, eventsCh, 2)
	if got := events[0].Kind; got != eventspkg.EventKindShutdownRequested {
		t.Fatalf("expected shutdown.requested, got %s", got)
	}
	if got := events[1].Kind; got != eventspkg.EventKindShutdownDraining {
		t.Fatalf("expected shutdown.draining, got %s", got)
	}

	if cancelCalls != 1 {
		t.Fatalf("expected one cancel request, got %d", cancelCalls)
	}

	close(done)
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("awaitCompletion returned unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for draining controller completion")
	}
	if ui.shutdownCalls != 1 {
		t.Fatalf("expected draining path to shut down the UI, got %d calls", ui.shutdownCalls)
	}
}

func TestExecutorControllerSecondQuitForcesActiveClients(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg, 8)}
	client := newFakeACPClient(nil)
	execCtx := &jobExecutionContext{
		ctx:     context.Background(),
		total:   1,
		cfg:     &config{RunArtifacts: model.RunArtifacts{RunID: runID}},
		ui:      ui,
		journal: runJournal,
		activeClients: map[agent.Client]struct{}{
			client: {},
		},
	}
	done := make(chan struct{})
	controller := &executorController{
		ctx:              context.Background(),
		execCtx:          execCtx,
		cancelJobs:       func(error) {},
		done:             done,
		shutdownRequests: make(chan shutdownRequest, 4),
	}

	resultCh := make(chan error, 1)
	go func() {
		_, _, _, err := controller.awaitCompletion()
		resultCh <- err
	}()

	controller.requestShutdown(uiQuitRequestDrain)
	controller.requestShutdown(uiQuitRequestForce)

	events := collectRuntimeEvents(t, eventsCh, 2)
	if got := events[0].Kind; got != eventspkg.EventKindShutdownRequested {
		t.Fatalf("expected first shutdown event to be shutdown.requested, got %s", got)
	}
	if got := events[1].Kind; got != eventspkg.EventKindShutdownDraining {
		t.Fatalf("expected second shutdown event to be shutdown.draining, got %s", got)
	}

	if got := client.killCalls.Load(); got != 1 {
		t.Fatalf("expected force quit to kill active ACP clients, got %d kills", got)
	}

	close(done)
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("awaitCompletion returned unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for forced controller completion")
	}

	events = collectRuntimeEvents(t, eventsCh, 1)
	if got := events[0].Kind; got != eventspkg.EventKindShutdownTerminated {
		t.Fatalf("expected terminal shutdown event, got %s", got)
	}

	var payload kinds.ShutdownTerminatedPayload
	decodeRuntimeEventPayload(t, events[0], &payload)
	if !payload.Forced {
		t.Fatalf("expected forced shutdown payload, got %#v", payload)
	}
}

func TestExecutorControllerRootContextCancellationPublishesDrainingState(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg, 4)}
	execCtx := &jobExecutionContext{
		ctx:           context.Background(),
		total:         1,
		cfg:           &config{RunArtifacts: model.RunArtifacts{RunID: runID}},
		ui:            ui,
		journal:       runJournal,
		activeClients: make(map[agent.Client]struct{}),
	}
	done := make(chan struct{})
	cancelCalls := 0
	ctx, cancel := context.WithCancel(context.Background())
	controller := &executorController{
		ctx:              ctx,
		execCtx:          execCtx,
		cancelJobs:       func(error) { cancelCalls++ },
		done:             done,
		shutdownRequests: make(chan shutdownRequest, 4),
	}

	resultCh := make(chan error, 1)
	go func() {
		_, _, _, err := controller.awaitCompletion()
		resultCh <- err
	}()

	cancel()

	events := collectRuntimeEvents(t, eventsCh, 2)
	if got := events[0].Kind; got != eventspkg.EventKindShutdownRequested {
		t.Fatalf("expected shutdown.requested, got %s", got)
	}
	if got := events[1].Kind; got != eventspkg.EventKindShutdownDraining {
		t.Fatalf("expected shutdown.draining, got %s", got)
	}

	var payload kinds.ShutdownDrainingPayload
	decodeRuntimeEventPayload(t, events[1], &payload)
	if got := payload.Source; got != string(shutdownSourceSignal) {
		t.Fatalf("expected signal-sourced drain payload, got %s", got)
	}

	if cancelCalls != 1 {
		t.Fatalf("expected one cancel request after root context cancellation, got %d", cancelCalls)
	}

	close(done)
	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("awaitCompletion returned unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for controller completion after root cancellation")
	}
}

func TestExecutorControllerSuppressesFallbackShutdownLogsWhileUIIsActive(t *testing.T) {
	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg, 4)}
	controller := &executorController{
		ctx: context.Background(),
		execCtx: &jobExecutionContext{
			ctx:   context.Background(),
			cfg:   &config{OutputFormat: model.OutputFormatText},
			ui:    ui,
			total: 1,
		},
		cancelJobs: func(error) {},
		state:      executorStateRunning,
	}

	stdout, stderr, err := captureExecuteStreams(t, func() error {
		if !controller.beginDrain(shutdownSourceSignal) {
			t.Fatal("expected drain to start")
		}
		controller.beginForce(shutdownSourceSignal)
		_, _, _, doneErr := controller.handleDone(nil)
		return doneErr
	})
	if err != nil {
		t.Fatalf("handleDone: %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout fallback output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected UI mode to suppress stderr fallback output, got %q", stderr)
	}
}

func TestExecutorControllerUsesNeutralFallbackCompletionMessage(t *testing.T) {
	controller := &executorController{
		ctx: context.Background(),
		execCtx: &jobExecutionContext{
			ctx:   context.Background(),
			cfg:   &config{OutputFormat: model.OutputFormatText},
			total: 1,
		},
		state: executorStateForcing,
	}

	_, stderr, err := captureExecuteStreams(t, func() error {
		_, _, _, doneErr := controller.handleDone(nil)
		return doneErr
	})
	if err != nil {
		t.Fatalf("handleDone: %v", err)
	}
	if !strings.Contains(stderr, "Controller shutdown complete after shutdown grace period") {
		t.Fatalf("expected neutral shutdown completion message, got %q", stderr)
	}
	if strings.Contains(stderr, "gracefully") {
		t.Fatalf("expected forced shutdown message to avoid graceful wording, got %q", stderr)
	}
}

type fakeUISession struct {
	ch               chan uiMsg
	waitCalled       chan struct{}
	waitRelease      chan error
	closeEventsCalls int
	shutdownCalls    int
	mu               sync.Mutex
}

func newFakeUISession() *fakeUISession {
	return &fakeUISession{
		ch:          make(chan uiMsg, 8),
		waitCalled:  make(chan struct{}, 1),
		waitRelease: make(chan error, 1),
	}
}

func (f *fakeUISession) Enqueue(msg any) {
	select {
	case f.ch <- msg:
	default:
	}
}

func (f *fakeUISession) SetQuitHandler(func(uiQuitRequest)) {}

func (f *fakeUISession) CloseEvents() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeEventsCalls++
}

func (f *fakeUISession) Shutdown() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutdownCalls++
}

func (f *fakeUISession) Wait() error {
	select {
	case f.waitCalled <- struct{}{}:
	default:
	}
	return <-f.waitRelease
}

func (f *fakeUISession) awaitWaitCall(t *testing.T) {
	t.Helper()

	select {
	case <-f.waitCalled:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ui wait invocation")
	}
}

func (f *fakeUISession) releaseWait(err error) {
	f.waitRelease <- err
}
