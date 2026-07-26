package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
)

const (
	roleResolutionFailedCode = contract.CodeRoleResolutionFailed
	roleEventWriteTimeout    = 5 * time.Second
)

type roleEventSummaryWriter interface {
	WriteEventSummary(context.Context, store.EventSummary) error
}

type roleInvocationCorrelation struct {
	WorkspaceID     string
	SessionID       string
	AgentName       string
	ParentSessionID string
	RootSessionID   string
	SpawnDepth      int
	Event           store.EventCorrelation
}

type roleInvocationCorrelationContextKey struct{}

type roleAttemptRoute struct {
	AgentName       string
	Provider        string
	Model           string
	ReasoningEffort string
}

type roleFallbackEventPayload struct {
	Role     string `json:"role"`
	Attempt  int    `json:"attempt"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type roleResolveErrorEventPayload struct {
	Role      string `json:"role"`
	ErrorCode string `json:"error_code"`
	Agent     string `json:"agent,omitempty"`
}

func withRoleInvocationCorrelation(ctx context.Context, correlation roleInvocationCorrelation) context.Context {
	return context.WithValue(ctx, roleInvocationCorrelationContextKey{}, correlation)
}

func roleInvocationCorrelationFromContext(ctx context.Context, workspaceID string) roleInvocationCorrelation {
	correlation := roleInvocationCorrelation{WorkspaceID: strings.TrimSpace(workspaceID)}
	if ctx == nil {
		return correlation
	}
	stored, ok := ctx.Value(roleInvocationCorrelationContextKey{}).(roleInvocationCorrelation)
	if !ok {
		return correlation
	}
	stored.WorkspaceID = firstRoleValue(stored.WorkspaceID, workspaceID)
	return stored
}

func invokeRoleWithFallback[T any](
	ctx context.Context,
	role ResolvedRole,
	correlation roleInvocationCorrelation,
	invoke func(context.Context, roleAttemptRoute) (T, bool, error),
) (T, error) {
	var zero T
	if invoke == nil {
		return zero, errors.New("daemon: role invocation callback is required")
	}
	primary := roleAttemptRoute{
		AgentName:       strings.TrimSpace(role.AgentName),
		Provider:        strings.TrimSpace(role.Provider),
		Model:           strings.TrimSpace(role.Model),
		ReasoningEffort: strings.TrimSpace(role.ReasoningEffort),
	}
	value, accepted, err := invoke(ctx, primary)
	if accepted {
		return value, err
	}
	attemptErrors := []error{roleAttemptError(role.Role, 0, err)}
	for index, fallback := range role.Fallbacks {
		route := roleAttemptRoute{
			AgentName:       primary.AgentName,
			Provider:        strings.TrimSpace(fallback.Provider),
			Model:           strings.TrimSpace(fallback.Model),
			ReasoningEffort: strings.TrimSpace(fallback.ReasoningEffort),
		}
		attempt := index + 1
		if eventErr := recordRoleFallbackEvent(ctx, role, correlation, attempt, route); eventErr != nil {
			return zero, errors.Join(append(attemptErrors, eventErr)...)
		}
		value, accepted, err = invoke(ctx, route)
		if accepted {
			return value, err
		}
		attemptErrors = append(attemptErrors, roleAttemptError(role.Role, attempt, err))
	}
	return zero, fmt.Errorf(
		"daemon: %s role invocation exhausted %d attempt(s): %w",
		role.Role,
		len(role.Fallbacks)+1,
		errors.Join(attemptErrors...),
	)
}

func roleAttemptError(role aghconfig.RoleName, attempt int, err error) error {
	if err == nil {
		err = errors.New("invocation returned without acceptance")
	}
	return fmt.Errorf("%s role attempt %d failed before acceptance: %w", role, attempt, err)
}

func recordRoleFallbackEvent(
	ctx context.Context,
	role ResolvedRole,
	correlation roleInvocationCorrelation,
	attempt int,
	route roleAttemptRoute,
) error {
	if role.eventWriter == nil {
		return nil
	}
	content, err := json.Marshal(roleFallbackEventPayload{
		Role: string(role.Role), Attempt: attempt, Provider: route.Provider, Model: route.Model,
	})
	if err != nil {
		return fmt.Errorf("daemon: marshal role fallback event: %w", err)
	}
	eventCtx, cancel := detachedDaemonOperationContext(ctx, roleEventWriteTimeout)
	defer cancel()
	if err := role.eventWriter.WriteEventSummary(eventCtx, store.EventSummary{
		SessionID: correlation.SessionID, WorkspaceID: correlation.WorkspaceID,
		Type: eventspkg.RoleFallbackUsed, AgentName: firstRoleValue(correlation.AgentName, route.AgentName),
		Provider: route.Provider, Outcome: string(eventspkg.OutcomeFor(eventspkg.RoleFallbackUsed)), Content: content,
		EventCorrelation: correlation.Event, ParentSessionID: correlation.ParentSessionID,
		RootSessionID: correlation.RootSessionID, SpawnDepth: correlation.SpawnDepth,
		Summary: fmt.Sprintf("%s role fallback attempt %d", role.Role, attempt),
	}); err != nil {
		return fmt.Errorf("daemon: record %s: %w", eventspkg.RoleFallbackUsed, err)
	}
	return nil
}

func (r *roleResolver) recordRoleResolveError(
	ctx context.Context,
	workspaceID string,
	role aghconfig.RoleName,
	resolveErr error,
) error {
	if r == nil || r.events == nil || resolveErr == nil {
		return nil
	}
	correlation := roleInvocationCorrelationFromContext(ctx, workspaceID)
	errorCode := roleResolutionFailedCode
	agentName := ""
	var resolutionErr *RoleResolutionError
	if errors.As(resolveErr, &resolutionErr) {
		errorCode = firstRoleValue(resolutionErr.Code, errorCode)
		agentName = strings.TrimSpace(resolutionErr.Agent)
	}
	content, err := json.Marshal(roleResolveErrorEventPayload{
		Role: string(role), ErrorCode: errorCode, Agent: agentName,
	})
	if err != nil {
		return fmt.Errorf("daemon: marshal role resolution event: %w", err)
	}
	eventCtx, cancel := detachedDaemonOperationContext(ctx, roleEventWriteTimeout)
	defer cancel()
	if err := r.events.WriteEventSummary(eventCtx, store.EventSummary{
		SessionID: correlation.SessionID, WorkspaceID: correlation.WorkspaceID,
		Type: eventspkg.RoleResolveError, AgentName: firstRoleValue(correlation.AgentName, agentName),
		Outcome: string(eventspkg.OutcomeFor(eventspkg.RoleResolveError)), Content: content,
		EventCorrelation: correlation.Event, ParentSessionID: correlation.ParentSessionID,
		RootSessionID: correlation.RootSessionID, SpawnDepth: correlation.SpawnDepth,
		Summary: fmt.Sprintf("%s role resolution failed: %s", role, errorCode),
	}); err != nil {
		return fmt.Errorf("daemon: record %s: %w", eventspkg.RoleResolveError, err)
	}
	return nil
}
