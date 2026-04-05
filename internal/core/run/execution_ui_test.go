package run

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
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

	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg, 4)}
	execCtx := &jobExecutionContext{
		total:         2,
		ui:            ui,
		uiCh:          ui.eventsCh,
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

	select {
	case msg := <-ui.eventsCh:
		status, ok := msg.(shutdownStatusMsg)
		if !ok {
			t.Fatalf("expected shutdown status message, got %T", msg)
		}
		if got := status.State.Phase; got != shutdownPhaseDraining {
			t.Fatalf("expected draining phase, got %s", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for draining status")
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

	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg, 8)}
	client := newFakeACPClient(nil)
	execCtx := &jobExecutionContext{
		total: 1,
		ui:    ui,
		uiCh:  ui.eventsCh,
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

	var sawDrain bool
	var sawForce bool
	for !sawDrain || !sawForce {
		select {
		case msg := <-ui.eventsCh:
			status, ok := msg.(shutdownStatusMsg)
			if !ok {
				continue
			}
			switch status.State.Phase {
			case shutdownPhaseDraining:
				sawDrain = true
			case shutdownPhaseForcing:
				sawForce = true
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for shutdown status transitions")
		}
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
}

func TestExecutorControllerRootContextCancellationPublishesDrainingState(t *testing.T) {
	t.Parallel()

	ui := &fakeLifecycleUISession{eventsCh: make(chan uiMsg, 4)}
	execCtx := &jobExecutionContext{
		total:         1,
		ui:            ui,
		uiCh:          ui.eventsCh,
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

	select {
	case msg := <-ui.eventsCh:
		status, ok := msg.(shutdownStatusMsg)
		if !ok {
			t.Fatalf("expected shutdown status message, got %T", msg)
		}
		if got := status.State.Source; got != shutdownSourceSignal {
			t.Fatalf("expected signal-sourced drain status, got %s", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for signal-driven drain status")
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
			uiCh:  ui.eventsCh,
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
