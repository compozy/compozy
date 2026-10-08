package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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

	budget := replayBudget{MaxBytes: 8192, MaxMessageBytes: 4096}

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

	for _, pin := range []bool{false, true} {
		for _, below := range []bool{false, true} {
			t.Run(
				fmt.Sprintf(
					"Should retain the newest fitting suffix with first-user pin %t and below-eight budget %t",
					pin,
					below,
				),
				func(t *testing.T) {
					t.Parallel()
					messages := deriveTestMessages(12, 100)
					for index := range 4 {
						messages[index].Content = strings.Repeat("older", 400)
					}
					omitted := 4
					candidate := []transcript.Message{}
					if pin {
						candidate = append(candidate, messages[0])
						omitted--
					}
					candidate = append(candidate, omittedReplayNote(omitted, messages[3].Timestamp))
					candidate = append(candidate, messages[4:]...)
					budget := replayBudget{
						MaxBytes:        replayArrayBytes(candidate),
						MaxMessageBytes: 4096,
						PinFirstUser:    pin,
					}
					wantStart := 4
					if below {
						budget.MaxBytes--
						wantStart++
						omitted++
					}
					bounded, stats := boundReplay(messages, budget)
					if stats.OmittedCount != omitted || stats.Bytes > budget.MaxBytes || stats.FirstUserPinned != pin {
						t.Fatalf(
							"suffix bound stats = %+v, want %d omissions, pin=%t and bytes <= %d",
							stats,
							omitted,
							pin,
							budget.MaxBytes,
						)
					}
					if pin && bounded[0].ID != messages[0].ID {
						t.Fatalf("pinned first user = %q, want %q", bounded[0].ID, messages[0].ID)
					}
					retained := bounded[len(bounded)-(len(messages)-wantStart):]
					if !slices.EqualFunc(retained, messages[wantStart:], func(got, want transcript.Message) bool {
						return got.ID == want.ID && got.Role == want.Role && got.Content == want.Content &&
							got.Timestamp.Equal(want.Timestamp)
					}) {
						t.Fatalf("retained suffix = %+v, want unchanged messages from %d onward", retained, wantStart)
					}
				},
			)
		}
	}

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

	for _, firstBytes := range []int{1024, 40 << 10} {
		t.Run(
			fmt.Sprintf("Should pin the first user request with %d bytes before omitted history", firstBytes),
			func(t *testing.T) {
				t.Parallel()
				messages := deriveTestMessages(300, 1024)
				messages[0].Content = strings.Repeat("request", firstBytes/7)
				messages[298] = transcript.Message{ID: "recorded-tool", Role: transcript.RoleToolCall,
					ToolName: "compozy__task_read", ToolInput: json.RawMessage(`{"query":"original"}`)}
				pinnedBudget := replayBudget{MaxBytes: 32768, MaxMessageBytes: 16384, PinFirstUser: true}
				bounded, stats := boundReplay(messages, pinnedBudget)
				if !stats.FirstUserPinned || bounded[0].ID != messages[0].ID ||
					bounded[1].Content != fmt.Sprintf(deriveOmittedFmt, stats.OmittedCount) {
					t.Fatalf("pin and omission = %+v / %+v", bounded[:2], stats)
				}
				if stats.MessageCount+stats.OmittedCount != len(messages) || stats.Bytes > pinnedBudget.MaxBytes {
					t.Fatalf("stats = %+v, want kept+dropped=%d within budget", stats, len(messages))
				}
				for index, message := range bounded[len(bounded)-8:] {
					want := messages[len(messages)-8+index]
					if replayMessageBytes(message) != replayMessageBytes(want) ||
						!slices.Equal(message.ToolInput, want.ToolInput) ||
						message.ID != want.ID ||
						message.Role != want.Role ||
						message.ToolName != want.ToolName ||
						message.Content != want.Content {
						t.Fatalf("protected tail message = %+v, want %+v", message, want)
					}
				}
				if firstBytes > pinnedBudget.MaxMessageBytes &&
					(!strings.HasSuffix(bounded[0].Content, deriveTruncatedMark) || replayMessageBytes(bounded[0]) > pinnedBudget.MaxMessageBytes) {
					t.Fatalf("original request was not capped: %+v", bounded[0])
				}
			},
		)
	}
	t.Run("Should move a retained first user ahead of omitted non-user history", func(t *testing.T) {
		t.Parallel()
		messages := deriveTestMessages(3, 50)
		messages[0].Role, messages[0].Content = transcript.RoleSystem, strings.Repeat("old", 2000)
		messages[1].Role = transcript.RoleUser
		bounded, stats := boundReplay(
			messages,
			replayBudget{MaxBytes: 1024, MaxMessageBytes: 8192, PinFirstUser: true},
		)
		if len(bounded) != 3 || bounded[0].ID != messages[1].ID || bounded[1].ID != deriveOmittedMessageID ||
			bounded[2].ID != messages[2].ID ||
			!stats.FirstUserPinned ||
			stats.OmittedCount != 1 {
			t.Fatalf("pin retained behind omitted non-user history: %+v, %+v", bounded, stats)
		}
	})

	t.Run("Should leave a fitting pinned transcript unchanged", func(t *testing.T) {
		t.Parallel()
		messages := deriveTestMessages(8, 80)
		pinnedBudget := budget
		pinnedBudget.PinFirstUser = true
		bounded, stats := boundReplay(messages, pinnedBudget)
		before, _ := json.Marshal(messages)
		after, _ := json.Marshal(bounded)
		if !bytes.Equal(before, after) || stats.FirstUserPinned || stats.OmittedCount != 0 {
			t.Fatalf("fitting transcript changed: %+v", stats)
		}
	})

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

	t.Run("Should not flag lifecycle-only turns of a stopped or unprompted session as in progress", func(t *testing.T) {
		t.Parallel()
		stopMarker, err := json.Marshal(map[string]any{"marker": map[string]any{"kind": "session_stopped"}})
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		unprompted := []store.SessionEvent{
			event(1, "h1", "hook.dispatch.start"),
			event(2, "h2", "hook.dispatch.complete"),
		}
		if turnID, _, laterOpen := lastSettledTurn(unprompted); turnID != "" || laterOpen {
			t.Fatalf("lastSettledTurn(unprompted) = %q/%t, want no turn and nothing open", turnID, laterOpen)
		}
		stopped := slices.Concat(unprompted, []store.SessionEvent{
			event(3, "t1", acp.EventTypeUserMessage),
			event(4, "t1", acp.EventTypeAgentMessage),
			event(5, "t1", acp.EventTypeDone),
			event(6, "s1", "session.stop_escalated"),
			event(7, "s1", "session_stopped"),
			{
				Sequence: 8,
				TurnID:   "s1",
				Type:     "transcript_marker.created",
				Content:  string(stopMarker),
			},
		})
		turnID, through, laterOpen := lastSettledTurn(stopped)
		if turnID != "t1" || through != 5 || laterOpen {
			t.Fatalf("lastSettledTurn(stopped) = %q/%d/%t, want t1/5/false", turnID, through, laterOpen)
		}
		interrupted := slices.Concat(stopped, []store.SessionEvent{event(9, "t2", acp.EventTypeUserMessage)})
		if _, _, laterOpen := lastSettledTurn(interrupted); !laterOpen {
			t.Fatal("lastSettledTurn(prompt turn without a terminal event) laterOpen = false, want true")
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
		text := `{"code":-32603,"message":"429 rate limit exceeded"}; provider_failure_kind=rate_limited; ` +
			"next_action=" + string(action) + "; guidance=retry later"
		return acp.AgentEvent{
			Type:    acp.EventTypeError,
			Error:   text,
			Failure: &store.SessionFailure{Kind: store.FailurePrompt, Summary: text},
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
			// The error text is what the prompt stream shows; it must agree with the summary.
			for field, text := range map[string]string{"error": event.Error, "failure summary": event.Failure.Summary} {
				if !strings.Contains(text, "next_action=handoff") ||
					!strings.Contains(text, "compozy session continue sess_1 --agent <name>") ||
					!strings.HasPrefix(text, `{"code":-32603,"message":"429 rate limit exceeded"}`) {
					t.Fatalf("%s = %q, want the handoff metadata after the unchanged provider text", field, text)
				}
			}
		})
	}

	t.Run("Should keep the classifier action for spawned sessions", func(t *testing.T) {
		t.Parallel()
		for _, code := range []string{acp.ProviderErrorRateLimited, acp.ProviderErrorAuthRequired} {
			event := promptFailure(code, acp.ProviderFailureActionLogin)
			decorateHandoffAction(SessionTypeSpawned, "spawn-with-user-text", &event)
			if event.ProviderError.NextAction != acp.ProviderFailureActionLogin ||
				!strings.Contains(event.Error, "next_action=login") {
				t.Fatalf("%s on spawned session = %s / %q, want login unchanged", code,
					event.ProviderError.NextAction, event.Error)
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
			err := (&Manager{}).validateDeriveSource(t.Context(), spec, &deriveSnapshot{meta: tc.meta})
			if !errors.Is(err, tc.want) {
				t.Fatalf("validateDeriveSource() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestReplayBudgetWorkspaceOverlay(t *testing.T) {
	t.Parallel()
	t.Run("Should use the same workspace overlay for resume and derive", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		session := createSession(t, h)
		workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		workspace.Config.Session.Derive = compozyconfig.SessionDeriveConfig{MaxReplayBytes: 8192, MaxMessageBytes: 4096}
		h.resolver.upsert(&workspace)
		resolved, err := resolveStoredSessionWorkspace(
			t.Context(),
			new(session.Meta()),
			h.manager.workspace,
			h.manager.profileNames,
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, ws := range []*workspacepkg.ResolvedWorkspace{&workspace, &resolved} {
			budget := h.manager.deriveBudget(ws)
			if budget.MaxBytes != 8192 || budget.MaxMessageBytes != 4096 ||
				!budget.PinFirstUser {
				t.Fatalf("replay budget = %+v, want workspace limits with pin", budget)
			}
		}
	})
	for _, historyAvailable := range []bool{false, true} {
		name := "Should rebuild cached replay under current bounds without a removed history tool"
		if historyAvailable {
			name = "Should keep the omission pointer under the current effective history tool"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			sess := createSession(t, h)
			messages := deriveTestMessages(30, 6000)
			encoded, err := json.Marshal(messages)
			if err != nil {
				t.Fatal(err)
			}
			cached := renderResumeReplay(sess.ID, nil, string(encoded), true, true)
			workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
			if err != nil {
				t.Fatal(err)
			}
			workspace.Config.Session.Derive = compozyconfig.SessionDeriveConfig{
				MaxReplayBytes:  8192,
				MaxMessageBytes: 4096,
			}
			replay, _, err := h.manager.reboundResumeReplay(sess, cached, rebuildReplayContext{
				workspace: &workspace, historyAvailable: historyAvailable, reason: "resume",
			})
			if err != nil {
				t.Fatal(err)
			}
			bounded := resumeReplayMessagesFromPrompt(t, replay)
			if replayArrayBytes(bounded) > 8192 || len(bounded) == 0 || bounded[0].ID != messages[0].ID {
				t.Fatalf(
					"replay bytes=%d count=%d, want <=8192 with original request",
					replayArrayBytes(bounded),
					len(bounded),
				)
			}
			for _, message := range bounded {
				if replayMessageBytes(message) > 4096 {
					t.Fatalf("message bytes=%d, want <=4096", replayMessageBytes(message))
				}
			}
			if strings.Contains(replay, "Read them with the compozy__session_history tool") != historyAvailable {
				t.Fatalf("effective history pointer disagrees with availability %t", historyAvailable)
			}
		})
	}
}

// Invariant: derive readers rebuild a stale retained prefix without mutating their source.
// Owner: derive snapshot; canonical suite: derive_test.go.
func TestDeriveRewindBaselineRebuildsStaleState(t *testing.T) {
	t.Parallel()
	t.Run("Should reconstruct restored rewind context using a read-only recorder", func(t *testing.T) {
		t.Parallel()
		path := store.SessionDBFile(t.TempDir())
		owner := store.SessionDBOwner{SessionID: "legacy-derive", WorkspaceID: "workspace-derive"}
		seedLegacyRewindDatabase(t, path, owner)
		writer, err := sessiondb.OpenSessionDB(t.Context(), owner, path)
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(t.Context()); err != nil {
			t.Fatal(err)
		}
		reader, err := sessiondb.OpenSessionDBReadOnly(t.Context(), owner, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reader.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		before, found, err := reader.ConversationRewindState(t.Context())
		if err != nil || !found || !before.BaselineStale {
			t.Fatalf("before=%+v found=%v err=%v", before, found, err)
		}
		messages, covered, err := deriveRewindBaseline(t.Context(), reader)
		if err != nil || covered != 149 || len(messages) != 149 {
			t.Fatalf("derive messages=%d covered=%d err=%v", len(messages), covered, err)
		}
		for index, message := range messages {
			if want := fmt.Sprintf("message-%03d", index+1); message.Content != want {
				t.Fatalf("message %d=%q want=%q", index, message.Content, want)
			}
		}
		after, found, err := reader.ConversationRewindState(t.Context())
		if err != nil || !found || after != before {
			t.Fatalf(
				"read-only derive changed stored baseline: before=%+v after=%+v found=%v err=%v",
				before,
				after,
				found,
				err,
			)
		}
	})
}
