package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestContextCompactionDispatchesHooksAndUsesPatchedParams(t *testing.T) {
	t.Parallel()

	session := &Session{
		ID:          "sess-context",
		AgentName:   "coder",
		Workspace:   "/tmp/workspace",
		WorkspaceID: "ws-context",
		Type:        SessionTypeUser,
		State:       StateActive,
	}

	var (
		prePayload     hookspkg.ContextPreCompactPayload
		compactPayload hookspkg.ContextPreCompactPayload
		postPayload    hookspkg.ContextPostCompactPayload
	)

	dispatcher := &spyHookDispatcher{
		dispatchContextPreCompactFn: func(_ context.Context, payload hookspkg.ContextPreCompactPayload) (hookspkg.ContextPreCompactPayload, error) {
			prePayload = payload
			patchedReason := "token_limit"
			patchedStrategy := "summary"
			payload.Reason = patchedReason
			payload.Strategy = patchedStrategy
			payload.ContextBlocks = []hookspkg.ContextBlock{{Kind: "summary", Text: "patched"}}
			return payload, nil
		},
		dispatchContextPostCompactFn: func(_ context.Context, payload hookspkg.ContextPostCompactPayload) (hookspkg.ContextPostCompactPayload, error) {
			postPayload = payload
			return payload, nil
		},
	}

	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
	result, err := h.manager.runContextCompaction(
		testutil.Context(t),
		session,
		"turn-compact",
		"manual",
		"noop",
		"",
		[]hookspkg.ContextBlock{{Kind: "note", Text: "before"}},
		func(_ context.Context, payload hookspkg.ContextPreCompactPayload) (hookspkg.ContextPostCompactPayload, error) {
			compactPayload = payload
			return hookspkg.ContextPostCompactPayload{
				Summary:       "after",
				ContextBlocks: []hookspkg.ContextBlock{{Kind: "summary", Text: "after"}},
			}, nil
		},
	)
	if err != nil {
		t.Fatalf("runContextCompaction() error = %v", err)
	}

	if prePayload.Reason != "manual" || prePayload.Strategy != "noop" {
		t.Fatalf("pre-compaction payload = %#v, want original reason/strategy", prePayload)
	}
	if compactPayload.Reason != "token_limit" || compactPayload.Strategy != "summary" {
		t.Fatalf("compaction payload = %#v, want patched reason/strategy", compactPayload)
	}
	if len(compactPayload.ContextBlocks) != 1 || compactPayload.ContextBlocks[0].Text != "patched" {
		t.Fatalf("compaction context blocks = %#v, want patched blocks", compactPayload.ContextBlocks)
	}
	if postPayload.Summary != "after" {
		t.Fatalf("post-compaction summary = %q, want %q", postPayload.Summary, "after")
	}
	if postPayload.Reason != "token_limit" || postPayload.Strategy != "summary" {
		t.Fatalf("post-compaction reason/strategy = %#v, want patched values", postPayload)
	}
	if result.Summary != "after" {
		t.Fatalf("result summary = %q, want %q", result.Summary, "after")
	}
}

func TestPressureCompactionArchivesCoveredReplaySpans(t *testing.T) {
	t.Parallel()

	t.Run("Should fire once above pressure and cover the summary before archive", func(t *testing.T) {
		t.Parallel()

		order := make([]string, 0, 3)
		var active *Session
		handler := &compactionHandlerStub{compact: func(
			_ context.Context,
			request CompactionRequest,
		) (CompactionResult, error) {
			order = append(order, "summary")
			if request.FromSequence != 1 || request.ToSequence != 2 || len(request.Events) != 2 {
				return CompactionResult{}, fmt.Errorf("request = %#v, want complete sequence range 1..2", request)
			}
			archived, err := active.recorderHandle().Query(
				testutil.Context(t),
				store.EventQuery{Archive: store.EventArchiveArchived},
			)
			if err != nil {
				return CompactionResult{}, err
			}
			if len(archived) != 0 {
				return CompactionResult{}, fmt.Errorf("archived before summary coverage: %#v", archived)
			}
			return CompactionResult{Summary: "Covered cobalt decision."}, nil
		}}
		dispatcher := &spyHookDispatcher{
			dispatchContextPreCompactFn: func(
				_ context.Context,
				payload hookspkg.ContextPreCompactPayload,
			) (hookspkg.ContextPreCompactPayload, error) {
				order = append(order, "pre")
				return payload, nil
			},
			dispatchContextPostCompactFn: func(
				_ context.Context,
				payload hookspkg.ContextPostCompactPayload,
			) (hookspkg.ContextPostCompactPayload, error) {
				archived, err := active.recorderHandle().Query(
					testutil.Context(t),
					store.EventQuery{Archive: store.EventArchiveArchived},
				)
				if err != nil {
					return payload, err
				}
				if len(archived) != 2 {
					return payload, fmt.Errorf("post hook archived rows = %d, want 2", len(archived))
				}
				order = append(order, "post")
				return payload, nil
			},
		}
		h := newHarness(
			t,
			WithHookSet(fullHookSet(dispatcher)),
			WithSessionCompactionConfig(compozyconfig.SessionCompactionConfig{
				Enabled:            true,
				PressureThreshold:  0.85,
				MaxAttemptsPerTurn: 1,
				FailureCooldown:    10 * time.Minute,
			}),
			WithCompactionHandler(handler),
		)
		active = createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(
				testutil.Context(t),
				active.ID,
			); err != nil && !errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() error = %v", err)
			}
		})
		recordCompactionTurn(t, h.manager, active, "turn-old", "cobalt decision", true)
		used, size := int64(90), int64(100)
		usage := acp.TokenUsage{TurnID: "turn-current", ContextUsed: &used, ContextSize: &size}
		below := int64(80)
		if err := h.manager.maybeCompact(active, acp.TokenUsage{
			TurnID: "turn-current", ContextUsed: &below, ContextSize: &size,
		}); err != nil {
			t.Fatalf("maybeCompact(below) error = %v", err)
		}
		if handler.callCount() != 0 {
			t.Fatalf("handler calls below threshold = %d, want 0", handler.callCount())
		}
		if err := h.manager.observeRecordAndNotifyPromptEvent(
			testutil.Context(t),
			active,
			nil,
			&promptPumpLoopState{},
			acp.AgentEvent{
				Type:      acp.EventTypeUsage,
				TurnID:    usage.TurnID,
				Timestamp: time.Now().UTC(),
				Usage:     &usage,
			},
			false,
		); err != nil {
			t.Fatalf("observeRecordAndNotifyPromptEvent(usage) error = %v", err)
		}
		if err := h.manager.waitForCompactions(testutil.Context(t)); err != nil {
			t.Fatalf("waitForCompactions() error = %v", err)
		}
		if got := strings.Join(order, ","); got != "pre,summary,post" {
			t.Fatalf("compaction order = %q, want pre,summary,post", got)
		}
		if handler.callCount() != 1 {
			t.Fatalf("handler calls = %d, want 1", handler.callCount())
		}
		if err := h.manager.maybeCompact(active, usage); err != nil {
			t.Fatalf("maybeCompact(repeated turn) error = %v", err)
		}
		if handler.callCount() != 1 {
			t.Fatalf("handler calls after per-turn retry = %d, want 1", handler.callCount())
		}

		all, err := active.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
		if err != nil {
			t.Fatalf("Query(all) error = %v", err)
		}
		archived, err := active.recorderHandle().Query(
			testutil.Context(t),
			store.EventQuery{Archive: store.EventArchiveArchived},
		)
		if err != nil {
			t.Fatalf("Query(archived) error = %v", err)
		}
		if len(all) != 4 || len(archived) != 2 {
			t.Fatalf("all/archived rows = %d/%d, want 4/2", len(all), len(archived))
		}
		fired := storedEventByType(t, all, eventspkg.SessionCompactionFired)
		payload := decodeStoredEventPayload(t, fired)
		raw, ok := payload["raw"].(map[string]any)
		if !ok {
			t.Fatalf("compaction event raw = %#v, want object", payload["raw"])
		}
		if raw["workspace_id"] != active.WorkspaceID || raw["session_id"] != active.ID ||
			raw["from_sequence"] != float64(1) || raw["to_sequence"] != float64(2) {
			t.Fatalf("compaction event raw = %#v, want mandatory identity and range", raw)
		}
	})

	t.Run("Should enforce attempt caps and failure cooldown without archiving", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
		providerErr := errors.New("checkpoint provider unavailable")
		handler := &compactionHandlerStub{}
		handler.compact = func(_ context.Context, _ CompactionRequest) (CompactionResult, error) {
			if handler.callCount() == 1 {
				return CompactionResult{}, providerErr
			}
			return CompactionResult{Summary: "Recovered coverage."}, nil
		}
		h := newHarness(
			t,
			WithNow(func() time.Time { return now }),
			WithSessionCompactionConfig(compozyconfig.SessionCompactionConfig{
				Enabled:            true,
				PressureThreshold:  0.85,
				MaxAttemptsPerTurn: 1,
				FailureCooldown:    10 * time.Minute,
			}),
			WithCompactionHandler(handler),
		)
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(
				testutil.Context(t),
				active.ID,
			); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() error = %v", err)
			}
		})
		recordCompactionTurn(t, h.manager, active, "turn-old", "stable context", true)
		used, size := int64(90), int64(100)
		first := acp.TokenUsage{TurnID: "turn-failed", ContextUsed: &used, ContextSize: &size}
		if err := h.manager.maybeCompact(active, first); err != nil {
			t.Fatalf("maybeCompact(first) error = %v", err)
		}
		if err := h.manager.waitForCompactions(testutil.Context(t)); err != nil {
			t.Fatalf("waitForCompactions(first) error = %v", err)
		}
		archived, err := active.recorderHandle().Query(
			testutil.Context(t),
			store.EventQuery{Archive: store.EventArchiveArchived},
		)
		if err != nil {
			t.Fatalf("Query(after failure) error = %v", err)
		}
		if len(archived) != 0 {
			t.Fatalf("archived after failed summary = %#v, want none", archived)
		}

		if err := h.manager.maybeCompact(active, first); err != nil {
			t.Fatalf("maybeCompact(same turn) error = %v", err)
		}
		second := acp.TokenUsage{TurnID: "turn-cooldown", ContextUsed: &used, ContextSize: &size}
		if err := h.manager.maybeCompact(active, second); err != nil {
			t.Fatalf("maybeCompact(cooldown) error = %v", err)
		}
		if handler.callCount() != 1 {
			t.Fatalf("handler calls during cap/cooldown = %d, want 1", handler.callCount())
		}
		now = now.Add(11 * time.Minute)
		if err := h.manager.maybeCompact(active, second); err != nil {
			t.Fatalf("maybeCompact(after cooldown) error = %v", err)
		}
		if err := h.manager.waitForCompactions(testutil.Context(t)); err != nil {
			t.Fatalf("waitForCompactions(second) error = %v", err)
		}
		if handler.callCount() != 2 {
			t.Fatalf("handler calls after cooldown = %d, want 2", handler.callCount())
		}
	})

	t.Run("Should disable compaction and hooks at zero pressure", func(t *testing.T) {
		t.Parallel()

		preCalls := 0
		dispatcher := &spyHookDispatcher{dispatchContextPreCompactFn: func(
			_ context.Context,
			payload hookspkg.ContextPreCompactPayload,
		) (hookspkg.ContextPreCompactPayload, error) {
			preCalls++
			return payload, nil
		}}
		handler := &compactionHandlerStub{}
		h := newHarness(
			t,
			WithHookSet(fullHookSet(dispatcher)),
			WithSessionCompactionConfig(compozyconfig.SessionCompactionConfig{
				Enabled:            true,
				PressureThreshold:  0,
				MaxAttemptsPerTurn: 1,
				FailureCooldown:    time.Minute,
			}),
			WithCompactionHandler(handler),
		)
		active := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(
				testutil.Context(t),
				active.ID,
			); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop() error = %v", err)
			}
		})
		used, size := int64(100), int64(100)
		if err := h.manager.maybeCompact(active, acp.TokenUsage{
			TurnID: "turn-disabled", ContextUsed: &used, ContextSize: &size,
		}); err != nil {
			t.Fatalf("maybeCompact(disabled) error = %v", err)
		}
		if handler.callCount() != 0 || preCalls != 0 {
			t.Fatalf("disabled handler/pre-hook calls = %d/%d, want 0/0", handler.callCount(), preCalls)
		}
	})

	t.Run("Should stop at the first incomplete prior turn", func(t *testing.T) {
		t.Parallel()

		events := []store.SessionEvent{
			{Sequence: 1, TurnID: "turn-complete", Type: acp.EventTypeUserMessage},
			{Sequence: 2, TurnID: "turn-complete", Type: acp.EventTypeDone},
			{Sequence: 3, TurnID: "turn-incomplete", Type: acp.EventTypeAgentMessage},
			{Sequence: 4, TurnID: "turn-later", Type: acp.EventTypeUserMessage},
			{Sequence: 5, TurnID: "turn-later", Type: acp.EventTypeDone},
			{Sequence: 6, TurnID: "turn-current", Type: acp.EventTypeUsage},
		}
		span := completePriorTurnPrefix(events, "turn-current")
		if len(span) != 2 || span[0].Sequence != 1 || span[1].Sequence != 2 {
			t.Fatalf("completePriorTurnPrefix() = %#v, want only first complete turn", span)
		}
	})

	t.Run("Should include standalone markers before later complete turns", func(t *testing.T) {
		t.Parallel()

		events := []store.SessionEvent{
			{Sequence: 1, TurnID: "turn-marker", Type: eventspkg.TranscriptMarkerCreated},
			{Sequence: 2, TurnID: "clarify:req-1", Type: acp.EventTypeClarify},
			{Sequence: 3, TurnID: "turn-complete", Type: acp.EventTypeUserMessage},
			{Sequence: 4, TurnID: "turn-complete", Type: acp.EventTypeDone},
			{Sequence: 5, TurnID: "turn-current", Type: acp.EventTypeUsage},
		}
		span := completePriorTurnPrefix(events, "turn-current")
		if len(span) != 4 || span[0].Sequence != 1 || span[3].Sequence != 4 {
			t.Fatalf("completePriorTurnPrefix() = %#v, want standalone events plus complete turn", span)
		}
	})

	t.Run("Should join a canceled compaction before closing the recorder", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		canceled := make(chan struct{})
		release := make(chan struct{})
		handler := &compactionHandlerStub{compact: func(
			ctx context.Context,
			_ CompactionRequest,
		) (CompactionResult, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-release
			return CompactionResult{}, ctx.Err()
		}}
		h := newHarness(
			t,
			WithSessionCompactionConfig(compozyconfig.DefaultSessionCompactionConfig()),
			WithCompactionHandler(handler),
		)
		active := createSession(t, h)
		recordCompactionTurn(t, h.manager, active, "turn-old", "durable context", true)
		used, size := int64(90), int64(100)
		if err := h.manager.maybeCompact(active, acp.TokenUsage{
			TurnID: "turn-current", ContextUsed: &used, ContextSize: &size,
		}); err != nil {
			t.Fatalf("maybeCompact() error = %v", err)
		}
		<-started

		stopDone := make(chan error, 1)
		go func() {
			stopDone <- h.manager.Stop(testutil.Context(t), active.ID)
		}()
		<-canceled
		select {
		case err := <-stopDone:
			t.Fatalf("Stop() returned before compaction joined: %v", err)
		default:
		}
		if _, err := active.recorderHandle().Query(testutil.Context(t), store.EventQuery{}); err != nil {
			t.Fatalf("Query() while stop waits for compaction error = %v", err)
		}
		close(release)
		if err := <-stopDone; err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		if active.recorderHandle() != nil {
			t.Fatal("recorderHandle() != nil after joined stop")
		}
	})
}
