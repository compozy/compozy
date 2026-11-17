package terminal

// Suite: terminal wait engine.
// Invariant: waits wake from output/exit revisions and report observed state without inventing success or failure.
// Boundary IN: process/output state changes. Boundary OUT: WaitResult reasons.

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"testing"
	"testing/synctest"
	"time"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

func TestWaitEngineContract(t *testing.T) {
	t.Parallel()

	t.Run("Should return the process exit code after durable finalization [UT-050]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		code := 7
		starter.latest().complete(terminalExit("exited", &code, nil))
		result, err := handle.Wait(t.Context(), WaitCondition{Until: "exit"})
		if err != nil {
			t.Fatalf("Wait(exit) error = %v", err)
		}
		if result.Reason != "exit" || result.ExitCode == nil || *result.ExitCode != 7 {
			t.Fatalf("Wait(exit) = %#v", result)
		}
	})

	t.Run("Should wake on a matching output revision without timer polling [UT-051]", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			manager, starter, _ := newTestManager(t, DefaultSettings())
			handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
			viewer, err := handle.Attach(t.Context(), AttachOptions{
				Mode: "read", Flow: "ack",
				Actor: Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
			})
			if err != nil {
				t.Fatalf("Attach() error = %v", err)
			}
			t.Cleanup(func() {
				if err := viewer.Close(); err != nil {
					t.Errorf("Close() error = %v", err)
				}
			})
			assertAttachedFrame(t, receiveSubscriptionFrame(t, viewer), 0, false)
			// Fill the ACK window before timing Wait; leave every output frame unacknowledged.
			if err := starter.latest().emit(bytes.Repeat([]byte("x"), terminalwire.AckHighWatermark)); err != nil {
				t.Fatalf("emit(stalled viewer setup) error = %v", err)
			}
			waitForTerminalTail(t, handle, "setup output did not fill the ACK window", func(tail *ReadResult) bool {
				return tail.Seq == terminalwire.AckHighWatermark &&
					viewer.(*subscription).queue.PendingBytes() >= terminalwire.AckHighWatermark
			})
			// Settle the emulator before measuring the revision wake under a stalled viewer.
			probe := []byte("settled\r\n")
			probeDone := make(chan error, 1)
			go func() { probeDone <- starter.latest().emit(probe) }()
			setupCtx, cancelSetup := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancelSetup()
			waitForTerminalTail(t, handle, "setup probe or screen did not settle", func(tail *ReadResult) bool {
				wantSeq := uint64(terminalwire.AckHighWatermark + len(probe))
				if tail.Seq != wantSeq {
					return false
				}
				screen, err := handle.Screen(setupCtx, ReadOptions{View: terminalViewScreen})
				if err != nil {
					t.Fatalf("Screen(stalled viewer setup) error = %v", err)
				}
				return !screen.Busy && screen.Seq == wantSeq
			})
			if err := <-probeDone; err != nil {
				t.Fatalf("emit(setup probe) error = %v", err)
			}
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
			emitDone := make(chan error, 1)
			go func() { emitDone <- starter.latest().emit([]byte("ready-42\r\n")) }()
			select {
			case outcome := <-resultDone:
				if outcome.err != nil || outcome.result.Reason != "match" {
					t.Fatalf("Wait(match) = %#v error=%v", outcome.result, outcome.err)
				}
				if err := <-emitDone; err != nil {
					t.Fatalf("emit() error = %v", err)
				}
			case <-time.After(200 * time.Millisecond):
				t.Fatal("matching output did not wake wait")
			}
		})
	})

	t.Run("Should match exactly like a full scan while rescanning only new output", func(t *testing.T) {
		t.Parallel()
		filler := bytes.Repeat([]byte("log line\n"), (80<<10)/9)
		cases := []struct {
			name    string
			pattern string
			chunks  []string
			want    []bool
		}{
			{
				name: "line-local pattern split across chunks", pattern: `ready-[0-9]+`,
				chunks: []string{"noise rea", "dy-42\r\n"}, want: []bool{false, true},
			},
			{
				name: "multiline pattern spanning more than the old overlap", pattern: `(?s)BEGIN.*READY`,
				chunks: []string{"BEGIN\n" + string(filler), "READY\n"}, want: []bool{false, true},
			},
			{
				name: "begin-of-text anchor never matches at a rescan boundary", pattern: `^ready`,
				chunks: []string{string(filler), "ready\n"}, want: []bool{false, false},
			},
			{
				name: "output retained before the wait", pattern: `ready-1`,
				chunks: []string{"ready-1 before the wait\n"}, want: []bool{true},
			},
		}
		for _, tc := range cases {
			matcher := regexp.MustCompile(tc.pattern)
			ring := NewRing(1 << 20)
			scan := newMatchScanner(matcher)
			for index, chunk := range tc.chunks {
				ring.Append([]byte(chunk))
				full, _ := ring.Snapshot()
				got := scan.matches(ring)
				if got != tc.want[index] || got != matcher.Match(full) {
					t.Fatalf("%s: chunk %d matches = %v, want %v (full scan %v)",
						tc.name, index, got, tc.want[index], matcher.Match(full))
				}
				if got {
					break // a wait returns on its first match
				}
			}
		}
	})

	t.Run("Should rescan only from the last line start for line-local patterns", func(t *testing.T) {
		t.Parallel()
		if !lineLocalPattern(`ready-[0-9]+$`) || lineLocalPattern(`(?s)a.*b`) || lineLocalPattern(`a\sb`) ||
			lineLocalPattern(`\Astart`) || lineLocalPattern(`[^x]+`) {
			t.Fatal("lineLocalPattern misclassified a pattern")
		}
		ring := NewRing(1 << 20)
		scan := newMatchScanner(regexp.MustCompile(`ready`))
		ring.Append([]byte("first line\nsecond partial"))
		if scan.matches(ring) {
			t.Fatal("matched before the pattern arrived")
		}
		if want := uint64(len("first line\n")); scan.lineStart != want {
			t.Fatalf("lineStart = %d, want %d", scan.lineStart, want)
		}
	})

	t.Run("Should distinguish timeout idle and still-running states [UT-052]", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		if err := starter.latest().emit([]byte("alpha\rZ")); err != nil {
			t.Fatalf("emit(before timeout) error = %v", err)
		}
		tail := waitForTerminalTail(
			t,
			handle,
			"timeout fixture output did not reach terminal tail",
			func(tail *ReadResult) bool {
				return tail.Content == "alpha\rZ"
			},
		)
		expectedScreen, err := handle.Screen(t.Context(), ReadOptions{View: terminalViewScreen})
		if err != nil {
			t.Fatalf("Screen(timeout fixture) error = %v", err)
		}
		if expectedScreen.Content == tail.Content {
			t.Fatalf(
				"timeout fixture screen/tail = %q/%q, want distinct VT rendering",
				expectedScreen.Content,
				tail.Content,
			)
		}
		timedOut, err := handle.Wait(t.Context(), WaitCondition{Until: "exit", TimeoutMs: 25})
		if err != nil || timedOut == nil || timedOut.Reason != "timeout" ||
			!timedOut.Untrusted || timedOut.Screen != expectedScreen.Content {
			t.Fatalf("Wait(timeout) = %#v error=%v", timedOut, err)
		}
		idle, err := handle.Wait(t.Context(), WaitCondition{Until: "idle", TimeoutMs: 600})
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
		holding, err := handle.Wait(t.Context(), WaitCondition{Until: "idle"})
		close(stop)
		<-tickerDone
		if err != nil || holding.Reason != "still_running" {
			t.Fatalf("Wait(holding) = %#v error=%v", holding, err)
		}
	})

	t.Run("Should preserve the caller cancellation cause", func(t *testing.T) {
		t.Parallel()
		manager, _, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		cause := errors.New("turn canceled")
		waitCtx, cancel := context.WithCancelCause(t.Context())
		cancel(cause)
		result, err := handle.Wait(waitCtx, WaitCondition{Until: "exit"})
		if result != nil || !errors.Is(err, cause) {
			t.Fatalf("Wait(canceled) = %#v error=%v, want caller cause", result, err)
		}
	})

	t.Run("Should report a dead output stream as stalled and enforce idle debounce [UT-054]", func(t *testing.T) {
		t.Parallel()
		manager, _, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		item := handle.(*session)
		item.markReaderEnded()
		result, err := handle.Wait(t.Context(), WaitCondition{Until: "match", Pattern: "never"})
		if err != nil || result.Reason != "stalled" {
			t.Fatalf("Wait(stalled) = %#v error=%v", result, err)
		}

		managerTwo, _, _ := newTestManager(t, DefaultSettings())
		handleTwo := openTestTerminal(t, managerTwo, "workspace-a", "profile-a")
		started := time.Now()
		result, err = handleTwo.Wait(t.Context(), WaitCondition{Until: "idle"})
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

func TestOutputCursorContract(t *testing.T) {
	t.Parallel()

	t.Run("Should observe only output printed after the cursor opens", func(t *testing.T) {
		t.Parallel()
		s := newOutputTestSession(1 << 10)
		appendOutputTest(s, "before\n")
		cursor := s.OpenOutputCursor()
		appendOutputTest(s, "after\n")
		batch := cursor.Next(t.Context(), 0)
		if batch.Text != "after\n" || batch.Lost != 0 || batch.Exit != nil || batch.Closed {
			t.Fatalf("Next() = %#v, want only the output printed after open", batch)
		}
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		if again := cursor.Next(canceled, 0); again.Text != "" || again.Lost != 0 {
			t.Fatalf("Next() after drain = %#v, want an empty batch on ctx end", again)
		}
	})

	t.Run("Should report a lagging cursor as a gap and never replay dropped output", func(t *testing.T) {
		t.Parallel()
		s := newOutputTestSession(16)
		cursor := s.OpenOutputCursor()
		appendOutputTest(s, "0123456789abcdef-tail")
		oldest, _ := s.ring.Bounds()
		batch := cursor.Next(t.Context(), 0)
		if oldest == 0 || batch.Lost != oldest {
			t.Fatalf("Next().Lost = %d, want the %d units the scrollback dropped", batch.Lost, oldest)
		}
		if batch.Text != "56789abcdef-tail" {
			t.Fatalf("Next().Text = %q, want only the retained suffix", batch.Text)
		}
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		if again := cursor.Next(canceled, 0); again.Lost != 0 || again.Text != "" {
			t.Fatalf("second Next() = %#v, want the gap reported once", again)
		}
	})

	t.Run("Should batch output printed inside the settle window into one read", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			s := newOutputTestSession(1 << 10)
			cursor := s.OpenOutputCursor()
			done := make(chan OutputBatch, 1)
			go func() { done <- cursor.Next(t.Context(), 1500*time.Millisecond) }()
			appendOutputTest(s, "first\n")
			time.Sleep(500 * time.Millisecond)
			appendOutputTest(s, "second\n")
			time.Sleep(999 * time.Millisecond)
			synctest.Wait()
			select {
			case batch := <-done:
				t.Fatalf("Next() returned before the window closed: %#v", batch)
			default:
			}
			time.Sleep(time.Millisecond)
			synctest.Wait()
			if batch := <-done; batch.Text != "first\nsecond\n" {
				t.Fatalf("Next() = %#v, want one batch holding both writes", batch)
			}
		})
	})

	t.Run("Should return the exit with the output printed before it", func(t *testing.T) {
		t.Parallel()
		s := newOutputTestSession(1 << 10)
		cursor := s.OpenOutputCursor()
		appendOutputTest(s, "last words\n")
		code := 3
		s.mu.Lock()
		s.exit = &Exit{Cause: "exited", Code: &code}
		s.mu.Unlock()
		close(s.done)
		batch := cursor.Next(t.Context(), time.Hour)
		if batch.Text != "last words\n" || batch.Exit == nil || batch.Exit.Code == nil || *batch.Exit.Code != 3 {
			t.Fatalf("Next() = %#v, want the remaining output with exit code 3", batch)
		}
	})

	t.Run("Should report a session that closed without an exit record", func(t *testing.T) {
		t.Parallel()
		s := newOutputTestSession(1 << 10)
		cursor := s.OpenOutputCursor()
		close(s.done)
		if batch := cursor.Next(t.Context(), time.Hour); !batch.Closed || batch.Exit != nil {
			t.Fatalf("Next() = %#v, want Closed without an exit", batch)
		}
	})
}

func newOutputTestSession(capacity int) *session {
	return &session{ring: NewRing(capacity), done: make(chan struct{}), revisionReady: make(chan struct{})}
}

func appendOutputTest(s *session, text string) {
	s.ring.Append([]byte(text))
	s.mu.Lock()
	s.bumpRevisionLocked()
	s.mu.Unlock()
}
