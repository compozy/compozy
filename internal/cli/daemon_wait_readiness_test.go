package cli

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

type timeoutDaemonProcess struct {
	waitCalls atomic.Int32
	done      chan struct{}
	waitErr   error
}

func (p *timeoutDaemonProcess) PID() int {
	return 42
}

func (p *timeoutDaemonProcess) Done() <-chan struct{} {
	return p.done
}

func (p *timeoutDaemonProcess) Wait() error {
	p.waitCalls.Add(1)
	<-p.done
	return p.waitErr
}

func (p *timeoutDaemonProcess) complete(err error) {
	p.waitErr = err
	close(p.done)
}

func TestWaitForDaemonStartReadiness(t *testing.T) {
	t.Parallel()

	t.Run("Should timeout when child stays alive and readiness never succeeds", func(t *testing.T) {
		t.Parallel()

		child := &timeoutDaemonProcess{done: make(chan struct{})}
		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{}, errors.New("daemon unavailable")
			},
		})
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 5 * time.Millisecond
		deps.processAlive = func(int) bool { return true }

		_, err := waitForDaemonStart(testutil.Context(t), deps, child)
		if err == nil || !strings.Contains(err.Error(), "daemon did not become ready before timeout") {
			t.Fatalf("waitForDaemonStart() error = %v, want readiness timeout", err)
		}
		if calls := child.waitCalls.Load(); calls != 0 {
			t.Fatalf("process.Wait() calls = %d, want 0 before child exit", calls)
		}
		child.complete(nil)
	})

	t.Run("Should return child wait error when detached daemon exits before readiness", func(t *testing.T) {
		t.Parallel()

		waitErr := errors.New("exit status 2")
		child := &timeoutDaemonProcess{done: make(chan struct{})}
		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{}, errors.New("daemon unavailable")
			},
		})
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 100 * time.Millisecond
		deps.processAlive = func(int) bool { return true }
		child.complete(waitErr)

		_, err := waitForDaemonStart(testutil.Context(t), deps, child)
		if !errors.Is(err, waitErr) {
			t.Fatalf("waitForDaemonStart() error = %v, want child wait error", err)
		}
		if !strings.Contains(err.Error(), "detached daemon exited before readiness") {
			t.Fatalf("waitForDaemonStart() error = %v, want detached exit context", err)
		}
		if calls := child.waitCalls.Load(); calls != 1 {
			t.Fatalf("process.Wait() calls = %d, want 1", calls)
		}
	})
}
