package profiling

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultSeconds = 30
const MaxSeconds = 120

const (
	blockKind = "block"
	cpuKind   = "cpu"
	mutexKind = "mutex"
)

var ErrBusy = errors.New("profiling: capture already active")
var ErrInvalid = errors.New("profiling: invalid capture")

var cpuCapture sync.Mutex
var contentionCapture sync.Mutex
var active atomic.Int64

func Windowed(kind string) bool {
	return kind == cpuKind || kind == mutexKind || kind == blockKind
}

func Validate(kind string, seconds int) error {
	switch kind {
	case cpuKind, "heap", "allocs", "goroutine", mutexKind, blockKind, "threadcreate":
	default:
		return fmt.Errorf("%w: unknown profile kind %q", ErrInvalid, kind)
	}
	if seconds < 1 || seconds > MaxSeconds {
		return fmt.Errorf("%w: seconds must be between 1 and %d", ErrInvalid, MaxSeconds)
	}
	return nil
}

func Capture(ctx context.Context, dst io.Writer, kind string, seconds int) error {
	if err := Validate(kind, seconds); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	active.Add(1)
	defer active.Add(-1)
	if kind == cpuKind {
		return captureCPU(ctx, dst, seconds)
	}
	if kind == mutexKind || kind == blockKind {
		if !contentionCapture.TryLock() {
			return ErrBusy
		}
		defer contentionCapture.Unlock()
		if kind == mutexKind {
			previous := runtime.SetMutexProfileFraction(1)
			defer runtime.SetMutexProfileFraction(previous)
		} else {
			runtime.SetBlockProfileRate(1)
			defer runtime.SetBlockProfileRate(0)
		}
		if err := wait(ctx, seconds); err != nil {
			return err
		}
	}
	if err := pprof.Lookup(kind).WriteTo(dst, 0); err != nil {
		return fmt.Errorf("profiling: write %s: %w", kind, err)
	}
	return nil
}

func captureCPU(ctx context.Context, dst io.Writer, seconds int) error {
	if !cpuCapture.TryLock() {
		return ErrBusy
	}
	defer cpuCapture.Unlock()
	if err := pprof.StartCPUProfile(dst); err != nil {
		return fmt.Errorf("%w: %s", ErrBusy, err)
	}
	defer pprof.StopCPUProfile()
	return wait(ctx, seconds)
}

func wait(ctx context.Context, seconds int) error {
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func Enabled() bool { return active.Load() > 0 }

// Do avoids label allocations and runtime label updates outside a capture.
func Do(ctx context.Context, fn func(context.Context), labels ...string) {
	if active.Load() == 0 {
		fn(ctx)
		return
	}
	pprof.Do(ctx, pprof.Labels(labels...), fn)
}
