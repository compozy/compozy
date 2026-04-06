package run

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
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

	if ui.closeEventsCalls != 1 {
		t.Fatalf("expected closeEvents to be called once, got %d", ui.closeEventsCalls)
	}
	if ui.shutdownCalls != 0 {
		t.Fatalf("expected normal completion not to force UI shutdown, got %d calls", ui.shutdownCalls)
	}
}

func TestExecutorControllerUIQuitEntersDrainingPath(t *testing.T) {
	t.Parallel()

	runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
	defer cleanup()

	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg, 4)}
	execCtx := &jobExecutionContext{
		total:         2,
		cfg:           &config{runArtifacts: model.RunArtifacts{RunID: runID}},
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
		total:   1,
		cfg:     &config{runArtifacts: model.RunArtifacts{RunID: runID}},
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
		total:         1,
		cfg:           &config{runArtifacts: model.RunArtifacts{RunID: runID}},
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
	t.Parallel()

	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg, 4)}
	controller := &executorController{
		ctx: context.Background(),
		execCtx: &jobExecutionContext{
			cfg:   &config{outputFormat: model.OutputFormatText},
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
	t.Parallel()

	controller := &executorController{
		ctx: context.Background(),
		execCtx: &jobExecutionContext{
			cfg:   &config{outputFormat: model.OutputFormatText},
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
		ch:          make(chan uiMsg),
		waitCalled:  make(chan struct{}, 1),
		waitRelease: make(chan error, 1),
	}
}

func (f *fakeUISession) events() chan uiMsg {
	return f.ch
}

func (f *fakeUISession) setQuitHandler(func(uiQuitRequest)) {}

func (f *fakeUISession) closeEvents() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeEventsCalls++
}

func (f *fakeUISession) shutdown() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutdownCalls++
}

func (f *fakeUISession) wait() error {
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
