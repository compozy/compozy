package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

const checkpointSummaryStopTimeout = 10 * time.Second

type checkpointSummarySessionManager interface {
	CreateLifecycleContinuation(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	PromptLifecycleContinuation(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error)
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

type daemonCheckpointSummarizer struct {
	sessions checkpointSummarySessionManager
	roles    RoleResolver
}

var _ memory.CheckpointSummarizer = (*daemonCheckpointSummarizer)(nil)

func newDaemonCheckpointSummarizer(
	sessions checkpointSummarySessionManager,
	roles RoleResolver,
) *daemonCheckpointSummarizer {
	return &daemonCheckpointSummarizer{
		sessions: sessions,
		roles:    roles,
	}
}

func (s *daemonCheckpointSummarizer) Summarize(
	ctx context.Context,
	request memory.CheckpointSummaryRequest,
) (summary string, err error) {
	if s == nil || s.sessions == nil {
		return "", errors.New("daemon: checkpoint summary sessions are not configured")
	}
	if ctx == nil {
		return "", errors.New("daemon: checkpoint summary context is required")
	}
	if s.roles == nil {
		return "", errors.New("daemon: checkpoint summary role resolver is not configured")
	}
	correlation := roleInvocationCorrelation{
		SessionCompaction: request.Compaction,
		WorkspaceID:       strings.TrimSpace(request.WorkspaceID),
		SessionID:         strings.TrimSpace(request.SessionID),
		AgentName:         strings.TrimSpace(request.AgentName),
	}
	roleCtx := withRoleInvocationCorrelation(ctx, correlation)
	role, err := s.roles.Resolve(roleCtx, request.WorkspaceID, compozyconfig.RoleCheckpointSummary)
	if err != nil {
		return "", fmt.Errorf("daemon: resolve checkpoint summary role: %w", err)
	}
	if !role.Enabled {
		return "", memory.ErrCheckpointSummaryDisabled
	}
	prompt, err := memory.RenderCheckpointSummaryPrompt(request)
	if err != nil {
		return "", err
	}
	var summarySession *session.Session
	output, err := invokeRoleWithFallback(ctx, role, correlation, func(
		attemptCtx context.Context,
		route roleAttemptRoute,
	) (string, bool, error) {
		created, createErr := s.sessions.CreateLifecycleContinuation(attemptCtx, session.CreateOpts{
			AgentName:           route.AgentName,
			Provider:            route.Provider,
			Model:               route.Model,
			ReasoningEffort:     route.ReasoningEffort,
			Speed:               route.Speed,
			ACPOptions:          session.ACPOptionSelectionsFromConfig(route.ACPOptions),
			Name:                checkpointSummarySessionName,
			Workspace:           strings.TrimSpace(request.WorkspaceRoot),
			Type:                session.SessionTypeDream,
			Lineage:             &store.SessionLineage{SpawnRole: session.SpawnRoleCheckpointSummary},
			DiscardStartFailure: true,
		})
		if createErr != nil {
			summarySession = created
			return "", created != nil, fmt.Errorf("daemon: create checkpoint summary session: %w", createErr)
		}
		if created == nil {
			return "", false, errors.New("daemon: checkpoint summary create returned no session")
		}
		collected, attemptErr := s.runCheckpointSummaryTurn(attemptCtx, created.ID, prompt)
		if errors.Is(attemptErr, errProviderRefusedTurn) {
			return "", false, errors.Join(attemptErr, s.discardCheckpointSummaryAttempt(ctx, created.ID))
		}
		summarySession = created
		return collected, true, attemptErr
	})
	if summarySession != nil {
		defer s.stopCheckpointSummarySession(ctx, summarySession.ID, &err)
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// runCheckpointSummaryTurn prompts one created attempt and collects its output.
// A provider refusal is reported through errProviderRefusedTurn so the caller
// can advance to the next fallback route.
func (s *daemonCheckpointSummarizer) runCheckpointSummaryTurn(
	ctx context.Context,
	sessionID string,
	prompt string,
) (string, error) {
	events, err := s.sessions.PromptLifecycleContinuation(ctx, sessionID, prompt)
	if err != nil {
		return "", fmt.Errorf("daemon: prompt checkpoint summary session %q: %w", sessionID, err)
	}
	return collectCheckpointSummaryOutput(ctx, events)
}

// discardCheckpointSummaryAttempt stops the session of a refused route so the
// next attempt does not leave it running.
func (s *daemonCheckpointSummarizer) discardCheckpointSummaryAttempt(
	ctx context.Context,
	sessionID string,
) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), checkpointSummaryStopTimeout)
	defer cancel()
	if err := s.sessions.StopWithCause(
		stopCtx, sessionID, session.CauseFailed, "checkpoint summary route refused by provider",
	); err != nil {
		return fmt.Errorf("daemon: stop refused checkpoint summary session: %w", err)
	}
	return nil
}

func (s *daemonCheckpointSummarizer) stopCheckpointSummarySession(
	ctx context.Context,
	sessionID string,
	operationErr *error,
) {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), checkpointSummaryStopTimeout)
	defer cancel()
	cause := session.CauseCompleted
	detail := "checkpoint summary completed"
	if *operationErr != nil {
		cause = session.CauseFailed
		detail = (*operationErr).Error()
	}
	if err := s.sessions.StopWithCause(stopCtx, sessionID, cause, detail); err != nil {
		*operationErr = errors.Join(*operationErr, fmt.Errorf(
			"daemon: stop checkpoint summary session %q: %w",
			sessionID,
			err,
		))
	}
}

func collectCheckpointSummaryOutput(ctx context.Context, events <-chan acp.AgentEvent) (string, error) {
	var output strings.Builder
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("daemon: collect checkpoint summary output: %w", ctx.Err())
		case event, ok := <-events:
			if !ok {
				return output.String(), nil
			}
			switch event.Type {
			case acp.EventTypeAgentMessage:
				output.WriteString(event.Text)
			case acp.EventTypeError:
				return "", providerRefusedTurnError(event, fmt.Errorf(
					"daemon: checkpoint summary agent error: %s", strings.TrimSpace(event.Error),
				))
			}
		}
	}
}
