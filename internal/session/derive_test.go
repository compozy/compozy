package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func deriveTestMessages(count int, bodyBytes int) []transcript.Message {
	messages := make([]transcript.Message, 0, count)
	base := time.Date(2026, 9, 11, 20, 0, 0, 0, time.UTC)
	for index := range count {
		role := transcript.RoleUser
		if index%2 == 1 {
			role = transcript.RoleAssistant
		}
		messages = append(messages, transcript.Message{
			ID: fmt.Sprintf("m%03d", index), Role: role,
			Content:   strings.Repeat("x", bodyBytes),
			Timestamp: base.Add(time.Duration(index) * time.Second),
		})
	}
	return messages
}

func TestBoundReplay(t *testing.T) {
	t.Parallel()

	budget := replayBudget{MaxBytes: 8192, MaxMessageBytes: 4096, KeepRecent: deriveProtectedTail}

	t.Run("Should keep everything and report no truncation within budget", func(t *testing.T) {
		t.Parallel()
		bounded, stats := boundReplay(deriveTestMessages(6, 50), budget)
		if len(bounded) != 6 || stats.MessageCount != 6 || stats.Truncated || stats.OmittedCount != 0 {
			t.Fatalf("boundReplay() = %d messages, stats %+v, want 6 untouched", len(bounded), stats)
		}
		for _, message := range bounded {
			if strings.Contains(message.Content, strings.TrimSpace(deriveTruncatedMark)) {
				t.Fatalf("message %s carries a truncation mark within budget", message.ID)
			}
		}
	})

	t.Run("Should drop oldest messages behind an omission note within the hard bound", func(t *testing.T) {
		t.Parallel()
		bounded, stats := boundReplay(deriveTestMessages(40, 400), budget)
		if !stats.Truncated || stats.OmittedCount == 0 || stats.OmittedCount+stats.MessageCount != 40 {
			t.Fatalf("stats = %+v, want truncated with omitted+kept == 40", stats)
		}
		want := fmt.Sprintf(deriveOmittedFmt, stats.OmittedCount)
		if bounded[0].Role != transcript.RoleSystem || bounded[0].Content != want {
			t.Fatalf("first message = %+v, want omission note %q", bounded[0], want)
		}
		if got := replayArrayBytes(bounded); got > budget.MaxBytes || got != stats.Bytes {
			t.Fatalf("serialized bytes = %d (stats %d), want <= %d", got, stats.Bytes, budget.MaxBytes)
		}
		if bounded[len(bounded)-1].ID != "m039" {
			t.Fatalf("last kept message = %s, want the newest m039", bounded[len(bounded)-1].ID)
		}
	})

	t.Run("Should drop protected messages down to one when the tail exceeds the budget", func(t *testing.T) {
		t.Parallel()
		bounded, stats := boundReplay(deriveTestMessages(8, 3500), budget)
		if !stats.Truncated || stats.MessageCount >= 8 || stats.MessageCount < 1 {
			t.Fatalf("stats = %+v, want protected tail reduced", stats)
		}
		if got := replayArrayBytes(bounded); got > budget.MaxBytes {
			t.Fatalf("serialized bytes = %d, want <= %d", got, budget.MaxBytes)
		}
	})

	for _, tc := range []struct {
		name     string
		messages []transcript.Message
	}{
		{name: "empty", messages: nil},
		{name: "one huge message", messages: deriveTestMessages(1, 100<<10)},
		{name: "eight huge messages", messages: deriveTestMessages(8, 100<<10)},
		{name: "two hundred small messages", messages: deriveTestMessages(200, 40)},
	} {
		t.Run("Should respect the hard bound for "+tc.name, func(t *testing.T) {
			t.Parallel()
			bounded, stats := boundReplay(tc.messages, budget)
			if got := replayArrayBytes(bounded); got > budget.MaxBytes || got != stats.Bytes {
				t.Fatalf("serialized bytes = %d (stats %d), want <= %d", got, stats.Bytes, budget.MaxBytes)
			}
		})
	}

	t.Run("Should cap a huge tool result with the truncation mark", func(t *testing.T) {
		t.Parallel()
		message := transcript.Message{
			ID: "tool", Role: transcript.RoleToolResult, ToolName: "shell",
			ToolResult: &transcript.ToolResult{Stdout: strings.Repeat("y", 100<<10)},
		}
		capped, cut := capReplayMessage(message, 16384)
		if !cut || replayMessageBytes(capped) > 16384 {
			t.Fatalf("capped size = %d cut=%t, want <= 16384", replayMessageBytes(capped), cut)
		}
		if !strings.HasSuffix(capped.ToolResult.Stdout, deriveTruncatedMark) {
			t.Fatalf("capped stdout does not end with %q", deriveTruncatedMark)
		}
		if len(message.ToolResult.Stdout) != 100<<10 {
			t.Fatal("capReplayMessage mutated its input")
		}
	})
}

func TestComposeImportedContextBlock(t *testing.T) {
	t.Parallel()

	t.Run("Should open with the framing lines and one fenced transcript", func(t *testing.T) {
		t.Parallel()
		encoded, err := json.Marshal(deriveTestMessages(2, 10))
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		block := composeImportedContextBlock(store.SessionImportedContext{
			SourceSessionID: "sess_src", Kind: store.LineageKindContinue, OriginAgentName: "codex",
			MessagesJSON: string(encoded),
		})
		for _, want := range []string{
			"Context rebuilt from log.\nThis session continues sess_src (agent: codex). Continue this session",
			deriveWorkspaceLine,
			resumeReplayOpenTag + "\n" + string(encoded) + "\n" + resumeReplayCloseTag,
		} {
			if !strings.Contains(block, want) {
				t.Fatalf("block = %q, want it to contain %q", block, want)
			}
		}
		if !strings.HasPrefix(block, contextRebuiltMarkerSummary) ||
			strings.Count(block, resumeReplayOpenTag) != 1 {
			t.Fatalf("block = %q, want one replay section opened by the marker line", block)
		}
	})

	t.Run("Should name the fork anchor in the framing line", func(t *testing.T) {
		t.Parallel()
		block := composeImportedContextBlock(store.SessionImportedContext{
			SourceSessionID: "sess_src", Kind: store.LineageKindFork, OriginAgentName: "codex",
			OriginMessageID: "msg_3", MessagesJSON: "[]",
		})
		if !strings.Contains(block, "This session was forked from sess_src through message msg_3 (agent: codex). ") {
			t.Fatalf("block = %q, want the fork line", block)
		}
	})
}

func TestLastSettledTurn(t *testing.T) {
	t.Parallel()

	event := func(sequence int64, turnID string, eventType string) store.SessionEvent {
		return store.SessionEvent{Sequence: sequence, TurnID: turnID, Type: eventType}
	}

	t.Run("Should cut at the last settled turn and flag a later open synthetic turn", func(t *testing.T) {
		t.Parallel()
		events := []store.SessionEvent{
			event(1, "t1", acp.EventTypeUserMessage),
			event(2, "t1", acp.EventTypeAgentMessage),
			event(3, "t1", acp.EventTypeDone),
			event(4, "t2", acp.EventTypeSyntheticReentry),
			event(5, "t2", acp.EventTypeToolCall),
		}
		turnID, through, laterOpen := lastSettledTurn(events)
		if turnID != "t1" || through != 3 || !laterOpen {
			t.Fatalf("lastSettledTurn() = %q/%d/%t, want t1/3/true", turnID, through, laterOpen)
		}
		carried := carriedEvents(events, 0, through)
		if len(carried) != 3 {
			t.Fatalf("carriedEvents() = %d events, want the 3 of t1", len(carried))
		}
		cut, err := resolveDeriveCut(
			store.TranscriptUserAnchor{MessageID: "msg_1", TurnID: "t1", StartSequence: 1},
			events,
		)
		if err != nil || !cut.TurnSettled || cut.ThroughSequence != 3 {
			t.Fatalf("resolveDeriveCut(t1 before open synthetic t2) = %+v, %v, want settled through 3", cut, err)
		}
		if _, err := resolveDeriveCut(store.TranscriptUserAnchor{MessageID: "msg_x", TurnID: "t9"}, events); !errors.Is(
			err, ErrDeriveMessageNotFound,
		) {
			t.Fatalf("resolveDeriveCut(unknown turn) error = %v, want ErrDeriveMessageNotFound", err)
		}
	})

	t.Run("Should treat error and cancel markers as settlement but not a trailing tool call", func(t *testing.T) {
		t.Parallel()
		cancelMarker, err := json.Marshal(map[string]any{
			"marker": map[string]any{"kind": transcript.MarkerPromptCancel},
		})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		events := []store.SessionEvent{
			event(1, "t1", acp.EventTypeUserMessage),
			event(2, "t1", acp.EventTypeError),
			event(3, "t2", acp.EventTypeUserMessage),
			{Sequence: 4, TurnID: "t2", Type: "transcript_marker.created", Content: string(cancelMarker)},
			event(5, "t3", acp.EventTypeUserMessage),
			event(6, "t3", acp.EventTypeToolCall),
		}
		turnID, through, laterOpen := lastSettledTurn(events)
		if turnID != "t2" || through != 4 || !laterOpen {
			t.Fatalf("lastSettledTurn() = %q/%d/%t, want t2/4/true", turnID, through, laterOpen)
		}
		if _, err := resolveDeriveCut(
			store.TranscriptUserAnchor{MessageID: "msg_3", StartSequence: 5},
			events,
		); !errors.Is(
			err,
			ErrDeriveTurnInProgress,
		) {
			t.Fatalf("resolveDeriveCut(open turn) error = %v, want ErrDeriveTurnInProgress", err)
		}
		cut, err := resolveDeriveCut(
			store.TranscriptUserAnchor{MessageID: "msg_1", TurnID: "t1", StartSequence: 1},
			events,
		)
		if err != nil || cut.TurnID != "t1" || cut.ThroughSequence != 2 || !cut.TurnSettled {
			t.Fatalf("resolveDeriveCut(settled) = %+v, %v, want t1 through 2", cut, err)
		}
	})

	t.Run("Should report no settled turn for an empty ledger", func(t *testing.T) {
		t.Parallel()
		turnID, through, laterOpen := lastSettledTurn(nil)
		if turnID != "" || through != 0 || laterOpen {
			t.Fatalf("lastSettledTurn(nil) = %q/%d/%t, want empty", turnID, through, laterOpen)
		}
	})
}

func TestDecorateHandoffAction(t *testing.T) {
	t.Parallel()

	promptFailure := func(code string, action acp.ProviderFailureAction) acp.AgentEvent {
		return acp.AgentEvent{
			Type: acp.EventTypeError,
			ProviderError: &acp.ProviderErrorDiagnostic{
				Code: code, Provider: "claude", NextAction: action, Guidance: "retry later",
			},
		}
	}

	for _, tc := range []struct {
		name string
		code string
	}{
		{name: "rate limited", code: acp.ProviderErrorRateLimited},
		{name: "auth required", code: acp.ProviderErrorAuthRequired},
	} {
		t.Run("Should prescribe handoff for a "+tc.name+" user session", func(t *testing.T) {
			t.Parallel()
			event := promptFailure(tc.code, acp.ProviderFailureActionRetry)
			original := event.ProviderError
			decorateHandoffAction(SessionTypeUser, "sess_1", &event)
			if event.ProviderError.NextAction != acp.ProviderFailureActionHandoff ||
				!strings.Contains(event.ProviderError.Guidance, "compozy session continue sess_1 --agent <name>") {
				t.Fatalf("provider error = %+v, want handoff with continue guidance", event.ProviderError)
			}
			if original.NextAction != acp.ProviderFailureActionRetry {
				t.Fatal("decorateHandoffAction mutated the classifier's diagnostic")
			}
		})
	}

	t.Run("Should keep the classifier action for spawned sessions", func(t *testing.T) {
		t.Parallel()
		for _, code := range []string{acp.ProviderErrorRateLimited, acp.ProviderErrorAuthRequired} {
			event := promptFailure(code, acp.ProviderFailureActionLogin)
			decorateHandoffAction(SessionTypeSpawned, "spawn-with-user-text", &event)
			if event.ProviderError.NextAction != acp.ProviderFailureActionLogin {
				t.Fatalf("%s on spawned session = %s, want login unchanged", code, event.ProviderError.NextAction)
			}
		}
	})
}

func TestDeriveValidation(t *testing.T) {
	t.Parallel()

	epoch := int64(3)
	for _, tc := range []struct {
		name string
		opts ContinueSessionOpts
		want string
	}{
		{
			name: "runtime with route",
			opts: ContinueSessionOpts{
				SourceSessionID: "s", WorkspaceID: "w", AgentName: "b", IdempotencyKey: "k", Route: 2,
				Runtime: &DeriveRuntime{Provider: "claude"},
			},
			want: "runtime and route are mutually exclusive",
		},
		{
			name: "partial fences",
			opts: ContinueSessionOpts{
				SourceSessionID: "s", WorkspaceID: "w", AgentName: "b", IdempotencyKey: "k",
				Fences: DeriveFences{ExpectedEpoch: &epoch},
			},
			want: "set all transcript fences together",
		},
		{
			name: "missing agent",
			opts: ContinueSessionOpts{SourceSessionID: "s", WorkspaceID: "w", IdempotencyKey: "k"},
			want: "agent name is required",
		},
	} {
		t.Run("Should reject "+tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := (&Manager{}).ContinueSession(t.Context(), tc.opts)
			if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ContinueSession() error = %v, want validation %q", err, tc.want)
			}
		})
	}
}

func TestValidateDeriveSource(t *testing.T) {
	t.Parallel()

	spec := deriveSpec{sourceID: "sess_src", workspaceID: "ws"}
	for _, tc := range []struct {
		name string
		meta store.SessionMeta
		want error
	}{
		{
			name: "a spawned source", want: ErrSessionNotDerivable,
			meta: store.SessionMeta{ID: "sess_src", WorkspaceID: "ws", SessionType: string(SessionTypeSpawned)},
		},
		{
			name: "a coordinator source", want: ErrSessionNotDerivable,
			meta: store.SessionMeta{ID: "sess_src", WorkspaceID: "ws", SessionType: string(SessionTypeCoordinator)},
		},
		{
			name: "a source in another workspace", want: ErrSessionNotFound,
			meta: store.SessionMeta{ID: "sess_src", WorkspaceID: "other", SessionType: string(SessionTypeUser)},
		},
	} {
		t.Run("Should refuse "+tc.name, func(t *testing.T) {
			t.Parallel()
			err := (&Manager{}).validateDeriveSource(t.Context(), spec, deriveSnapshot{meta: tc.meta})
			if !errors.Is(err, tc.want) {
				t.Fatalf("validateDeriveSource() error = %v, want %v", err, tc.want)
			}
		})
	}
}
