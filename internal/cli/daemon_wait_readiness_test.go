package cli

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/testutil"
)

type timeoutDaemonProcess struct {
	waitCalls      atomic.Int32
	terminateCalls atomic.Int32
	done           chan struct{}
	waitErr        error
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

func (p *timeoutDaemonProcess) Terminate() error {
	p.terminateCalls.Add(1)
	select {
	case <-p.done:
	default:
		p.complete(nil)
	}
	return nil
}

func (p *timeoutDaemonProcess) complete(err error) {
	p.waitErr = err
	close(p.done)
}

// TestWaitForDaemonStartReadiness covers readiness, actual exit, and cancellation across polling windows.
func TestWaitForDaemonStartReadiness(t *testing.T) {
	t.Parallel()

	t.Run("Should wait through multiple readiness windows for a live child", func(t *testing.T) {
		t.Parallel()
		child := &timeoutDaemonProcess{done: make(chan struct{})}
		var calls atomic.Int32
		deps := newTestDeps(t, &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
			if calls.Add(1) < 20 {
				return DaemonStatus{}, errors.New("booting")
			}
			return DaemonStatus{Status: daemonRunningStatus, PID: child.PID()}, nil
		}})
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 3 * time.Millisecond
		deps.processAlive = func(int) bool { return true }
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop := context.AfterFunc(t.Context(), cancel)
		defer stop()
		status, err := waitForDaemonStart(ctx, deps, child)
		child.complete(nil)
		if err != nil || status.PID != child.PID() || status.Status != daemonRunningStatus {
			t.Fatalf("waitForDaemonStart() = %#v, %v; want eventual readiness", status, err)
		}
		if child.terminateCalls.Load() != 0 {
			t.Fatal("live child was terminated")
		}
	})

	t.Run("Should stop observing at the caller deadline without waiting for the live child", func(t *testing.T) {
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

		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		_, err := waitForDaemonStart(ctx, deps, child)
		if err == nil || !strings.Contains(err.Error(), "daemon did not become ready before timeout") {
			t.Fatalf("waitForDaemonStart() error = %v, want readiness timeout", err)
		}
		if calls := child.waitCalls.Load(); calls != 0 {
			t.Fatalf("process.Wait() calls = %d, want 0 before child exit", calls)
		}
		child.complete(nil)
	})

	t.Run("Should reap an actual child exit after the first readiness window", func(t *testing.T) {
		t.Parallel()
		child := &timeoutDaemonProcess{done: make(chan struct{})}
		cause := errors.New("boot failed after migration")
		var calls atomic.Int32
		deps := newTestDeps(t, &stubClient{daemonStatusFn: func(context.Context) (DaemonStatus, error) {
			if calls.Add(1) == 12 {
				child.complete(cause)
			}
			return DaemonStatus{}, errors.New("booting")
		}})
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 3 * time.Millisecond
		deps.processAlive = func(int) bool { return true }
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop := context.AfterFunc(t.Context(), cancel)
		defer stop()
		_, err := waitForDaemonStart(ctx, deps, child)
		if !errors.Is(err, cause) {
			t.Fatalf("waitForDaemonStart() = %v; want actual boot failure", err)
		}
		if child.waitCalls.Load() != 1 || child.terminateCalls.Load() != 0 {
			t.Fatal("actual exit was not reaped without termination")
		}
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

// TestStalledDaemonObservationReleasesMutationLock verifies cancellation releases ownership without killing the child.
func TestStalledDaemonObservationReleasesMutationLock(t *testing.T) {
	t.Parallel()
	t.Run("Should release the startup lock on cancellation without terminating the live daemon", func(t *testing.T) {
		t.Parallel()
		child := &timeoutDaemonProcess{done: make(chan struct{})}
		deps := newTestDeps(
			t,
			&stubClient{
				daemonStatusFn: func(context.Context) (DaemonStatus, error) { return DaemonStatus{}, errors.New("boot stalled") },
			},
		)
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) { return compozydaemon.Info{}, os.ErrNotExist }
		paths, err := deps.resolveHome()
		if err != nil {
			t.Fatal(err)
		}
		deps.spawnDetached = func(context.Context, compozyconfig.HomePaths) (daemonProcess, error) {
			if lock, err := acquireDaemonStartUpdateLock(daemonStartUpdateLockPath(paths)); err == nil {
				if releaseErr := lock.Release(); releaseErr != nil {
					t.Error(releaseErr)
				}
				t.Fatal("startup mutation lock was not held during spawn")
			}
			return child, nil
		}
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 3 * time.Millisecond
		deps.processAlive = func(int) bool { return true }
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
		defer cancel()
		_, err = runDaemonDetached(ctx, deps)
		if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "may still be starting") {
			t.Fatalf("runDaemonDetached() = %v", err)
		}
		if child.terminateCalls.Load() != 0 {
			t.Fatal("canceled observer terminated live daemon")
		}
		child.complete(nil)
		lock, err := acquireDaemonStartUpdateLock(daemonStartUpdateLockPath(paths))
		if err != nil {
			t.Fatalf("startup lock retained after cancellation: %v", err)
		}
		if err := lock.Release(); err != nil {
			t.Fatal(err)
		}
	})
}
