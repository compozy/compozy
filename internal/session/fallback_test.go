package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

// recordingEventLedger is the daemon-ledger fake: it records session.fallback.used rows
// and can fail a chosen write.
type recordingEventLedger struct {
	mu       sync.Mutex
	rows     []store.EventSummary
	failNext error
}

func (l *recordingEventLedger) WriteEventSummary(_ context.Context, summary store.EventSummary) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failNext != nil {
		err := l.failNext
		l.failNext = nil
		return err
	}
	l.rows = append(l.rows, summary)
	return nil
}

func (l *recordingEventLedger) ListEventSummaries(
	context.Context,
	store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]store.EventSummary(nil), l.rows...), nil
}

func (l *recordingEventLedger) fallbackRows() []store.EventSummary {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows := make([]store.EventSummary, 0, len(l.rows))
	for _, row := range l.rows {
		if row.Type == eventspkg.SessionFallbackUsed {
			rows = append(rows, row)
		}
	}
	return rows
}

func decodeFallbackPayload(t *testing.T, row store.EventSummary) sessionFallbackEventPayload {
	t.Helper()
	var payload sessionFallbackEventPayload
	if err := json.Unmarshal(row.Content, &payload); err != nil {
		t.Fatalf("json.Unmarshal(fallback row) error = %v", err)
	}
	return payload
}

func refusedRateLimit() error {
	return startupFailure("agent runtime startup failed", errors.New("HTTP 429 rate limit exceeded"))
}

func testFallbackRoutes() (FallbackRoute, []FallbackRoute) {
	primary := FallbackRoute{Provider: "claude", Model: "opus-4-8"}
	return primary, []FallbackRoute{
		{Provider: "claude", Model: "opus-4-8", Command: "CLAUDE_CONFIG_DIR=/seat2 claude --acp"},
		{Provider: "cursor", Model: "grok-4.6"},
	}
}

func TestBindWithFallback(t *testing.T) {
	t.Parallel()

	newSequence := func(ledger store.EventSummaryStore) fallbackSequence {
		return fallbackSequence{
			agent: "reviewer", phase: fallbackPhaseBind, ledger: ledger,
			effectiveCommand: func(FallbackRoute) string { return "cursor-agent acp" },
		}
	}
	manager := &Manager{now: time.Now}

	t.Run("Should record one event before the accepted fallback attempt", func(t *testing.T) {
		t.Parallel()
		ledger := &recordingEventLedger{}
		primary, routes := testFallbackRoutes()
		var calls []int
		value, index, err := manager.bindWithFallback(testutil.Context(t), newSequence(ledger), primary, routes,
			func(_ context.Context, attempt int, _ FallbackRoute) (string, error) {
				calls = append(calls, attempt)
				if attempt == 1 && len(ledger.fallbackRows()) != 1 {
					t.Errorf("attempt 1 started with %d ledger rows, want 1", len(ledger.fallbackRows()))
				}
				if attempt == 0 {
					return "", refusedRateLimit()
				}
				return "route-1", nil
			})
		if err != nil || value != "route-1" || index != 1 {
			t.Fatalf("bindWithFallback() = (%q, %d, %v), want (route-1, 1, nil)", value, index, err)
		}
		if len(calls) != 2 {
			t.Fatalf("attempts = %v, want primary and route 1", calls)
		}
		rows := ledger.fallbackRows()
		if len(rows) != 1 {
			t.Fatalf("ledger rows = %d, want 1", len(rows))
		}
		payload := decodeFallbackPayload(t, rows[0])
		want := compozyconfig.CommandFingerprint(routes[0].Command)
		if payload.Attempt != 1 || payload.Phase != fallbackPhaseBind || payload.Agent != "reviewer" ||
			payload.ProviderCommandFingerprint != want {
			t.Fatalf("payload = %#v, want attempt 1 bind reviewer with %s", payload, want)
		}
		if got := rows[0].Summary; got != "reviewer fallback attempt 1 (bind)" {
			t.Fatalf("summary = %q", got)
		}
		if strings.Contains(string(rows[0].Content), "/seat2") {
			t.Fatalf("ledger content leaked the raw command: %s", rows[0].Content)
		}
	})

	t.Run("Should stop without events when the primary is accepted", func(t *testing.T) {
		t.Parallel()
		ledger := &recordingEventLedger{}
		primary, routes := testFallbackRoutes()
		_, index, err := manager.bindWithFallback(testutil.Context(t), newSequence(ledger), primary, routes,
			func(context.Context, int, FallbackRoute) (int, error) { return 7, nil })
		if err != nil || index != 0 || len(ledger.fallbackRows()) != 0 {
			t.Fatalf("bindWithFallback() index=%d err=%v rows=%d, want 0/nil/0", index, err, len(ledger.fallbackRows()))
		}
	})

	t.Run("Should run a single attempt and return its error when the chain is empty", func(t *testing.T) {
		t.Parallel()
		ledger := &recordingEventLedger{}
		primary, _ := testFallbackRoutes()
		refused := refusedRateLimit()
		calls := 0
		_, _, err := manager.bindWithFallback(testutil.Context(t), newSequence(ledger), primary, nil,
			func(context.Context, int, FallbackRoute) (int, error) {
				calls++
				return 0, refused
			})
		if !errors.Is(err, refused) || calls != 1 || len(ledger.fallbackRows()) != 0 {
			t.Fatalf(
				"empty chain err=%v calls=%d rows=%d, want the attempt error, 1, 0",
				err,
				calls,
				len(ledger.fallbackRows()),
			)
		}
	})

	t.Run("Should treat an accepted-then-failed attempt as final", func(t *testing.T) {
		t.Parallel()
		ledger := &recordingEventLedger{}
		primary, routes := testFallbackRoutes()
		acceptedErr := acp.WrapAcceptedStart("acp_1", errors.New("set config option failed"))
		calls := 0
		_, index, err := manager.bindWithFallback(testutil.Context(t), newSequence(ledger), primary, routes,
			func(context.Context, int, FallbackRoute) (int, error) {
				calls++
				return 0, acceptedErr
			})
		if calls != 1 || index != 0 || !errors.Is(err, acceptedErr) {
			t.Fatalf("accepted-with-error calls=%d index=%d err=%v, want one attempt and that error", calls, index, err)
		}
	})

	t.Run("Should list every attempt in order and wrap the last cause on exhaustion", func(t *testing.T) {
		t.Parallel()
		primary, routes := testFallbackRoutes()
		last := errors.New("cursor refused")
		run := func() error {
			_, _, err := manager.bindWithFallback(testutil.Context(t), newSequence(&recordingEventLedger{}), primary,
				routes, func(_ context.Context, attempt int, _ FallbackRoute) (int, error) {
					if attempt == 2 {
						return 0, last
					}
					return 0, refusedRateLimit()
				})
			return err
		}
		err := run()
		if !errors.Is(err, last) {
			t.Fatalf("exhaustion error = %v, want the last cause wrapped", err)
		}
		text := err.Error()
		for _, want := range []string{
			"fallback chain exhausted after 3 attempt(s)",
			"attempt 1 (claude/opus-4-8) refused before acceptance",
			"attempt 2 (claude/opus-4-8) refused before acceptance",
			"attempt 3 (cursor/grok-4.6) refused before acceptance: cursor refused",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("exhaustion error %q missing %q", text, want)
			}
		}
		if strings.Index(text, "attempt 1") > strings.Index(text, "attempt 3") {
			t.Fatalf("exhaustion error %q is out of order", text)
		}
		if again := run(); again.Error() != text {
			t.Fatalf("exhaustion error is not deterministic: %q vs %q", again, text)
		}
	})

	t.Run("Should abort before the next attempt when the ledger write fails", func(t *testing.T) {
		t.Parallel()
		writeErr := errors.New("ledger unavailable")
		ledger := &recordingEventLedger{failNext: writeErr}
		primary, routes := testFallbackRoutes()
		refused := refusedRateLimit()
		calls := 0
		_, _, err := manager.bindWithFallback(testutil.Context(t), newSequence(ledger), primary, routes,
			func(context.Context, int, FallbackRoute) (int, error) {
				calls++
				return 0, refused
			})
		if calls != 1 || !errors.Is(err, writeErr) || !errors.Is(err, refused) {
			t.Fatalf("ledger failure calls=%d err=%v, want one attempt and the joined write error", calls, err)
		}
		if !strings.Contains(err.Error(), "session: record session.fallback.used: ledger unavailable") {
			t.Fatalf("ledger failure error = %q", err)
		}
	})

	t.Run("Should stop advancing when the context is canceled", func(t *testing.T) {
		t.Parallel()
		primary, routes := testFallbackRoutes()
		ctx, cancel := context.WithCancel(testutil.Context(t))
		calls := 0
		_, _, err := manager.bindWithFallback(ctx, newSequence(&recordingEventLedger{}), primary, routes,
			func(ctx context.Context, _ int, _ FallbackRoute) (int, error) {
				calls++
				cancel()
				return 0, ctx.Err()
			})
		if calls != 1 || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled sequence calls=%d err=%v, want one attempt and context.Canceled", calls, err)
		}
	})
}

func TestSessionFallbackEventPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should fingerprint the inherited effective command when the route has none", func(t *testing.T) {
		t.Parallel()
		seq := fallbackSequence{
			agent: "reviewer", phase: fallbackPhaseCreate,
			effectiveCommand: func(FallbackRoute) string { return "claude --acp" },
		}
		payload := newSessionFallbackEventPayload(seq, 2, FallbackRoute{Provider: "claude", Model: "opus-4-8"})
		want := sessionFallbackEventPayload{
			Agent: "reviewer", Phase: fallbackPhaseCreate, Attempt: 2, Provider: "claude", Model: "opus-4-8",
			ProviderCommandFingerprint: compozyconfig.CommandFingerprint("claude --acp"),
		}
		if payload != want {
			t.Fatalf("payload = %#v, want %#v", payload, want)
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if strings.Contains(string(encoded), "claude --acp") || strings.Contains(string(encoded), `"command"`) {
			t.Fatalf("payload JSON %s leaks the command", encoded)
		}
	})
}

func TestDecorateRefusedAttempt(t *testing.T) {
	t.Parallel()

	next := &FallbackRoute{Provider: "claude", Model: "opus-4-8"}
	failureFor := func(t *testing.T, err error) *store.SessionFailure {
		t.Helper()
		failure := sessionFailureFromError(startupFailure("agent runtime startup failed", err), store.FailureStartup)
		if failure == nil {
			t.Fatal("sessionFailureFromError() = nil")
		}
		return failure
	}
	actionOf := func(t *testing.T, failure *store.SessionFailure) acp.ProviderFailureDiagnostic {
		t.Helper()
		diagnostic, ok := acp.ProviderFailureDiagnosticFromSummary(failure.Summary)
		if !ok {
			t.Fatalf("summary %q has no provider diagnostic", failure.Summary)
		}
		return diagnostic
	}

	cases := []struct {
		name       string
		cause      error
		next       *FallbackRoute
		wantAction acp.ProviderFailureAction
		wantHint   string
	}{
		{
			name:  "Should prescribe use_fallback for a rate limit with a route remaining",
			cause: errors.New("HTTP 429 rate limit exceeded"), next: next,
			wantAction: acp.ProviderFailureActionUseFallback,
			wantHint:   "CompozyOS is trying the next configured route (claude/opus-4-8)",
		},
		{
			name:  "Should prescribe use_fallback for a missing login with a route remaining",
			cause: errors.New("Not logged in · Please run /login"), next: next,
			wantAction: acp.ProviderFailureActionUseFallback,
		},
		{
			name:  "Should keep retry for a rate limit on the last route",
			cause: errors.New("HTTP 429 rate limit exceeded"), next: nil,
			wantAction: acp.ProviderFailureActionRetry,
		},
		{
			name:  "Should keep install_cli for a missing CLI with a route remaining",
			cause: &exec.Error{Name: "claude", Err: exec.ErrNotFound}, next: next,
			wantAction: acp.ProviderFailureActionInstallCLI,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			failure := failureFor(t, tc.cause)
			decorated := decorateRefusedAttempt(failure, tc.next)
			diagnostic := actionOf(t, decorated)
			if diagnostic.Action != tc.wantAction {
				t.Fatalf("action = %q, want %q (summary %q)", diagnostic.Action, tc.wantAction, decorated.Summary)
			}
			if tc.wantHint != "" && diagnostic.Guidance != tc.wantHint {
				t.Fatalf("guidance = %q, want %q", diagnostic.Guidance, tc.wantHint)
			}
			if decorated == failure {
				t.Fatal("decorateRefusedAttempt() returned the input pointer, want a copy")
			}
		})
	}
}

const (
	fallbackSeatZero = "SEAT=0 claude --acp"
	fallbackSeatOne  = "SEAT=1 claude --acp"
)

// installFallbackAgent registers agent "reviewer" (claude on seat zero) with chain and
// wires a recording ledger into a fresh manager for the harness.
func installFallbackAgent(t *testing.T, h *harness, chain ...compozyconfig.RoleFallback) *recordingEventLedger {
	t.Helper()
	workspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	workspace.Agents = append(workspace.Agents, compozyconfig.AgentDef{
		Name: "reviewer", Provider: "claude", Command: fallbackSeatZero, Prompt: "Review work.",
		FallbackChain: chain,
	})
	h.resolver.upsert(&workspace)
	ledger := &recordingEventLedger{}
	h.manager.eventLedger = ledger
	return ledger
}

func claudeSeatOneRoute(h *harness) compozyconfig.RoleFallback {
	return compozyconfig.RoleFallback{
		Provider: "claude", Model: h.cfg.Providers["claude"].Models.Default, Command: fallbackSeatOne,
	}
}

func codexRoute(h *harness) compozyconfig.RoleFallback {
	return compozyconfig.RoleFallback{Provider: "codex", Model: h.cfg.Providers["codex"].Models.Default}
}

// refuseStartCommands makes the fake driver refuse session/new for the given commands
// (before acceptance) and accept every other command.
func refuseStartCommands(h *harness, refusals map[string]error) {
	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		if err, refused := refusals[opts.Command]; refused {
			return nil, err
		}
		sessionID := fmt.Sprintf("acp-%d", sequence)
		if opts.ResumeSessionID != "" {
			sessionID = opts.ResumeSessionID
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, sessionID), nil
	}
}

func startCommands(h *harness) []string {
	h.driver.mu.Lock()
	defer h.driver.mu.Unlock()
	commands := make([]string, 0, len(h.driver.startCalls))
	for _, call := range h.driver.startCalls {
		commands = append(commands, call.Command)
	}
	return commands
}

func transcriptMarkersOfKind(t *testing.T, manager *Manager, sessionID string, kind string) []transcript.Marker {
	t.Helper()
	eventsList, err := manager.Events(
		testutil.Context(t),
		sessionID,
		store.EventQuery{Type: eventspkg.TranscriptMarkerCreated},
	)
	if err != nil {
		t.Fatalf("Events(transcript marker) error = %v", err)
	}
	markers := make([]transcript.Marker, 0, len(eventsList))
	for _, event := range eventsList {
		agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			t.Fatalf("transcript.UnmarshalAgentEvent(%s) error = %v", event.ID, err)
		}
		if marker, ok := transcript.ParseMarker(agentEvent.Raw); ok && marker.Kind == kind {
			markers = append(markers, marker.Normalize())
		}
	}
	return markers
}

func rateLimitRefusal() error { return errors.New("HTTP 429 rate limit exceeded") }

func missingCLIRefusal() error { return &exec.Error{Name: "claude", Err: exec.ErrNotFound} }
