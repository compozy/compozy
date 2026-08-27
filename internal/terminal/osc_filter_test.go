package terminal

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

func TestOSCSecurityFilterShouldAuthenticateMarkersAndSplitDisplay(t *testing.T) {
	t.Parallel()
	filter := newOSCSecurityFilter("nonce-1", nil)
	first := filter.Filter([]byte("before\x1b]7113;v1;nonce-1;S;cmd=bun%20test;"))
	if string(first.DisplayBytes) != "before" || len(first.MarkerFacts) != 0 {
		t.Fatalf("first Filter() = %#v", first)
	}
	second := filter.Filter([]byte("cwd=%2Ftmp\x1b\\after"))
	if string(second.DisplayBytes) != "after" {
		t.Fatalf("DisplayBytes = %q, want after", second.DisplayBytes)
	}
	if len(second.MarkerFacts) != 1 || second.MarkerFacts[0].Kind != "S" ||
		second.MarkerFacts[0].Command != "bun test" || second.MarkerFacts[0].Cwd != "/tmp" {
		t.Fatalf("MarkerFacts = %#v", second.MarkerFacts)
	}

	for _, forged := range []string{
		"\x1b]133;A\x07", "\x1b]7113;v1;wrong;F;exit=0\x1b\\", "\x1b]7113;v1;;F;exit=0\x1b\\",
	} {
		result := filter.Filter([]byte(forged))
		if len(result.MarkerFacts) != 0 {
			t.Fatalf("forged marker %q emitted facts %#v", forged, result.MarkerFacts)
		}
	}

	t.Run("Should bound and fail closed on an unterminated OSC sequence", func(t *testing.T) {
		filter := newOSCSecurityFilter("nonce-1", nil)
		result := filter.Filter(append([]byte("visible\x1b]52;c;"), bytes.Repeat([]byte("x"), maxPendingOSCBytes)...))
		if string(result.DisplayBytes) != "visible" || len(filter.output.pending) != 0 || !filter.output.discarding {
			t.Fatalf(
				"overflow filter result/state = %#v pending=%d discarding=%v",
				result,
				len(filter.output.pending),
				filter.output.discarding,
			)
		}
		result = filter.Filter([]byte("ignored\x1b\\safe"))
		if string(result.DisplayBytes) != "safe" || filter.output.discarding {
			t.Fatalf("post-terminator filter result/state = %#v discarding=%v", result, filter.output.discarding)
		}
	})

	t.Run("Should neutralize OSC location hyperlinks and DCS for every output consumer", func(t *testing.T) {
		t.Parallel()
		input := []byte(
			"before\x1b]7;file:///tmp\x07middle\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\after\x1bPprivate\x1b\\done",
		)
		filtered := modelFacingOutput(input)
		if got, want := string(filtered), "beforemiddlelinkafterdone"; got != want {
			t.Fatalf("modelFacingOutput() = %q, want %q", got, want)
		}
		if got, want := string(newOSCSecurityFilter("nonce-1", nil).Filter(input).DisplayBytes),
			"beforemiddlelinkafterdone"; got != want {
			t.Fatalf("human display output = %q, want %q", got, want)
		}
	})
}

func TestOSCSecurityFilterShouldDeliverTypedFactsBeforeDisplayFanout(t *testing.T) {
	t.Run("Should deliver typed facts before filtered display bytes", func(t *testing.T) {
		t.Parallel()
		consumer := &recordingMarkerConsumer{facts: make(chan MarkerFacts, 1)}
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithMarkerConsumer(consumer))
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		marker := "visible\x1b]7113;v1;" + handle.MarkerNonce() + ";S;cmd=bun%20test;cwd=%2Ftmp\x1b\\after"
		if err := starter.latest().emit([]byte(marker)); err != nil {
			t.Fatalf("emit marker output: %v", err)
		}
		select {
		case fact := <-consumer.facts:
			if fact.Kind != "S" || fact.Command != "bun test" || fact.Cwd != "/tmp" {
				t.Fatalf("consumed marker fact = %#v", fact)
			}
		case <-time.After(time.Second):
			t.Fatal("authenticated marker fact did not reach the consumer")
		}
		waitForTerminalTail(
			t,
			handle,
			"display fanout did not receive only filtered bytes",
			func(read *ReadResult) bool {
				return read.Content == "visibleafter"
			},
		)
	})
}

func TestOSCSecurityFilterShouldBlockInputWhenAuthenticatedFactsCannotBeJournaled(t *testing.T) {
	t.Run("Should block input after authenticated facts miss the journal lane", func(t *testing.T) {
		t.Parallel()
		consumer := &failingMarkerConsumer{called: make(chan struct{}, 1)}
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithMarkerConsumer(consumer))
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		marker := "\x1b]7113;v1;" + handle.MarkerNonce() + ";S;cmd=pwd;cwd=%2Ftmp\x1b\\"
		if err := starter.latest().emit([]byte(marker)); err != nil {
			t.Fatalf("emit marker output: %v", err)
		}
		deadline := time.NewTimer(time.Second)
		defer deadline.Stop()
		select {
		case <-consumer.called:
		case <-deadline.C:
			t.Fatal("marker consumer was not called")
		}
		actor := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		if err := handle.Write(t.Context(), actor, []byte("blocked")); !errors.Is(err, ErrJournalUnavailable) {
			t.Fatalf("Write(after marker journal failure) error = %v, want ErrJournalUnavailable", err)
		}
	})
}

type recordingMarkerConsumer struct {
	facts chan MarkerFacts
}

type failingMarkerConsumer struct {
	called chan struct{}
}

func (c *failingMarkerConsumer) ConsumeMarkerFacts(context.Context, Info, []MarkerFacts) error {
	c.called <- struct{}{}
	return ErrJournalUnavailable
}

func (c *recordingMarkerConsumer) ConsumeMarkerFacts(_ context.Context, _ Info, facts []MarkerFacts) error {
	for _, fact := range facts {
		c.facts <- fact
	}
	return nil
}

func TestOSCSecurityFilterShouldStripClipboardBeforeSequenceAccounting(t *testing.T) {
	t.Run("Should strip clipboard controls before sequence accounting", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		sub, err := handle.Attach(t.Context(), AttachOptions{
			Mode: "read", Flow: "drop", Actor: Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
		})
		if err != nil {
			t.Fatalf("Attach() error = %v", err)
		}
		t.Cleanup(func() {
			if err := sub.Close(); err != nil {
				t.Errorf("subscription.Close() error = %v", err)
			}
		})
		<-sub.Frames()
		assertPresenceFrame(t, receiveSubscriptionFrame(t, sub), 1)
		input := []byte("before\x1b]52;c;c2VjcmV0\x07after")
		if err := starter.latest().emit(input); err != nil {
			t.Fatalf("emit() error = %v", err)
		}
		select {
		case frame := <-sub.Frames():
			if frame.Op != terminalwire.ServerOpOutput || frame.Seq != 0 || string(frame.Payload) != "beforeafter" {
				t.Fatalf("OUTPUT = %#v", frame)
			}
		case <-time.After(time.Second):
			t.Fatal("filtered OUTPUT was not delivered")
		}
		read, err := handle.Screen(t.Context(), ReadOptions{View: "tail"})
		if err != nil {
			t.Fatalf("Screen() error = %v", err)
		}
		if read.Seq != uint64(len("beforeafter")) || read.Content != "beforeafter" ||
			bytes.Contains([]byte(read.Content), []byte("52;")) {
			t.Fatalf("Screen() = %#v", read)
		}
		actor := Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"}
		if err := handle.Write(t.Context(), actor, []byte("x\x1b]52;c;leak\x07y")); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if got := starter.latest().inputString(); got != "xy" {
			t.Fatalf("process input = %q, want xy", got)
		}
	})
}

func TestTerminalTitlePipelineShouldPinAndSanitize(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a user title pinned over program OSC", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle, err := manager.Open(context.Background(), OpenRequest{
			WS:           "workspace-a",
			Title:        "Pinned",
			Actor:        Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
			Capabilities: Capabilities{Interactive: true},
		})
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		if err := starter.latest().emit([]byte("\x1b]2;program\x07sync")); err != nil {
			t.Fatalf("emit() error = %v", err)
		}
		waitForTerminalTail(t, handle, "title output was not consumed", func(read *ReadResult) bool {
			return strings.Contains(read.Content, "sync")
		})
		if got := handle.Info().Title; got != "Pinned" {
			t.Fatalf("Info().Title = %q, want Pinned", got)
		}
	})

	t.Run("Should strip controls and bound an unpinned title", func(t *testing.T) {
		t.Parallel()
		bus := NewNotifier(nil)
		titleChanged := make(chan Event, 1)
		bus.Observe(func(_ context.Context, event Event) {
			if event.Kind == EventKindTitleChanged {
				titleChanged <- event
			}
		})
		manager, starter, _ := newTestManager(t, DefaultSettings(), WithNotifier(bus))
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		programTitle := "hello\n" + strings.Repeat("界", 200)
		if err := starter.latest().emit([]byte("\x1b]2;" + programTitle + "\x07")); err != nil {
			t.Fatalf("emit() error = %v", err)
		}
		deadline := time.NewTimer(time.Second)
		defer deadline.Stop()
		select {
		case <-titleChanged:
		case <-deadline.C:
			t.Fatal("title change event was not emitted")
		}
		title := handle.Info().Title
		if strings.ContainsAny(title, "\r\n") || len([]byte(title)) > 256 {
			t.Fatalf("sanitized title = %q (%d bytes)", title, len([]byte(title)))
		}
	})
}

func TestSessionFlowShouldPauseAndResumeThePTYReader(t *testing.T) {
	t.Run("Should pause above the high watermark and resume below the low watermark", func(t *testing.T) {
		t.Parallel()
		manager, starter, _ := newTestManager(t, DefaultSettings())
		handle := openTestTerminal(t, manager, "workspace-a", "profile-a")
		sub, err := handle.Attach(t.Context(), AttachOptions{
			Mode: "read", Flow: "ack", Actor: Actor{Kind: ActorKindHuman, ID: "operator", ProfileID: "profile-a"},
		})
		if err != nil {
			t.Fatalf("Attach() error = %v", err)
		}
		t.Cleanup(func() {
			if err := sub.Close(); err != nil {
				t.Errorf("subscription.Close() error = %v", err)
			}
		})
		<-sub.Frames()
		emitDone := make(chan error, 1)
		go func() { emitDone <- starter.latest().emit(make([]byte, terminalwire.AckHighWatermark+(64<<10))) }()
		waitForTerminalCondition(t, "queue did not reach the high watermark", func() bool {
			return sub.(*subscription).queue.PendingBytes() >= terminalwire.AckHighWatermark
		})
		readsBefore := starter.latest().reads.Load()
		stableWindow := time.NewTimer(20 * time.Millisecond)
		defer stableWindow.Stop()
		<-stableWindow.C
		readsAfter := starter.latest().reads.Load()
		if readsAfter != readsBefore {
			t.Fatalf("PTY reads advanced while high: before=%d after=%d", readsBefore, readsAfter)
		}
		for sub.(*subscription).queue.PendingBytes() > terminalwire.AckLowWatermark {
			select {
			case frame := <-sub.Frames():
				if frame.Op == terminalwire.ServerOpOutput {
					sub.Ack(len(frame.Payload))
				}
			case <-time.After(time.Second):
				t.Fatal("queued output was not delivered for ACK")
			}
		}
		select {
		case err := <-emitDone:
			if err != nil {
				t.Fatalf("producer after ACK error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("producer did not resume after ACK")
		}
		readsBefore = starter.latest().reads.Load()
		if err := starter.latest().emit(make([]byte, outputReadBytes)); err != nil {
			t.Fatalf("emit after resume error = %v", err)
		}
		waitForTerminalCondition(t, "PTY reads did not resume", func() bool {
			return starter.latest().reads.Load() > readsBefore
		})
		if reads := starter.latest().reads.Load(); reads <= readsBefore {
			t.Fatalf("PTY reads did not resume: before=%d after=%d", readsBefore, reads)
		}
	})
}

func waitForTerminalCondition(t *testing.T, failure string, ready func() bool) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for !ready() {
		select {
		case <-deadline.C:
			t.Fatal(failure)
		case <-ticker.C:
		}
	}
}
