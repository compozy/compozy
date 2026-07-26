package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/session"
)

const checkpointSummaryStopTimeout = 10 * time.Second

type checkpointSummarySessionManager interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error)
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
		WorkspaceID: strings.TrimSpace(request.WorkspaceID),
		SessionID:   strings.TrimSpace(request.SessionID),
		AgentName:   strings.TrimSpace(request.AgentName),
	}
	roleCtx := withRoleInvocationCorrelation(ctx, correlation)
	role, err := s.roles.Resolve(roleCtx, request.WorkspaceID, aghconfig.RoleCheckpointSummary)
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
	summarySession, err := invokeRoleWithFallback(ctx, role, correlation, func(
		attemptCtx context.Context,
		route roleAttemptRoute,
	) (*session.Session, bool, error) {
		created, createErr := s.sessions.Create(attemptCtx, session.CreateOpts{
			AgentName:       route.AgentName,
			Provider:        route.Provider,
			Model:           route.Model,
			ReasoningEffort: route.ReasoningEffort,
			Name:            checkpointSummarySessionName,
			Workspace:       strings.TrimSpace(request.WorkspaceRoot),
			Type:            session.SessionTypeDream,
		})
		return created, created != nil, createErr
	})
	if summarySession != nil {
		defer s.stopCheckpointSummarySession(ctx, summarySession.ID, &err)
	}
	if err != nil {
		return "", fmt.Errorf("daemon: create checkpoint summary session: %w", err)
	}

	events, err := s.sessions.Prompt(ctx, summarySession.ID, prompt)
	if err != nil {
		return "", fmt.Errorf("daemon: prompt checkpoint summary session %q: %w", summarySession.ID, err)
	}
	output, err := collectCheckpointSummaryOutput(ctx, events)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
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
				return "", fmt.Errorf("daemon: checkpoint summary agent error: %s", strings.TrimSpace(event.Error))
			}
		}
	}
}
