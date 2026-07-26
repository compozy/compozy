package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
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
			event.SessionID != "session-1" || event.TaskID != "task-1" || event.RunID != "run-1" {
			t.Fatalf("fallback event = %#v", event)
		}
		var payload roleFallbackEventPayload
		if err := json.Unmarshal(event.Content, &payload); err != nil {
			t.Fatalf("json.Unmarshal(event.Content) error = %v", err)
		}
		if payload.Role != string(aghconfig.RoleDream) || payload.Attempt != 1 ||
			payload.Provider != "secondary" || payload.Model != "m2" {
			t.Fatalf("fallback payload = %#v", payload)
		}
	})

	t.Run("Should emit the canonical error event with role correlation", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DefaultWithHome(aghconfig.HomePaths{})
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
			aghconfig.RoleDream,
		)
		var resolutionErr *RoleResolutionError
		if !errors.As(err, &resolutionErr) || resolutionErr.Code != roleErrorAgentNotFound {
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
		if payload.Role != string(aghconfig.RoleDream) ||
			payload.ErrorCode != roleErrorAgentNotFound || payload.Agent != "missing-curator" {
			t.Fatalf("resolution payload = %#v", payload)
		}
	})
}

func fallbackTestRole(writer roleEventSummaryWriter) ResolvedRole {
	return ResolvedRole{
		Role:            aghconfig.RoleDream,
		AgentName:       aghconfig.BuiltinDreamingCuratorAgentName,
		Provider:        "primary",
		Model:           "m1",
		ReasoningEffort: "low",
		Fallbacks: []aghconfig.RoleFallback{
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
