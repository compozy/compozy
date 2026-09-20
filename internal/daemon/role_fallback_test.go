package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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
		result, err := invokeRoleWithFallback(t.Context(), role, roleInvocationCorrelation{}, func(
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
		_, err := invokeRoleWithFallback(t.Context(), role, roleInvocationCorrelation{}, func(
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
		_, err := invokeRoleWithFallback(t.Context(), fallbackTestRole(nil), roleInvocationCorrelation{}, func(
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
		value, err := invokeRoleWithFallback(t.Context(), fallbackTestRole(nil), roleInvocationCorrelation{}, func(
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
		_, err := invokeRoleWithFallback(t.Context(), role, roleInvocationCorrelation{}, func(
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
		_, err := invokeRoleWithFallback(t.Context(), role, roleInvocationCorrelation{}, func(
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
		_, err := invokeRoleWithFallback(t.Context(), role, correlation, func(
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

func fallbackTestRole(writer roleEventSummaryWriter) ResolvedRole {
	return ResolvedRole{
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

func TestProviderRefusedTurnError(t *testing.T) {
	t.Parallel()

	agentErr := errors.New("daemon: automatic title agent error")
	for _, testCase := range []struct {
		name    string
		event   acp.AgentEvent
		err     error
		refused bool
	}{
		{name: "rate limited", event: providerErrorEvent(acp.ProviderErrorRateLimited), err: agentErr, refused: true},
		{name: "auth required", event: providerErrorEvent(acp.ProviderErrorAuthRequired), err: agentErr, refused: true},
		{name: "unclassified", event: acp.AgentEvent{Type: acp.EventTypeError}, err: agentErr},
		{name: "no error", event: providerErrorEvent(acp.ProviderErrorRateLimited)},
	} {
		got := providerRefusedTurnError(testCase.event, testCase.err)
		if errors.Is(got, errProviderRefusedTurn) != testCase.refused {
			t.Fatalf("providerRefusedTurnError(%s) = %v, want refused=%v", testCase.name, got, testCase.refused)
		}
	}
}

func providerErrorEvent(code string) acp.AgentEvent {
	return acp.AgentEvent{Type: acp.EventTypeError, ProviderError: &acp.ProviderErrorDiagnostic{Code: code}}
}

// TestRoleFallbackAdvancesOnProviderRefusal guards the wiring: the session is
// accepted, the provider then refuses the turn, and the remaining route runs it.
func TestRoleFallbackAdvancesOnProviderRefusal(t *testing.T) {
	t.Parallel()

	role := fallbackTestRole(nil)
	role.Role = compozyconfig.RoleAutoTitle
	role.Enabled = true
	sessions := &autoTitleSpawnSessionsStub{refuseProvider: "primary"}
	generator := newForkedAutoTitleGenerator(sessions, roleResolverFunc(
		func(context.Context, string, compozyconfig.RoleName) (ResolvedRole, error) {
			return role, nil
		},
	), 0, nil)
	title, err := generator.Generate(t.Context(), autoTitleRequest{
		SessionID:      "sess-parent",
		UserMessage:    "why did checkout retry twice",
		AssistantReply: "a race in the retry guard",
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if title != "Checkout retry race" {
		t.Fatalf("Generate() = %q, want the title produced by the second route", title)
	}
	if want := []string{"primary", "secondary"}; !reflect.DeepEqual(sessions.providers, want) {
		t.Fatalf("attempted providers = %#v, want %#v", sessions.providers, want)
	}
	if len(sessions.stops) != 2 || sessions.stops[0].cause != session.CauseFailed {
		t.Fatalf("stops = %#v, want the refused child stopped with CauseFailed first", sessions.stops)
	}
}
