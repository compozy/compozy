package hooks

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewAsyncPoolAppliesDefaults(t *testing.T) {
	t.Parallel()

	pool := newAsyncPool(asyncPoolConfig{})
	if pool.workerCount != defaultAsyncWorkerCount {
		t.Fatalf("workerCount = %d, want %d", pool.workerCount, defaultAsyncWorkerCount)
	}
	if pool.queueCapacity != defaultAsyncQueueCapacity {
		t.Fatalf("queueCapacity = %d, want %d", pool.queueCapacity, defaultAsyncQueueCapacity)
	}
	if pool.drainTimeout != defaultAsyncDrainTimeout {
		t.Fatalf("drainTimeout = %s, want %s", pool.drainTimeout, defaultAsyncDrainTimeout)
	}
	if pool.logger == nil {
		t.Fatal("logger = nil, want non-nil")
	}
}

func TestAsyncPoolStartsConfiguredWorkers(t *testing.T) {
	t.Parallel()

	const workers = 3
	pool := newAsyncPool(asyncPoolConfig{
		WorkerCount:   workers,
		QueueCapacity: workers,
		Logger:        discardPoolLogger(),
	})
	pool.Start(t.Context())

	started := make(chan struct{}, workers)
	release := make(chan struct{})
	for i := range workers {
		if ok := pool.Submit(asyncTask{
			run: func(context.Context) {
				started <- struct{}{}
				<-release
			},
		}); !ok {
			t.Fatalf("Submit() #%d = false, want true", i)
		}
	}

	for range workers {
		waitForPoolSignal(t, started, "worker start")
	}

	close(release)
	pool.Close()
}

func TestAsyncPoolSubmitWithAvailableCapacitySucceeds(t *testing.T) {
	t.Parallel()

	pool := newAsyncPool(asyncPoolConfig{
		WorkerCount:   1,
		QueueCapacity: 1,
		Logger:        discardPoolLogger(),
	})
	pool.Start(t.Context())

	done := make(chan struct{})
	if ok := pool.Submit(asyncTask{
		run: func(context.Context) {
			close(done)
		},
	}); !ok {
		t.Fatal("Submit() = false, want true")
	}

	waitForPoolSignal(t, done, "task completion")
	pool.Close()
}

func TestAsyncPoolSubmitDropsWhenQueueIsFullAndLogsDepth(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	pool := newAsyncPool(asyncPoolConfig{
		WorkerCount:   1,
		QueueCapacity: 1,
		Logger:        logger,
	})
	pool.Start(t.Context())

	started := make(chan struct{})
	release := make(chan struct{})
	if ok := pool.Submit(asyncTask{
		hook: RegisteredHook{
			Name:   "hook-1",
			Event:  HookEventPostRecord,
			Source: HookSourceSkill,
		},
		run: func(context.Context) {
			close(started)
			<-release
		},
	}); !ok {
		t.Fatal("Submit() first task = false, want true")
	}
	waitForPoolSignal(t, started, "first task start")

	if ok := pool.Submit(asyncTask{
		hook: RegisteredHook{
			Name:   "hook-2",
			Event:  HookEventPostRecord,
			Source: HookSourceSkill,
		},
		run: func(context.Context) {},
	}); !ok {
		t.Fatal("Submit() queued task = false, want true")
	}

	if ok := pool.Submit(asyncTask{
		hook: RegisteredHook{
			Name:   "hook-3",
			Event:  HookEventPostRecord,
			Source: HookSourceSkill,
		},
		run: func(context.Context) {},
	}); ok {
		t.Fatal("Submit() overflow task = true, want false")
	}

	logOutput := logs.String()
	if !bytes.Contains([]byte(logOutput), []byte("hook.dispatch.async_dropped")) {
		t.Fatalf("logs = %q, want async_dropped entry", logOutput)
	}
	if !bytes.Contains([]byte(logOutput), []byte("queue_depth=1")) {
		t.Fatalf("logs = %q, want queue_depth=1", logOutput)
	}
	if !bytes.Contains([]byte(logOutput), []byte("hook=hook-3")) {
		t.Fatalf("logs = %q, want dropped hook name", logOutput)
	}

	close(release)
	pool.Close()
}

func TestAsyncPoolCloseDrainsQueuedTasksBeforeReturning(t *testing.T) {
	t.Parallel()

	pool := newAsyncPool(asyncPoolConfig{
		WorkerCount:   1,
		QueueCapacity: 2,
		Logger:        discardPoolLogger(),
	})
	pool.Start(t.Context())

	var ran atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	if ok := pool.Submit(asyncTask{
		run: func(context.Context) {
			ran.Add(1)
			close(started)
			<-release
		},
	}); !ok {
		t.Fatal("Submit() first task = false, want true")
	}
	waitForPoolSignal(t, started, "first task start")

	completed := make(chan struct{}, 2)
	for i := range 2 {
		if ok := pool.Submit(asyncTask{
			run: func(context.Context) {
				ran.Add(1)
				completed <- struct{}{}
			},
		}); !ok {
			t.Fatalf("Submit() queued task #%d = false, want true", i)
		}
	}

	closed := make(chan struct{})
	go func() {
		pool.Close()
		close(closed)
	}()

	select {
	case <-closed:
		t.Fatal("Close() returned before queued tasks were drainable")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	waitForPoolSignal(t, completed, "second task completion")
	waitForPoolSignal(t, completed, "third task completion")
	waitForPoolSignal(t, closed, "pool close")

	if got := ran.Load(); got != 3 {
		t.Fatalf("ran = %d, want 3", got)
	}
}

func TestAsyncPoolCloseCancelsAfterDrainDeadline(t *testing.T) {
	t.Parallel()

	pool := newAsyncPool(asyncPoolConfig{
		WorkerCount:   1,
		QueueCapacity: 1,
		DrainTimeout:  40 * time.Millisecond,
		Logger:        discardPoolLogger(),
	})
	pool.Start(t.Context())

	started := make(chan struct{})
	canceled := make(chan struct{})
	if ok := pool.Submit(asyncTask{
		run: func(ctx context.Context) {
			close(started)
			<-ctx.Done()
			close(canceled)
		},
	}); !ok {
		t.Fatal("Submit() first task = false, want true")
	}
	waitForPoolSignal(t, started, "first task start")

	var queuedRan atomic.Bool
	if ok := pool.Submit(asyncTask{
		run: func(context.Context) {
			queuedRan.Store(true)
		},
	}); !ok {
		t.Fatal("Submit() queued task = false, want true")
	}

	start := time.Now()
	pool.Close()
	elapsed := time.Since(start)

	waitForPoolSignal(t, canceled, "task cancellation")
	if queuedRan.Load() {
		t.Fatal("queued task ran after drain deadline, want abandoned")
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("Close() elapsed = %s, want at least drain timeout", elapsed)
	}
	if elapsed > time.Second {
		t.Fatalf("Close() elapsed = %s, want prompt deadline handling", elapsed)
	}
}

func TestAsyncPoolRecoversPanicsAndContinues(t *testing.T) {
	t.Parallel()

	pool := newAsyncPool(asyncPoolConfig{
		WorkerCount:   1,
		QueueCapacity: 1,
		Logger:        discardPoolLogger(),
	})
	pool.Start(t.Context())

	started := make(chan struct{})
	if ok := pool.Submit(asyncTask{
		run: func(context.Context) {
			close(started)
			panic("boom")
		},
	}); !ok {
		t.Fatal("Submit() panic task = false, want true")
	}
	waitForPoolSignal(t, started, "panic task start")

	done := make(chan struct{})
	if ok := pool.Submit(asyncTask{
		run: func(context.Context) {
			close(done)
		},
	}); !ok {
		t.Fatal("Submit() recovery task = false, want true")
	}

	waitForPoolSignal(t, done, "post-panic task completion")
	pool.Close()
}

func TestAsyncPoolContextCancellationStopsWorkers(t *testing.T) {
	t.Parallel()

	parent, cancel := context.WithCancel(t.Context())
	pool := newAsyncPool(asyncPoolConfig{
		WorkerCount:   2,
		QueueCapacity: 2,
		Logger:        discardPoolLogger(),
	})
	pool.Start(parent)

	started := make(chan struct{}, 2)
	stopped := make(chan struct{}, 2)
	for i := range 2 {
		if ok := pool.Submit(asyncTask{
			run: func(ctx context.Context) {
				started <- struct{}{}
				<-ctx.Done()
				stopped <- struct{}{}
			},
		}); !ok {
			t.Fatalf("Submit() #%d = false, want true", i)
		}
	}

	waitForPoolSignal(t, started, "first worker start")
	waitForPoolSignal(t, started, "second worker start")
	cancel()
	waitForPoolSignal(t, stopped, "first worker stop")
	waitForPoolSignal(t, stopped, "second worker stop")
	pool.Close()
}

func TestAsyncPoolCloseWithNoTasksReturnsCleanly(t *testing.T) {
	t.Parallel()

	pool := newAsyncPool(asyncPoolConfig{
		WorkerCount:   2,
		QueueCapacity: 4,
		Logger:        discardPoolLogger(),
	})
	pool.Start(t.Context())

	closed := make(chan struct{})
	go func() {
		pool.Close()
		close(closed)
	}()

	waitForPoolSignal(t, closed, "pool close without tasks")
}

func TestAsyncPoolConcurrentSubmitIsSafe(t *testing.T) {
	t.Parallel()

	const (
		goroutines   = 20
		tasksPerGoro = 10
		totalTasks   = goroutines * tasksPerGoro
	)

	pool := newAsyncPool(asyncPoolConfig{
		WorkerCount:   4,
		QueueCapacity: totalTasks,
		Logger:        discardPoolLogger(),
	})
	pool.Start(t.Context())

	var dropped atomic.Int32
	var ran atomic.Int32
	var submitWG sync.WaitGroup
	submitWG.Add(goroutines)
	for range goroutines {
		go func() {
			defer submitWG.Done()
			for range tasksPerGoro {
				if ok := pool.Submit(asyncTask{
					run: func(context.Context) {
						ran.Add(1)
					},
				}); !ok {
					dropped.Add(1)
				}
			}
		}()
	}

	submitWG.Wait()
	pool.Close()

	if got := dropped.Load(); got != 0 {
		t.Fatalf("dropped = %d, want 0", got)
	}
	if got := ran.Load(); got != totalTasks {
		t.Fatalf("ran = %d, want %d", got, totalTasks)
	}
}

func discardPoolLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitForPoolSignal(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", label)
	}
}
