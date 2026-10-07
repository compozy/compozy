package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func TestInvokeRoleWithFallback(t *testing.T) {
	t.Parallel()

	t.Run("Should stop after the first accepted fallback", func(t *testing.T) {
		t.Parallel()

		role := fallbackTestRole(nil)
		var attempts []string
		result, err := role.invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			_ context.Context,
			route roleAttemptRoute,
		) (string, bool, error) {
			attempts = append(attempts, route.Provider+"/"+route.Model)
			if route.Provider == "secondary" {
				return "accepted", true, nil
			}
			return "", false, errors.New("rejected before acceptance")
		})
		if err != nil {
			t.Fatalf("invokeRoleWithFallback() error = %v", err)
		}
		if result != "accepted" {
			t.Fatalf("invokeRoleWithFallback() = %q, want accepted", result)
		}
		if want := []string{"primary/m1", "secondary/m2"}; !reflect.DeepEqual(attempts, want) {
			t.Fatalf("attempts = %#v, want %#v", attempts, want)
		}
	})

	t.Run("Should try every route once in declared order without overlap", func(t *testing.T) {
		t.Parallel()

		role := fallbackTestRole(nil)
		routeErr := errors.New("route rejected before acceptance")
		var active atomic.Int32
		var maximum atomic.Int32
		var attempts []string
		_, err := role.invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			_ context.Context,
			route roleAttemptRoute,
		) (struct{}, bool, error) {
			current := active.Add(1)
			defer active.Add(-1)
			if current > maximum.Load() {
				maximum.Store(current)
			}
			attempts = append(attempts, route.Provider)
			return struct{}{}, false, routeErr
		})
		if !errors.Is(err, routeErr) {
			t.Fatalf("invokeRoleWithFallback() error = %v, want route rejection", err)
		}
		if want := []string{"primary", "secondary", "tertiary"}; !reflect.DeepEqual(attempts, want) {
			t.Fatalf("attempts = %#v, want %#v", attempts, want)
		}
		if maximum.Load() != 1 {
			t.Fatalf("maximum concurrent attempts = %d, want 1", maximum.Load())
		}
	})

	t.Run("Should return deterministic exhaustion with the last cause", func(t *testing.T) {
		t.Parallel()

		lastCause := errors.New("tertiary unavailable")
		attempt := 0
		_, err := fallbackTestRole(nil).invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			_ context.Context,
			_ roleAttemptRoute,
		) (struct{}, bool, error) {
			attempt++
			if attempt == 3 {
				return struct{}{}, false, lastCause
			}
			return struct{}{}, false, errors.New("route unavailable")
		})
		if !errors.Is(err, lastCause) {
			t.Fatalf("invokeRoleWithFallback() error = %v, want last cause", err)
		}
		if attempt != 3 {
			t.Fatalf("attempt count = %d, want 3", attempt)
		}
	})

	t.Run("Should never fall back after authoritative acceptance", func(t *testing.T) {
		t.Parallel()

		acceptedErr := errors.New("accepted session failed during startup")
		attempts := 0
		value, err := fallbackTestRole(nil).invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			_ context.Context,
			_ roleAttemptRoute,
		) (string, bool, error) {
			attempts++
			return "accepted-session", true, acceptedErr
		})
		if !errors.Is(err, acceptedErr) {
			t.Fatalf("invokeRoleWithFallback() error = %v, want accepted failure", err)
		}
		if value != "accepted-session" || attempts != 1 {
			t.Fatalf("value/attempts = %q/%d, want accepted-session/1", value, attempts)
		}
	})

	t.Run("Should preserve speed and ACP options on every route", func(t *testing.T) {
		t.Parallel()

		role := fallbackTestRole(nil)
		role.setRuntime(
			speedpkg.SpeedFast,
			[]compozyconfig.ACPOptionSelection{{ID: "thinking", BoolValue: new(true)}},
		)
		role.Fallbacks[0].Speed = speedpkg.SpeedNormal
		role.Fallbacks[0].ACPOptions = []compozyconfig.ACPOptionSelection{{ID: "context", ValueID: "1m"}}
		role.Fallbacks[1].Speed = speedpkg.SpeedFast
		role.Fallbacks[1].ACPOptions = []compozyconfig.ACPOptionSelection{{ID: "thinking", BoolValue: new(false)}}
		var routes []roleAttemptRoute
		_, err := role.invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			_ context.Context,
			route roleAttemptRoute,
		) (struct{}, bool, error) {
			routes = append(routes, route)
			return struct{}{}, false, errors.New("rejected")
		})
		if err == nil || len(routes) != 3 {
			t.Fatalf("invokeRoleWithFallback() error/routes = %v/%d, want exhaustion/3", err, len(routes))
		}
		want := []roleAttemptRoute{
			{
				AgentName:       compozyconfig.BuiltinDreamingCuratorAgentName,
				Provider:        "primary",
				Model:           "m1",
				ReasoningEffort: "low",
				Speed:           speedpkg.SpeedFast,
				ACPOptions: []compozyconfig.ACPOptionSelection{
					{ID: "thinking", BoolValue: new(true)},
				},
			},
			{
				AgentName:       compozyconfig.BuiltinDreamingCuratorAgentName,
				Provider:        "secondary",
				Model:           "m2",
				ReasoningEffort: "medium",
				Speed:           speedpkg.SpeedNormal,
				ACPOptions: []compozyconfig.ACPOptionSelection{
					{ID: "context", ValueID: "1m"},
				},
			},
			{
				AgentName:       compozyconfig.BuiltinDreamingCuratorAgentName,
				Provider:        "tertiary",
				Model:           "m3",
				ReasoningEffort: "high",
				Speed:           speedpkg.SpeedFast,
				ACPOptions: []compozyconfig.ACPOptionSelection{
					{ID: "thinking", BoolValue: new(false)},
				},
			},
		}
		if !reflect.DeepEqual(routes, want) {
			t.Fatalf("routes = %#v, want %#v", routes, want)
		}
	})

	t.Run("Should make one attempt and emit no event for an empty chain", func(t *testing.T) {
		t.Parallel()

		recorder := &roleEventRecorder{}
		role := fallbackTestRole(recorder)
		role.Fallbacks = nil
		primaryErr := errors.New("primary rejected")
		attempts := 0
		_, err := role.invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			_ context.Context,
			_ roleAttemptRoute,
		) (struct{}, bool, error) {
			attempts++
			return struct{}{}, false, primaryErr
		})
		if !errors.Is(err, primaryErr) {
			t.Fatalf("invokeRoleWithFallback() error = %v, want primary rejection", err)
		}
		if attempts != 1 || recorder.count() != 0 {
			t.Fatalf("attempts/events = %d/%d, want 1/0", attempts, recorder.count())
		}
	})
}

func TestInvokeRoleWithFallbackRouteAccounts(t *testing.T) {
	t.Parallel()

	const seatTwo = "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"
	const seatThree = "CODEX_HOME=/Users/ada/.codex-work codex acp"
	accountRole := func(writer roleEventSummaryWriter) *ResolvedRole {
		role := fallbackTestRole(writer)
		role.Fallbacks[0].Command = seatTwo
		role.Fallbacks[1].Command = "  " + seatThree + "  "
		return role
	}

	t.Run("Should hand each attempt its route command verbatim", func(t *testing.T) { // UT-008
		t.Parallel()

		var commands []string
		_, err := accountRole(nil).invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			_ context.Context,
			route roleAttemptRoute,
		) (struct{}, bool, error) {
			commands = append(commands, route.Command)
			return struct{}{}, false, errors.New("refused before acceptance")
		})
		if err == nil {
			t.Fatal("invokeRoleWithFallback() error = nil, want exhaustion")
		}
		if want := []string{"", seatTwo, seatThree}; !reflect.DeepEqual(commands, want) {
			t.Fatalf("attempt commands = %#v, want %#v", commands, want)
		}
	})

	t.Run("Should fingerprint the route account and never write the command", func(t *testing.T) { // UT-009
		t.Parallel()

		recorder := &roleEventRecorder{}
		_, err := accountRole(recorder).invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			_ context.Context,
			route roleAttemptRoute,
		) (struct{}, bool, error) {
			if route.Command == seatTwo {
				return struct{}{}, true, nil
			}
			return struct{}{}, false, errors.New("refused before acceptance")
		})
		if err != nil {
			t.Fatalf("invokeRoleWithFallback() error = %v", err)
		}
		event := recorder.single(t)
		sum := sha256.Sum256([]byte(seatTwo))
		want := "sha256:" + hex.EncodeToString(sum[:])
		var payload map[string]any
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(event.Content) error = %v", err)
		}
		if payload["provider_command_fingerprint"] != want {
			t.Fatalf("provider_command_fingerprint = %v, want %q", payload["provider_command_fingerprint"], want)
		}
		if _, leaked := payload["command"]; leaked || strings.Contains(string(event.Content), "claude-work") ||
			strings.Contains(event.Summary, "claude-work") {
			t.Fatalf("fallback event leaked the raw command: %s / %q", event.Content, event.Summary)
		}
	})

	t.Run("Should not start an attempt whose event write failed", func(t *testing.T) { // UT-009
		t.Parallel()

		writeErr := errors.New("ledger unavailable")
		primaryErr := errors.New("primary refused")
		attempts := 0
		_, err := accountRole(failingRoleEventWriter{err: writeErr}).invokeRoleWithFallback(t.Context(),
			roleInvocationCorrelation{},
			func(context.Context, roleAttemptRoute) (struct{}, bool, error) {
				attempts++
				return struct{}{}, false, primaryErr
			},
		)
		if attempts != 1 || !errors.Is(err, writeErr) || !errors.Is(err, primaryErr) {
			t.Fatalf("attempts/error = %d/%v, want one attempt and joined write+primary errors", attempts, err)
		}
	})

	t.Run("Should stop on an attempt accepted by ACP that then failed", func(t *testing.T) { // UT-009
		t.Parallel()

		recorder := &roleEventRecorder{}
		acceptedErr := &acp.AcceptedStartError{SessionID: "acp_1", Cause: errors.New("configure failed")}
		attempts := 0
		value, err := accountRole(recorder).invokeRoleWithFallback(t.Context(), roleInvocationCorrelation{}, func(
			context.Context,
			roleAttemptRoute,
		) (*struct{}, bool, error) {
			attempts++
			var created *struct{}
			return created, session.StartAccepted(acceptedErr) || created != nil, acceptedErr
		})
		if attempts != 1 || value != nil || !errors.Is(err, acceptedErr) || recorder.count() != 0 {
			t.Fatalf(
				"attempts/value/error/events = %d/%v/%v/%d, want one accepted attempt and no fallback",
				attempts, value, err, recorder.count(),
			)
		}
	})
}

type failingRoleEventWriter struct {
	err error
}

func (w failingRoleEventWriter) WriteEventSummary(context.Context, store.EventSummary) error {
	return w.err
}

func TestRoleObservabilityCoverageMatrix(t *testing.T) {
	t.Parallel()

	t.Run("Should persist the fallback event before starting the attempt", func(t *testing.T) {
		t.Parallel()

		recorder := &roleEventRecorder{}
		role := fallbackTestRole(recorder)
		correlation := roleInvocationCorrelation{
			WorkspaceID: "ws-1",
			SessionID:   "session-1",
			Event:       store.EventCorrelation{TaskID: "task-1", RunID: "run-1"},
		}
		attempts := 0
		_, err := role.invokeRoleWithFallback(t.Context(), correlation, func(
			_ context.Context,
			_ roleAttemptRoute,
		) (struct{}, bool, error) {
			attempts++
			if attempts == 1 {
				return struct{}{}, false, errors.New("primary rejected")
			}
			if recorder.count() != 1 {
				t.Fatalf("event count at fallback start = %d, want 1", recorder.count())
			}
			return struct{}{}, true, nil
		})
		if err != nil {
			t.Fatalf("invokeRoleWithFallback() error = %v", err)
		}
		event := recorder.single(t)
		if event.Type != eventspkg.RoleFallbackUsed || event.WorkspaceID != "ws-1" ||
			event.SessionID != "session-1" || event.TaskID != "task-1" || event.RunID != "run-1" ||
			event.Provider != "" {
			t.Fatalf("fallback event = %#v", event)
		}
		var payload roleFallbackEventPayload
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(event.Content) error = %v", err)
		}
		if payload.Role != string(compozyconfig.RoleDream) || payload.Attempt != 1 ||
			payload.Provider != "secondary" || payload.Model != "m2" {
			t.Fatalf("fallback payload = %#v", payload)
		}
	})

	t.Run("Should emit the canonical error event with role correlation", func(t *testing.T) {
		t.Parallel()

		cfg := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
		cfg.Memory.Enabled = true
		cfg.Roles.Dream.Enabled = true
		cfg.Roles.Dream.Agent = "missing-curator"
		recorder := &roleEventRecorder{}
		resolver := newRoleResolver(&cfg, nil, nil, recorder)
		correlation := roleInvocationCorrelation{
			WorkspaceID: "ws-1",
			SessionID:   "session-1",
			Event: store.EventCorrelation{
				TaskID: "task-1",
				RunID:  "run-1",
			},
		}
		_, err := resolver.Resolve(
			withRoleInvocationCorrelation(t.Context(), correlation),
			"",
			compozyconfig.RoleDream,
		)
		resolutionErr, resolutionErrMatched := errors.AsType[*RoleResolutionError](err)
		if !resolutionErrMatched || resolutionErr.Code != roleErrorAgentNotFound {
			t.Fatalf("Resolve() error = %v, want role_agent_not_found", err)
		}
		event := recorder.single(t)
		if event.Type != eventspkg.RoleResolveError || event.Outcome != string(eventspkg.OutcomeFailure) ||
			event.SessionID != "session-1" || event.WorkspaceID != "ws-1" ||
			event.TaskID != "task-1" || event.RunID != "run-1" {
			t.Fatalf("resolution event = %#v", event)
		}
		var payload roleResolveErrorEventPayload
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(event.Content) error = %v", err)
		}
		if payload.Role != string(compozyconfig.RoleDream) ||
			payload.ErrorCode != roleErrorAgentNotFound || payload.Agent != "missing-curator" {
			t.Fatalf("resolution payload = %#v", payload)
		}
	})
}

func fallbackTestRole(writer roleEventSummaryWriter) *ResolvedRole {
	return &ResolvedRole{
		Role:            compozyconfig.RoleDream,
		AgentName:       compozyconfig.BuiltinDreamingCuratorAgentName,
		Provider:        "primary",
		Model:           "m1",
		ReasoningEffort: "low",
		Fallbacks: []compozyconfig.RoleFallback{
			{Provider: "secondary", Model: "m2", ReasoningEffort: "medium"},
			{Provider: "tertiary", Model: "m3", ReasoningEffort: "high"},
		},
		eventWriter: writer,
	}
}

type roleEventRecorder struct {
	mu     sync.Mutex
	events []store.EventSummary
}

func (r *roleEventRecorder) WriteEventSummary(_ context.Context, event store.EventSummary) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *roleEventRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

func (r *roleEventRecorder) single(t *testing.T) store.EventSummary {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) != 1 {
		t.Fatalf("event count = %d, want 1", len(r.events))
	}
	return r.events[0]
}
