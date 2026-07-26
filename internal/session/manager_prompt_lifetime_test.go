package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
)

func TestPromptCallerCancellationContract(t *testing.T) {
	t.Run("Should keep accepted prompt execution after delivery context cancellation", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		driver := &promptContextCapturingDriver{fakeDriver: h.driver}
		h.manager = newManagerWithHarness(t, h, WithDriver(driver))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		source := make(chan acp.AgentEvent, 2)
		var turnID string
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			turnID = req.TurnID
			return source, nil
		}

		deliveryCtx, cancelDelivery := context.WithCancel(testutil.Context(t))
		result, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
			Message:         "hello",
			DeliveryContext: deliveryCtx,
		})
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}
		if result.Events == nil {
			t.Fatal("SendPrompt().Events = nil, want accepted stream")
		}
		providerCtx := driver.lastPromptContext(t)
		waitForCondition(t, "session prompting", func() bool {
			return session.IsPrompting()
		})

		cancelDelivery()
		if !errors.Is(deliveryCtx.Err(), context.Canceled) {
			t.Fatalf("delivery context err = %v, want context.Canceled", deliveryCtx.Err())
		}
		select {
		case <-providerCtx.Done():
			t.Fatalf("provider context canceled with delivery context: %v", providerCtx.Err())
		default:
		}

		source <- acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: session.Info().ACPSessionID,
			TurnID:    turnID,
			Timestamp: time.Date(2026, 5, 17, 17, 0, 0, 0, time.UTC),
			Text:      "still running after detach",
		}
		source <- acp.AgentEvent{
			Type:      acp.EventTypeDone,
			SessionID: session.Info().ACPSessionID,
			TurnID:    turnID,
			Timestamp: time.Date(2026, 5, 17, 17, 0, 1, 0, time.UTC),
		}
		close(source)

		waitForCondition(t, "terminal persistence after delivery cancellation", func() bool {
			events, queryErr := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
			return queryErr == nil &&
				countEventType(events, acp.EventTypeUserMessage) == 1 &&
				countEventType(events, acp.EventTypeAgentMessage) == 1 &&
				countEventType(events, acp.EventTypeDone) == 1
		})
		waitForCondition(t, "prompt state cleared after provider completion", func() bool {
			return !session.IsPrompting()
		})
		if events := collectEvents(t, result.Events); len(events) != 0 {
			t.Fatalf("delivered events after delivery cancellation = %d, want 0", len(events))
		}
	})

	t.Run("Should keep accepted prompt execution after caller context cancellation", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		driver := &promptContextCapturingDriver{fakeDriver: h.driver}
		h.manager = newManagerWithHarness(t, h, WithDriver(driver))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		source := make(chan acp.AgentEvent, 1)
		var turnID string
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			turnID = req.TurnID
			return source, nil
		}

		callerCtx, cancelCaller := context.WithCancel(testutil.Context(t))
		eventsCh, err := h.manager.Prompt(callerCtx, session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		providerCtx := driver.lastPromptContext(t)
		waitForCondition(t, "session prompting", func() bool {
			return session.IsPrompting()
		})

		cancelCaller()
		select {
		case <-callerCtx.Done():
		default:
			t.Fatal("caller context is still active after cancel")
		}
		select {
		case <-providerCtx.Done():
			t.Fatalf("provider context canceled with caller context: %v", providerCtx.Err())
		default:
		}
		if !session.IsPrompting() {
			t.Fatal("session prompting = false after caller cancellation, want active prompt execution")
		}

		source <- acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: session.Info().ACPSessionID,
			TurnID:    turnID,
			Timestamp: time.Date(2026, 5, 17, 16, 0, 0, 0, time.UTC),
			Text:      "still running",
		}
		waitForCondition(t, "agent message persistence after caller cancellation", func() bool {
			events, queryErr := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
			return queryErr == nil && countEventType(events, acp.EventTypeAgentMessage) == 1
		})

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		select {
		case <-providerCtx.Done():
		default:
			t.Fatal("provider context is still active after CancelPrompt()")
		}
		close(source)
		if events := collectEvents(t, eventsCh); len(events) != 0 {
			t.Fatalf("delivered events after caller cancellation = %d, want 0", len(events))
		}
		waitForCondition(t, "prompt state cleared after explicit cancellation", func() bool {
			return !session.IsPrompting()
		})
	})
}

func TestPromptTranscriptMarkerClassifiesStructuredMCPAuthReason(t *testing.T) {
	t.Parallel()

	t.Run("Should classify MCP auth from structured request error data", func(t *testing.T) {
		t.Parallel()

		kind, _, _, ok := promptTranscriptMarker(acp.AgentEvent{
			Type:  acp.EventTypeError,
			Error: "provider authentication failed",
			Failure: &store.SessionFailure{
				Kind: store.FailureProviderAuth,
			},
			Raw: []byte(`{"data":{"reason_codes":["mcp_auth_required"]}}`),
		})
		if !ok {
			t.Fatal("promptTranscriptMarker() ok = false, want true")
		}
		if kind != transcript.MarkerMCPAuthRequired {
			t.Fatalf("promptTranscriptMarker() kind = %q, want %q", kind, transcript.MarkerMCPAuthRequired)
		}
	})

	t.Run("Should stop MCP auth reason scanning at bounded JSON depth", func(t *testing.T) {
		t.Parallel()

		raw := strings.Repeat(`{"child":`, maxMCPAuthReasonJSONDepth+2) +
			`{"reason":"mcp_auth_required"}` +
			strings.Repeat(`}`, maxMCPAuthReasonJSONDepth+2)
		if eventHasMCPAuthReason(acp.AgentEvent{Raw: []byte(raw)}) {
			t.Fatal("eventHasMCPAuthReason() = true for reason beyond bounded scan depth")
		}
	})
}

func TestPromptFileMutationVerifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		recovered  bool
		terminal   string
		wantMarker int
	}{
		{name: "Should persist one verifier marker for an unrecovered failed write", wantMarker: 1},
		{name: "Should suppress the verifier marker after a later successful write", recovered: true},
		{
			name:       "Should persist one verifier marker when the failed-write turn ends with an error",
			terminal:   acp.EventTypeError,
			wantMarker: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			sess := createSession(t, h)
			t.Cleanup(func() {
				if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil &&
					!errors.Is(err, ErrSessionNotFound) {
					t.Errorf("Stop() error = %v", err)
				}
			})
			terminal := tc.terminal
			if terminal == "" {
				terminal = acp.EventTypeDone
			}
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent, 8)
				now := time.Now().UTC()
				events <- acp.AgentEvent{
					Type: acp.EventTypeAgentMessage, TurnID: req.TurnID, Timestamp: now,
					Text: "I attempted the requested file update.",
				}
				events <- fileMutationToolKindUpdate(req.TurnID, "write-failed", now)
				events <- fileMutationToolInputUpdate(t, req.TurnID, "write-failed", "src/retry.go", now)
				events <- acp.AgentEvent{
					Type: acp.EventTypeToolResult, TurnID: req.TurnID, ToolCallID: "write-failed", Timestamp: now,
				}.WithToolDetail("Write", nil, true, "permission denied")
				if tc.recovered {
					events <- fileMutationToolKindUpdate(req.TurnID, "write-recovered", now)
					events <- fileMutationToolInputUpdate(t, req.TurnID, "write-recovered", "src/retry.go", now)
					events <- acp.AgentEvent{
						Type: acp.EventTypeToolResult, TurnID: req.TurnID,
						ToolCallID: "write-recovered", Timestamp: now,
					}.WithTool("Write", nil, false)
				}
				terminalEvent := acp.AgentEvent{Type: terminal, TurnID: req.TurnID, Timestamp: now}
				if terminal == acp.EventTypeError {
					terminalEvent.Error = "provider turn failed"
				}
				events <- terminalEvent
				close(events)
				return events, nil
			}

			events, err := h.manager.Prompt(testutil.Context(t), sess.ID, "Update the retry implementation")
			if err != nil {
				t.Fatalf("Prompt() error = %v", err)
			}
			collectEvents(t, events)
			stored, err := h.manager.Events(testutil.Context(t), sess.ID, store.EventQuery{})
			if err != nil {
				t.Fatalf("Events() error = %v", err)
			}
			markers := 0
			persistedEditKind := false
			for _, storedEvent := range stored {
				event, unmarshalErr := transcript.UnmarshalAgentEvent(storedEvent.Content)
				if unmarshalErr != nil {
					t.Fatalf("UnmarshalAgentEvent() error = %v", unmarshalErr)
				}
				if event.Type == acp.EventTypeToolCall && event.ToolKind() == "edit" {
					persistedEditKind = true
				}
				marker, ok := transcript.ParseMarker(event.Raw)
				if ok && marker.Kind == transcript.MarkerFileMutationUnverified {
					markers++
					if got := marker.Evidence["failure_count"]; got != float64(1) && got != 1 {
						t.Fatalf("marker failure_count = %#v, want 1", got)
					}
				}
			}
			if markers != tc.wantMarker {
				t.Fatalf("verifier markers = %d, want %d", markers, tc.wantMarker)
			}
			if !persistedEditKind {
				t.Fatal("persisted edit tool kind = false, want canonical edit evidence")
			}
			if tc.wantMarker > 0 {
				assertFileMutationMarkerNotifiedBeforeTerminal(t, h.notifier.eventsForSession(sess.ID), terminal)
			}
		})
	}

	t.Run("Should bound marker paths while retaining the total unresolved count", func(t *testing.T) {
		t.Parallel()

		verifier := fileMutationVerifier{}
		verifier.Observe(acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "Mutation summary"})
		const failures = maxFileMutationEvidencePaths + 3
		for index := range failures {
			callID := fmt.Sprintf("write-%d", index)
			verifier.Observe(fileMutationToolCall(
				t,
				"turn-bounded",
				callID,
				fmt.Sprintf("src/file-%02d.go", index),
				time.Now().UTC(),
			))
			verifier.Observe(acp.AgentEvent{
				Type: acp.EventTypeToolResult, ToolCallID: callID,
			}.WithToolDetail("Write", nil, true, "permission denied"))
		}

		_, evidence, ok := verifier.Marker()
		if !ok {
			t.Fatal("Marker() ok = false, want bounded marker")
		}
		if got := evidence["failure_count"]; got != failures {
			t.Fatalf("Marker() failure_count = %#v, want %d", got, failures)
		}
		paths, ok := evidence["paths"].([]string)
		if !ok || len(paths) != maxFileMutationEvidencePaths {
			t.Fatalf("Marker() paths = %#v, want %d entries", evidence["paths"], maxFileMutationEvidencePaths)
		}
		if truncated, ok := evidence["paths_truncated"].(bool); !ok || !truncated {
			t.Fatalf("Marker() paths_truncated = %#v, want true", evidence["paths_truncated"])
		}
	})
}

func assertFileMutationMarkerNotifiedBeforeTerminal(
	t *testing.T,
	events []acp.AgentEvent,
	terminal string,
) {
	t.Helper()
	markerIndex := -1
	terminalIndex := -1
	for index, event := range events {
		if marker, ok := transcript.ParseMarker(event.Raw); ok &&
			marker.Kind == transcript.MarkerFileMutationUnverified {
			markerIndex = index
		}
		if event.Type == terminal {
			terminalIndex = index
		}
	}
	if markerIndex < 0 || terminalIndex < 0 || markerIndex >= terminalIndex {
		t.Fatalf(
			"notifier marker index = %d, terminal index = %d, want marker before %q",
			markerIndex,
			terminalIndex,
			terminal,
		)
	}
}

func fileMutationToolCall(t *testing.T, turnID string, callID string, path string, at time.Time) acp.AgentEvent {
	t.Helper()

	input, err := json.Marshal(map[string]string{"file_path": path})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return acp.AgentEvent{
		Type: acp.EventTypeToolCall, TurnID: turnID, ToolCallID: callID, Timestamp: at,
	}.WithTool("Write", input, false).WithToolKind("edit")
}

func fileMutationToolKindUpdate(turnID string, callID string, at time.Time) acp.AgentEvent {
	return acp.AgentEvent{
		Type: acp.EventTypeToolCall, TurnID: turnID, ToolCallID: callID, Timestamp: at,
	}.WithTool("Write", nil, false).WithToolKind("edit")
}

func fileMutationToolInputUpdate(
	t *testing.T,
	turnID string,
	callID string,
	path string,
	at time.Time,
) acp.AgentEvent {
	t.Helper()

	event := fileMutationToolCall(t, turnID, callID, path, at)
	return event.WithToolKind("")
}

type promptContextCapturingDriver struct {
	*fakeDriver
	mu       sync.Mutex
	contexts []context.Context
}

func (d *promptContextCapturingDriver) Prompt(
	ctx context.Context,
	proc *AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	d.contexts = append(d.contexts, ctx)
	d.mu.Unlock()
	return d.fakeDriver.Prompt(ctx, proc, req)
}

func (d *promptContextCapturingDriver) lastPromptContext(t *testing.T) context.Context {
	t.Helper()

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.contexts) == 0 {
		t.Fatal("driver prompt contexts = 0, want at least 1")
	}
	return d.contexts[len(d.contexts)-1]
}
