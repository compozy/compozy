package terminal

// Suite: terminal wait engine.
// Invariant: waits wake from output/exit revisions and report observed state without inventing success or failure.
// Boundary IN: process/output state changes. Boundary OUT: WaitResult reasons.

import (
	"context"
	"testing"
	"time"
)

func TestWaitEngineContract(t *testing.T) {
	t.Parallel()

	t.Run("Should return the process exit code after durable finalization [UT-050]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		code := 7
		starter.latest().complete(terminalExit("exited", &code, nil))
		result, err := handle.Wait(context.Background(), WaitCondition{Until: "exit"})
		if err != nil {
			t.Fatalf("Wait(exit) error = %v", err)
		}
		if result.Reason != "exit" || result.ExitCode == nil || *result.ExitCode != 7 {
			t.Fatalf("Wait(exit) = %#v", result)
		}
	})

	t.Run("Should wake on a matching output revision without timer polling [UT-051]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		type waitOutcome struct {
			result *WaitResult
			err    error
		}
		waitCtx, cancelWait := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancelWait()
		resultDone := make(chan waitOutcome, 1)
		waitStarted := make(chan struct{})
		go func() {
			close(waitStarted)
			result, err := handle.Wait(waitCtx, WaitCondition{Until: "match", Pattern: "ready-[0-9]+"})
			resultDone <- waitOutcome{result: result, err: err}
		}()
		<-waitStarted
		if err := starter.latest().emit([]byte("ready-42\r\n")); err != nil {
			t.Fatalf("emit() error = %v", err)
		}
		select {
		case outcome := <-resultDone:
			if outcome.err != nil || outcome.result.Reason != "match" {
				t.Fatalf("Wait(match) = %#v error=%v", outcome.result, outcome.err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("matching output did not wake wait")
		}
	})

	t.Run("Should distinguish timeout idle and still-running states [UT-052]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		timedOut, err := handle.Wait(context.Background(), WaitCondition{Until: "exit", TimeoutMs: 25})
		if err != nil || timedOut.Reason != "timeout" {
			t.Fatalf("Wait(timeout) = %#v error=%v", timedOut, err)
		}
		idle, err := handle.Wait(context.Background(), WaitCondition{Until: "idle", TimeoutMs: 600})
		if err != nil || idle.Reason != "idle" {
			t.Fatalf("Wait(idle) = %#v error=%v", idle, err)
		}

		stop := make(chan struct{})
		tickerDone := make(chan struct{})
		go func() {
			defer close(tickerDone)
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					if emitErr := starter.latest().emit([]byte(".")); emitErr != nil {
						return
					}
				}
			}
		}()
		holding, err := handle.Wait(context.Background(), WaitCondition{Until: "idle"})
		close(stop)
		<-tickerDone
		if err != nil || holding.Reason != "still_running" {
			t.Fatalf("Wait(holding) = %#v error=%v", holding, err)
		}
	})

	t.Run("Should report a dead output stream as stalled and enforce idle debounce [UT-054]", func(t *testing.T) {
		t.Parallel()
		manager, _, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		item := handle.(*session)
		item.markReaderEnded()
		result, err := handle.Wait(context.Background(), WaitCondition{Until: "match", Pattern: "never"})
		if err != nil || result.Reason != "stalled" {
			t.Fatalf("Wait(stalled) = %#v error=%v", result, err)
		}

		managerTwo, _, _ := newTestManager(t, DefaultSettings())
		handleTwo := openTestTerminal(t, managerTwo, "workspace-a", "profile-a")
		started := time.Now()
		result, err = handleTwo.Wait(context.Background(), WaitCondition{Until: "idle"})
		elapsed := time.Since(started)
		const expectedIdleDebounce = 300 * time.Millisecond
		const expectedIdleMaximum = 700 * time.Millisecond
		const schedulerTolerance = 75 * time.Millisecond
		if err != nil || result.Reason != "idle" || elapsed < expectedIdleDebounce-schedulerTolerance ||
			elapsed > expectedIdleMaximum+schedulerTolerance {
			t.Fatalf("Wait(idle) = %#v error=%v elapsed=%s", result, err, elapsed)
		}
	})
}
