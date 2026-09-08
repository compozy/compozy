package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

func TestIngestGate(t *testing.T) {
	t.Parallel()
	t.Run("Should finish the producer while the consumer remains gated", func(t *testing.T) {
		t.Parallel()
		gate := NewIngestGate(4, 1<<20)
		produced := make(chan error, 1)
		go func() {
			for range 10000 {
				if err := gate.Admit(AgentEvent{Type: EventTypeAgentMessage, TurnID: "turn", Text: "x"}); err != nil {
					produced <- err
					return
				}
			}
			produced <- nil
		}()
		// No Next call is possible before the producer reaches this checkpoint.
		if err := <-produced; err != nil {
			t.Fatal(err)
		}
		if stats := gate.Stats(); stats.Depth != 3 || stats.Coalesced != 9997 {
			t.Fatalf("gated stats = %#v", stats)
		}
		gate.Close(nil)
		var text strings.Builder
		for range 3 {
			event, err := gate.Next(t.Context())
			if err != nil || len(event.Text) > 4096 {
				t.Fatalf("bounded text event = %#v, %v", event, err)
			}
			text.WriteString(event.Text)
		}
		if text.String() != strings.Repeat("x", 10000) {
			t.Fatalf("lossless text bytes = %d", text.Len())
		}
		if _, err := gate.Next(t.Context()); !errors.Is(err, io.EOF) {
			t.Fatalf("drained gate = %v, want EOF", err)
		}
	})
	t.Run("Should preserve text metadata and semantic boundaries", func(t *testing.T) {
		t.Parallel()
		gate := NewIngestGate(16, 1<<20)
		text := func(value string) AgentEvent {
			return AgentEvent{
				Type:   EventTypeAgentMessage,
				TurnID: "turn-a",
				Text:   value,
				Raw: json.RawMessage(
					`{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"` + value + `"},"_meta":{"entry":"a"}}`,
				),
			}
		}
		for _, event := range []AgentEvent{
			text("one"), text("two"),
			{Type: EventTypeToolCall, TurnID: "turn-a", ToolCallID: "tool"},
			text("three"),
			{Type: EventTypeThought, TurnID: "turn-a", Text: "thought"},
			{Type: EventTypeAgentMessage, TurnID: "turn-b", Text: "other turn"},
			(AgentEvent{Type: EventTypeAgentMessage, TurnID: "turn-b", Text: "other entry"}).WithMessageID("new"),
		} {
			if err := gate.Admit(event); err != nil {
				t.Fatal(err)
			}
		}
		gate.Close(nil)
		for index, want := range []string{"onetwo", "", "three", "thought", "other turn", "other entry"} {
			event, err := gate.Next(t.Context())
			if err != nil || event.Text != want {
				t.Fatalf("event %d = %#v, %v; want %q", index, event, err, want)
			}
			if index == 0 && (!strings.Contains(string(event.Raw), `"text":"onetwo"`) ||
				!strings.Contains(string(event.Raw), `"entry":"a"`)) {
				t.Fatalf("coalesced raw payload = %s", event.Raw)
			}
			if index == 1 && event.Type != EventTypeToolCall {
				t.Fatalf("tool boundary = %#v", event)
			}
		}
	})
	t.Run("Should refuse saturation and drain every accepted semantic event before the failure", func(t *testing.T) {
		t.Parallel()
		gate := NewIngestGate(2, 4096)
		for _, id := range []string{"one", "two"} {
			if err := gate.Admit(AgentEvent{Type: EventTypeToolCall, ToolCallID: id}); err != nil {
				t.Fatal(err)
			}
		}
		if err := gate.Admit(AgentEvent{Type: EventTypePermission}); !errors.Is(err, ErrIngestSaturated) {
			t.Fatalf("full gate = %v", err)
		}
		gate.Close(ErrIngestSaturated)
		gate.Close(context.Canceled)
		for _, want := range []string{"one", "two"} {
			event, err := gate.Next(t.Context())
			if err != nil || event.ToolCallID != want {
				t.Fatalf("accepted semantic event = %#v, %v", event, err)
			}
		}
		if _, err := gate.Next(t.Context()); !errors.Is(err, ErrIngestSaturated) {
			t.Fatalf("terminal ingestion error = %v", err)
		}
		if err := gate.Admit(AgentEvent{}); !errors.Is(err, ErrIngestClosed) {
			t.Fatalf("admission after close = %v", err)
		}
	})
	t.Run("Should wake every waiting consumer with the closing error", func(t *testing.T) {
		t.Parallel()
		gate := NewIngestGate(2, 4096)
		results := make(chan error, 4)
		for range 4 {
			go func() { _, err := gate.Next(context.Background()); results <- err }()
		}
		gate.Close(ErrIngestSaturated)
		for range 4 {
			if err := <-results; !errors.Is(err, ErrIngestSaturated) {
				t.Fatalf("closed waiter = %v", err)
			}
		}
	})

	t.Run("Should serialize concurrent close and admission without losing accepted bytes", func(t *testing.T) {
		t.Parallel()
		gate := NewIngestGate(4, 1<<20)
		var accepted atomic.Int64
		var workers sync.WaitGroup
		for range 4 {
			workers.Go(func() {
				for range 1000 {
					err := gate.Admit(AgentEvent{Type: EventTypeAgentMessage, Text: "x"})
					if errors.Is(err, ErrIngestClosed) {
						return
					}
					if err != nil {
						t.Errorf("concurrent admission: %v", err)
						return
					}
					accepted.Add(1)
				}
			})
		}
		gate.Close(nil)
		workers.Wait()
		var text strings.Builder
		for {
			event, err := gate.Next(t.Context())
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			text.WriteString(event.Text)
		}
		if int64(text.Len()) != accepted.Load() {
			t.Fatalf("delivered bytes = %d, accepted = %d", text.Len(), accepted.Load())
		}
	})
}

func TestPromptIngestSaturation(t *testing.T) {
	t.Parallel()
	t.Run("Should emit transport failure after all accepted events and cancel the RPC", func(t *testing.T) {
		t.Parallel()
		canceled := make(chan struct{})
		active := &activePromptState{
			turnID: "turn-saturated", ingest: NewIngestGate(2, 4096), events: make(chan AgentEvent),
			cancel: func() { close(canceled) },
		}
		for _, id := range []string{"first", "second", "refused"} {
			active.sendEventLocked(AgentEvent{Type: EventTypeToolCall, ToolCallID: id})
		}
		<-canceled
		proc := &AgentProcess{SessionID: "session"}
		go proc.forwardIngestEvents(active)
		for _, id := range []string{"first", "second"} {
			if event := <-active.events; event.ToolCallID != id {
				t.Fatalf("accepted event = %#v", event)
			}
		}
		failure := <-active.events
		if failure.Type != EventTypeError || failure.Failure == nil || failure.Failure.Kind != store.FailureTransport ||
			failure.TurnID != active.turnID || !strings.Contains(failure.Error, ErrIngestSaturated.Error()) {
			t.Fatalf("saturation terminal = %#v", failure)
		}
		if _, ok := <-active.events; ok {
			t.Fatal("saturated prompt remains open")
		}
	})
}

func TestLiveBroadcast(t *testing.T) {
	t.Parallel()
	// Invariant UT-114: a projection subscriber retains one latest durable watermark
	// across token bursts without shedding; owner: post-append delivery, existing suite.
	t.Run("Should coalesce projection wakes to the latest sequence", func(t *testing.T) {
		t.Parallel()
		broadcast := NewLiveBroadcast(2)
		wakes, cancel := broadcast.Subscribe(0, true)
		defer cancel()
		for seq := int64(1); seq <= 500; seq++ {
			if broadcast.Publish(uint64(seq), store.SessionEvent{Sequence: seq, Type: EventTypeAgentMessage}) {
				t.Fatal("projection wake shed")
			}
		}
		if stats := broadcast.Stats(); stats.Depth != 1 || stats.Shed != 0 {
			t.Fatalf("wake stats = %+v", stats)
		}
		if wake := <-wakes; wake.Sequence != 500 {
			t.Fatalf("wake = %+v", wake)
		}
		broadcast.Publish(501, store.SessionEvent{Sequence: 501, Type: "session_stopped"})
		if wake := <-wakes; wake.Sequence != 501 || wake.Type != "session_stopped" {
			t.Fatalf("terminal wake = %+v", wake)
		}
	})

	t.Run("Should shed only the slow watcher and expose a durable replay cursor", func(t *testing.T) {
		t.Parallel()
		broadcast := NewLiveBroadcast(2)
		slow, cancelSlow := broadcast.Subscribe(0, false)
		defer cancelSlow()
		fast, cancelFast := broadcast.Subscribe(0, false)
		defer cancelFast()
		var durable []store.SessionEvent
		for seq := int64(1); seq <= 3; seq++ {
			event := store.SessionEvent{SessionID: "session", Sequence: seq, Type: EventTypeAgentMessage}
			durable = append(durable, event) // Only already-appended events may publish.
			broadcast.Publish(uint64(seq), event)
			if got := <-fast; got.Sequence != seq {
				t.Fatalf("healthy watcher sequence = %d, want %d", got.Sequence, seq)
			}
		}
		for _, seq := range []int64{1, 2} {
			if event := <-slow; event.Sequence != seq {
				t.Fatalf("buffered sequence = %d, want %d", event.Sequence, seq)
			}
		}
		marker := <-slow
		var cursor struct {
			After   uint64 `json:"after_sequence"`
			Through uint64 `json:"through_sequence"`
			Refresh bool   `json:"refresh"`
		}
		if err := json.Unmarshal([]byte(marker.Content), &cursor); err != nil {
			t.Fatal(err)
		}
		if marker.Type != events.StreamConsumerDegraded || marker.Sequence != 0 ||
			cursor.After != 2 || cursor.Through != 3 || !cursor.Refresh {
			t.Fatalf("degrade marker = %#v, cursor = %#v", marker, cursor)
		}
		if _, ok := <-slow; ok {
			t.Fatal("shed watcher did not close after one marker")
		}
		if replay := durable[cursor.After:cursor.Through]; len(replay) != 1 || replay[0].Sequence != 3 {
			t.Fatalf("reconstructed shed range = %#v", replay)
		}
		resumed, cancelResumed := broadcast.Subscribe(cursor.Through, false)
		defer cancelResumed()
		broadcast.Publish(4, store.SessionEvent{Sequence: 4})
		if event := <-resumed; event.Sequence != 4 {
			t.Fatalf("resumed live sequence = %d", event.Sequence)
		}
		if stats := broadcast.Stats(); stats.Shed != 1 || stats.Subscribers != 2 || stats.Depth != 1 {
			t.Fatalf("broadcast stats = %#v", stats)
		}
	})
}
