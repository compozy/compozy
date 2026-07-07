package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
)

func TestSessionPublishBehavior(t *testing.T) {
	t.Run("fast path publishes immediately without counters", func(t *testing.T) {
		session := newTestSessionWithBuffer("sess-fast", 1)
		update := model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk}

		session.publish(context.Background(), update)

		got := mustReceiveSessionUpdate(t, session.updates)
		if got.Kind != update.Kind {
			t.Fatalf("unexpected update kind: got %q want %q", got.Kind, update.Kind)
		}
		if got.Status != model.StatusRunning {
			t.Fatalf("unexpected update status: got %q want %q", got.Status, model.StatusRunning)
		}
		if session.SlowPublishes() != 0 {
			t.Fatalf("unexpected slow publish count: %d", session.SlowPublishes())
		}
		if session.DroppedUpdates() != 0 {
			t.Fatalf("unexpected dropped update count: %d", session.DroppedUpdates())
		}
	})

	t.Run("backpressure success increments slow publish counter", func(t *testing.T) {
		setSessionPublishBackpressureTimeout(t, 5*time.Second)

		session := newTestSessionWithBuffer("sess-slow", 1)
		session.updates <- model.SessionUpdate{Kind: model.UpdateKindPlanUpdated}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		update := model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk}
		go func() {
			session.publish(ctx, update)
			close(done)
		}()

		waitForActivePublish(t, session)
		select {
		case <-done:
			t.Fatal("expected publish to wait while updates buffer is full")
		default:
		}

		buffered := mustReceiveSessionUpdate(t, session.updates)
		if buffered.Kind != model.UpdateKindPlanUpdated {
			t.Fatalf("unexpected buffered update kind before backpressure release: got %q", buffered.Kind)
		}

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("publish did not complete after backpressure was released")
		}

		got := mustReceiveSessionUpdate(t, session.updates)
		if got.Kind != update.Kind {
			t.Fatalf("unexpected update kind after backpressure: got %q want %q", got.Kind, update.Kind)
		}
		if session.SlowPublishes() != 1 {
			t.Fatalf("unexpected slow publish count: got %d want 1", session.SlowPublishes())
		}
		if session.DroppedUpdates() != 0 {
			t.Fatalf("unexpected dropped update count: %d", session.DroppedUpdates())
		}
	})

	t.Run("timeout path drops update and emits warn log", func(t *testing.T) {
		setSessionPublishBackpressureTimeout(t, 30*time.Millisecond)

		logBuf := captureDefaultLogger(t)
		session := newTestSessionWithBuffer("sess-timeout", 1)
		session.updates <- model.SessionUpdate{Kind: model.UpdateKindPlanUpdated}

		start := time.Now()
		update := model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk}
		session.publish(context.Background(), update)
		if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
			t.Fatalf("expected publish to block until timeout, returned after %v", elapsed)
		}

		if session.SlowPublishes() != 0 {
			t.Fatalf("unexpected slow publish count: %d", session.SlowPublishes())
		}
		if session.DroppedUpdates() != 1 {
			t.Fatalf("unexpected dropped update count: got %d want 1", session.DroppedUpdates())
		}

		got := mustReceiveSessionUpdate(t, session.updates)
		if got.Kind != model.UpdateKindPlanUpdated {
			t.Fatalf("unexpected buffered update kind after timeout: %q", got.Kind)
		}
		select {
		case extra := <-session.updates:
			t.Fatalf("expected dropped update to stay out of channel, got %#v", extra)
		default:
		}

		records := decodeLogRecords(t, logBuf)
		if len(records) != 1 {
			t.Fatalf("expected exactly one drop warning, got %d", len(records))
		}
		record := records[0]
		if gotMsg := record["msg"]; gotMsg != "acp session update dropped after backpressure timeout" {
			t.Fatalf("unexpected log message: %v", gotMsg)
		}
		if gotSessionID := record["session_id"]; gotSessionID != "sess-timeout" {
			t.Fatalf("unexpected log session_id: %v", gotSessionID)
		}
		if gotKind := record["kind"]; gotKind != string(model.UpdateKindAgentMessageChunk) {
			t.Fatalf("unexpected log kind: %v", gotKind)
		}
		if gotBufferCap := int(record["buffer_cap"].(float64)); gotBufferCap != 1 {
			t.Fatalf("unexpected log buffer_cap: %d", gotBufferCap)
		}
		if gotDroppedTotal := int(record["dropped_total"].(float64)); gotDroppedTotal != 1 {
			t.Fatalf("unexpected log dropped_total: %d", gotDroppedTotal)
		}
	})

	t.Run("context cancellation exits without counters", func(t *testing.T) {
		setSessionPublishBackpressureTimeout(t, time.Second)

		session := newTestSessionWithBuffer("sess-cancel", 1)
		session.updates <- model.SessionUpdate{Kind: model.UpdateKindPlanUpdated}

		ctx, cancel := context.WithCancel(context.Background())
		timer := time.NewTimer(20 * time.Millisecond)
		defer timer.Stop()
		go func() {
			<-timer.C
			cancel()
		}()

		start := time.Now()
		session.publish(ctx, model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk})
		if elapsed := time.Since(start); elapsed >= 200*time.Millisecond {
			t.Fatalf("expected publish to stop on cancellation, returned after %v", elapsed)
		}

		if session.SlowPublishes() != 0 {
			t.Fatalf("unexpected slow publish count: %d", session.SlowPublishes())
		}
		if session.DroppedUpdates() != 0 {
			t.Fatalf("unexpected dropped update count: %d", session.DroppedUpdates())
		}
		if gotLen := len(session.updates); gotLen != 1 {
			t.Fatalf("expected cancellation to leave buffer untouched, got len=%d", gotLen)
		}
	})

	t.Run("drop warnings are rate limited per session", func(t *testing.T) {
		setSessionPublishBackpressureTimeout(t, 0)

		logBuf := captureDefaultLogger(t)
		session := newTestSessionWithBuffer("sess-rate-limit", 1)
		session.updates <- model.SessionUpdate{Kind: model.UpdateKindPlanUpdated}

		for i := 0; i < 100; i++ {
			session.publish(context.Background(), model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk})
		}

		if session.DroppedUpdates() != 100 {
			t.Fatalf("unexpected dropped update count: got %d want 100", session.DroppedUpdates())
		}
		records := decodeLogRecords(t, logBuf)
		if len(records) > 1 {
			t.Fatalf("expected at most one rate-limited warning, got %d", len(records))
		}
		if len(records) != 1 {
			t.Fatalf("expected one warning for the first dropped update, got %d", len(records))
		}
	})

	t.Run("critical completed update is delivered under saturated buffer", func(t *testing.T) {
		// A backpressure window that would immediately drop a non-critical update.
		setSessionPublishBackpressureTimeout(t, time.Millisecond)

		session := newTestSessionWithBuffer("sess-critical", 1)
		session.updates <- model.SessionUpdate{Kind: model.UpdateKindPlanUpdated}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan struct{})
		critical := model.SessionUpdate{
			Kind:          model.UpdateKindToolCallUpdated,
			ToolCallState: model.ToolCallStateCompleted,
		}
		go func() {
			session.publish(ctx, critical)
			close(done)
		}()

		waitForActivePublish(t, session)
		// Prove the critical publish blocks past the backpressure window instead
		// of dropping the completion.
		select {
		case <-done:
			t.Fatal("expected critical publish to block until the drain accepts it")
		case <-time.After(20 * time.Millisecond):
		}

		buffered := mustReceiveSessionUpdate(t, session.updates)
		if buffered.Kind != model.UpdateKindPlanUpdated {
			t.Fatalf("unexpected saturating update kind: %q", buffered.Kind)
		}

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("critical publish did not complete after the drain freed the buffer")
		}

		got := mustReceiveSessionUpdate(t, session.updates)
		if got.Kind != model.UpdateKindToolCallUpdated || got.ToolCallState != model.ToolCallStateCompleted {
			t.Fatalf("unexpected delivered critical update: kind=%q state=%q", got.Kind, got.ToolCallState)
		}
		if session.DroppedUpdates() != 0 {
			t.Fatalf("critical update must never drop, got dropped=%d", session.DroppedUpdates())
		}
		if session.SlowPublishes() != 1 {
			t.Fatalf("expected one slow publish for the blocked critical update, got %d", session.SlowPublishes())
		}
	})

	t.Run("non-critical updates drop under sustained backpressure but critical never drops", func(t *testing.T) {
		setSessionPublishBackpressureTimeout(t, 0)

		session := newTestSessionWithBuffer("sess-mixed", 1)
		session.updates <- model.SessionUpdate{Kind: model.UpdateKindPlanUpdated}

		const chatter = 50
		for i := 0; i < chatter; i++ {
			session.publish(context.Background(), model.SessionUpdate{Kind: model.UpdateKindAgentThoughtChunk})
		}
		if got := session.DroppedUpdates(); got != chatter {
			t.Fatalf("expected %d non-critical drops, got %d", chatter, got)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() {
			session.publish(ctx, model.SessionUpdate{Kind: model.UpdateKindToolCallStarted})
			close(done)
		}()
		waitForActivePublish(t, session)

		buffered := mustReceiveSessionUpdate(t, session.updates)
		if buffered.Kind != model.UpdateKindPlanUpdated {
			t.Fatalf("unexpected saturating update kind: %q", buffered.Kind)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("critical publish did not complete after the buffer was freed")
		}

		got := mustReceiveSessionUpdate(t, session.updates)
		if got.Kind != model.UpdateKindToolCallStarted {
			t.Fatalf("unexpected critical update delivered: %q", got.Kind)
		}
		if session.DroppedUpdates() != chatter {
			t.Fatalf("critical update must not increment drops, got %d", session.DroppedUpdates())
		}
	})

	t.Run("accessors expose atomic counters", func(t *testing.T) {
		session := newTestSessionWithBuffer("sess-metrics", 1)
		session.slowPublishes.Store(7)
		session.droppedUpdates.Store(11)

		if got := session.SlowPublishes(); got != 7 {
			t.Fatalf("unexpected slow publish accessor: got %d want 7", got)
		}
		if got := session.DroppedUpdates(); got != 11 {
			t.Fatalf("unexpected dropped update accessor: got %d want 11", got)
		}
	})

	t.Run("finished session ignores publish", func(t *testing.T) {
		session := newTestSessionWithBuffer("sess-finished", 1)
		session.mu.Lock()
		session.finished = true
		session.mu.Unlock()

		session.publish(context.Background(), model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk})

		if gotLen := len(session.updates); gotLen != 0 {
			t.Fatalf("expected finished session to ignore publish, got len=%d", gotLen)
		}
		if session.SlowPublishes() != 0 || session.DroppedUpdates() != 0 {
			t.Fatalf(
				"expected counters to stay zero, got slow=%d dropped=%d",
				session.SlowPublishes(),
				session.DroppedUpdates(),
			)
		}
	})

	t.Run("suppressed session tracks update without publishing it", func(t *testing.T) {
		session := newTestSessionWithBuffer("sess-suppressed", 1)
		session.suppressUpdates = true

		session.publish(context.Background(), model.SessionUpdate{
			Kind:          model.UpdateKindToolCallUpdated,
			ToolCallState: model.ToolCallStateFailed,
		})

		if gotLen := len(session.updates); gotLen != 0 {
			t.Fatalf("expected suppressed session to hide updates, got len=%d", gotLen)
		}
		if session.updatesSeen != 1 {
			t.Fatalf("expected suppressed session to track seen updates, got %d", session.updatesSeen)
		}
		if !session.lastUpdateFailedToolCall() {
			t.Fatal("expected suppressed session to retain failed tool-call state")
		}
		if session.SlowPublishes() != 0 || session.DroppedUpdates() != 0 {
			t.Fatalf(
				"expected counters to stay zero, got slow=%d dropped=%d",
				session.SlowPublishes(),
				session.DroppedUpdates(),
			)
		}
	})
}

func TestIsCriticalSessionUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		update model.SessionUpdate
		want   bool
	}{
		{
			name:   "tool call started is critical",
			update: model.SessionUpdate{Kind: model.UpdateKindToolCallStarted},
			want:   true,
		},
		{
			name:   "tool call updated is critical",
			update: model.SessionUpdate{Kind: model.UpdateKindToolCallUpdated},
			want:   true,
		},
		{
			name: "tool call completed transition is critical",
			update: model.SessionUpdate{
				Kind:          model.UpdateKindToolCallUpdated,
				ToolCallState: model.ToolCallStateCompleted,
			},
			want: true,
		},
		{
			name: "terminal completed status is critical",
			update: model.SessionUpdate{
				Kind:   model.UpdateKindAgentMessageChunk,
				Status: model.StatusCompleted,
			},
			want: true,
		},
		{
			name:   "terminal failed status is critical",
			update: model.SessionUpdate{Status: model.StatusFailed},
			want:   true,
		},
		{
			name:   "agent message chunk is non-critical",
			update: model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk},
			want:   false,
		},
		{
			name:   "agent thought chunk is non-critical",
			update: model.SessionUpdate{Kind: model.UpdateKindAgentThoughtChunk},
			want:   false,
		},
		{
			name:   "plan update is non-critical",
			update: model.SessionUpdate{Kind: model.UpdateKindPlanUpdated},
			want:   false,
		},
		{
			name: "running message chunk is non-critical",
			update: model.SessionUpdate{
				Kind:   model.UpdateKindAgentMessageChunk,
				Status: model.StatusRunning,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isCriticalSessionUpdate(tt.update); got != tt.want {
				t.Fatalf("isCriticalSessionUpdate(%+v) = %v, want %v", tt.update, got, tt.want)
			}
		})
	}
}

func TestClientSessionBufferHandlesThousandUpdatesWithoutDrops(t *testing.T) {
	t.Parallel()

	updates := make([]acp.SessionUpdate, 0, 1000)
	for i := 0; i < 1000; i++ {
		updates = append(updates, acp.UpdateAgentMessageText(fmt.Sprintf("chunk-%04d", i)))
	}

	scenario := helperScenario{
		ExpectedCWD:          t.TempDir(),
		ExpectedPrompt:       "stream many updates",
		UpdateIntervalMillis: 10,
		Updates:              updates,
		StopReason:           string(acp.StopReasonEndTurn),
	}

	client := newTestClient(t, scenario)
	session, err := client.CreateSession(context.Background(), SessionRequest{
		WorkingDir: scenario.ExpectedCWD,
		Prompt:     []byte(scenario.ExpectedPrompt),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	gotUpdates := collectSessionUpdates(t, session)
	if len(gotUpdates) != 1001 {
		t.Fatalf("unexpected update count: got %d want 1001", len(gotUpdates))
	}
	textChunks := make(map[string]struct{}, 1000)
	for _, block := range flattenBlocks(gotUpdates) {
		if block.Type != model.BlockText {
			continue
		}
		textBlock, err := block.AsText()
		if err != nil {
			t.Fatalf("decode streamed text block: %v", err)
		}
		textChunks[textBlock.Text] = struct{}{}
	}
	if len(textChunks) != 1000 {
		t.Fatalf("expected all 1000 streamed chunks, got %d", len(textChunks))
	}
	if _, ok := textChunks["chunk-0000"]; !ok {
		t.Fatal("expected streamed chunks to include chunk-0000")
	}
	if _, ok := textChunks["chunk-0999"]; !ok {
		t.Fatal("expected streamed chunks to include chunk-0999")
	}
	if got := session.DroppedUpdates(); got != 0 {
		t.Fatalf("expected zero dropped updates, got %d", got)
	}
	if got := session.SlowPublishes(); got != 0 {
		t.Fatalf("expected zero slow publishes for buffered stream, got %d", got)
	}
	if gotUpdates[len(gotUpdates)-1].Status != model.StatusCompleted {
		t.Fatalf("unexpected final status: %q", gotUpdates[len(gotUpdates)-1].Status)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
}

func TestSessionUpdateNeverDropsCompletionWhenDrainBrieflyStalls(t *testing.T) {
	// A backpressure window that would immediately drop a non-critical update,
	// proving the completion survives because it is critical, not because the
	// window is generous.
	setSessionPublishBackpressureTimeout(t, time.Millisecond)

	sessionID := "sess-integration"
	session := newSession(sessionID)
	session.updates = make(chan model.SessionUpdate, 1)
	// Saturate the buffer so the incoming completion cannot land until the drain
	// resumes.
	session.updates <- model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk}

	client := &clientImpl{sessions: map[string]*sessionImpl{sessionID: session}}

	notification := acp.SessionNotification{
		SessionId: acp.SessionId(sessionID),
		Update: acp.UpdateToolCall(
			acp.ToolCallId("tool-1"),
			acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
		),
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.SessionUpdate(context.Background(), notification)
	}()

	// The completion is critical: with the buffer saturated it must block (the
	// drain is stalled) rather than drop.
	waitForActivePublish(t, session)

	first := mustReceiveSessionUpdate(t, session.updates)
	if first.Kind != model.UpdateKindAgentMessageChunk {
		t.Fatalf("unexpected first drained update: %q", first.Kind)
	}
	completion := mustReceiveSessionUpdate(t, session.updates)
	if completion.Kind != model.UpdateKindToolCallUpdated ||
		completion.ToolCallState != model.ToolCallStateCompleted {
		t.Fatalf(
			"completion not delivered downstream: kind=%q state=%q",
			completion.Kind,
			completion.ToolCallState,
		)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("SessionUpdate returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SessionUpdate did not return after the completion was delivered")
	}

	if got := session.DroppedUpdates(); got != 0 {
		t.Fatalf("completion must never drop, got dropped=%d", got)
	}
}

func newTestSessionWithBuffer(id string, bufferCap int) *sessionImpl {
	session := newSession(id)
	session.updates = make(chan model.SessionUpdate, bufferCap)
	return session
}

func setSessionPublishBackpressureTimeout(t *testing.T, timeout time.Duration) {
	t.Helper()

	previous := sessionPublishBackpressureTimeout
	sessionPublishBackpressureTimeout = timeout
	t.Cleanup(func() {
		sessionPublishBackpressureTimeout = previous
	})
}

func waitForActivePublish(t *testing.T, session *sessionImpl) {
	t.Helper()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		session.mu.RLock()
		activePublishes := session.activePublishes
		session.mu.RUnlock()
		if activePublishes > 0 {
			return
		}

		select {
		case <-timer.C:
			t.Fatal("timed out waiting for active session publish")
		case <-ticker.C:
		}
	}
}

func mustReceiveSessionUpdate(t *testing.T, ch <-chan model.SessionUpdate) model.SessionUpdate {
	t.Helper()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	select {
	case update := <-ch:
		return update
	case <-timer.C:
		t.Fatal("timed out waiting for session update")
		return model.SessionUpdate{}
	}
}

func captureDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
	})
	return &buf
}

func decodeLogRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()

	raw := strings.TrimSpace(buf.String())
	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")
	records := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode log record %q: %v", line, err)
		}
		records = append(records, record)
	}
	return records
}
