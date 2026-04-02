package run

import (
	"context"
	"sync"
	"testing"
	"time"
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

func (f *fakeUISession) setQuitHandler(func()) {}

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
