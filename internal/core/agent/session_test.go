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
		setSessionPublishBackpressureTimeout(t, 200*time.Millisecond)

		session := newTestSessionWithBuffer("sess-slow", 1)
		session.updates <- model.SessionUpdate{Kind: model.UpdateKindPlanUpdated}

		timer := time.NewTimer(25 * time.Millisecond)
		defer timer.Stop()
		go func() {
			<-timer.C
			<-session.updates
		}()

		start := time.Now()
		update := model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk}
		session.publish(context.Background(), update)
		if elapsed := time.Since(start); elapsed < 25*time.Millisecond {
			t.Fatalf("expected publish to wait for backpressure, returned after %v", elapsed)
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
