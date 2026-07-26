package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/subprocess"
	taskpkg "github.com/compozy/agh/internal/task"
)

const maxSubprocessHealthEscalationDiagnosticBytes = 1024

type subprocessHealthRunSource interface {
	ListTaskRuns(context.Context, taskpkg.RunQuery) ([]taskpkg.Run, error)
}

type subprocessHealthEscalationActor interface {
	MarkRunNeedsAttention(context.Context, string, string) (taskpkg.Run, error)
}

type subprocessHealthEscalator struct {
	runs      subprocessHealthRunSource
	actor     subprocessHealthEscalationActor
	threshold int
	logger    *slog.Logger
}

var _ session.SubprocessHealthNotifier = (*subprocessHealthEscalator)(nil)
var _ subprocessHealthRuntimeNotifier = (*subprocessHealthEscalator)(nil)

func newSubprocessHealthEscalator(
	runs subprocessHealthRunSource,
	actor subprocessHealthEscalationActor,
	threshold int,
	logger *slog.Logger,
) (*subprocessHealthEscalator, error) {
	if runs == nil {
		return nil, errors.New("daemon: subprocess health run source is required")
	}
	if actor == nil {
		return nil, errors.New("daemon: subprocess health escalation actor is required")
	}
	if threshold < 0 {
		return nil, fmt.Errorf("daemon: subprocess health escalation threshold must be zero or positive: %d", threshold)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &subprocessHealthEscalator{
		runs:      runs,
		actor:     actor,
		threshold: threshold,
		logger:    logger,
	}, nil
}

func (*subprocessHealthEscalator) OnSessionCreated(context.Context, *session.Session) {}

func (e *subprocessHealthEscalator) OnSessionStopped(ctx context.Context, sess *session.Session) {
	if e == nil || e.threshold == 0 || sess == nil {
		return
	}
	info := sess.Info()
	if info == nil || info.StopReason != store.StopAgentCrashed {
		return
	}
	e.escalate(ctx, session.SubprocessHealthSnapshot{
		SessionID:   info.ID,
		WorkspaceID: info.WorkspaceID,
		AgentName:   info.AgentName,
	}, "agent subprocess crashed")
}

func (e *subprocessHealthEscalator) OnSubprocessHealth(
	ctx context.Context,
	observation session.SubprocessHealthSnapshot,
) {
	if e == nil || e.threshold == 0 ||
		observation.Health.ConsecutiveFailures < e.threshold ||
		!subprocess.HealthFailureDetected(observation.Health) {
		return
	}
	reason := subprocess.HealthFailureReason(observation.Health)
	if strings.TrimSpace(reason) == "" {
		reason = "subprocess reported an unhealthy verdict"
	}
	e.escalate(ctx, observation, reason)
}

func (e *subprocessHealthEscalator) escalate(
	ctx context.Context,
	observation session.SubprocessHealthSnapshot,
	reason string,
) {
	sessionID := strings.TrimSpace(observation.SessionID)
	if sessionID == "" {
		return
	}
	runs, err := e.runs.ListTaskRuns(ctx, taskpkg.RunQuery{SessionID: sessionID})
	if err != nil {
		e.logFailure("list task runs", observation, taskpkg.Run{}, err)
		return
	}
	candidates := activeTaskRunsForSession(runs, sessionID)
	if len(candidates) == 0 {
		return
	}
	if len(candidates) != 1 {
		e.logFailure(
			"resolve task run",
			observation,
			taskpkg.Run{},
			fmt.Errorf("daemon: session %q has %d nonterminal task runs", sessionID, len(candidates)),
		)
		return
	}

	run := candidates[0]
	diagnostic := diagnostics.RedactAndBound(
		"subprocess health degraded: "+strings.TrimSpace(reason),
		maxSubprocessHealthEscalationDiagnosticBytes,
	)
	updated, err := e.actor.MarkRunNeedsAttention(ctx, run.ID, diagnostic)
	if errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
		return
	}
	if err != nil {
		e.logFailure("mark task run needs attention", observation, run, err)
		return
	}
	e.logger.Warn(
		"daemon: subprocess health escalated task run",
		"workspace_id", strings.TrimSpace(observation.WorkspaceID),
		"session_id", sessionID,
		"agent_name", strings.TrimSpace(observation.AgentName),
		"task_id", updated.TaskID,
		"run_id", updated.ID,
		"status", updated.Status.String(),
	)
}

func activeTaskRunsForSession(runs []taskpkg.Run, sessionID string) []taskpkg.Run {
	active := make([]taskpkg.Run, 0, 1)
	for _, run := range runs {
		if strings.TrimSpace(run.SessionID) != sessionID {
			continue
		}
		switch run.Status.Normalize() {
		case taskpkg.TaskRunStatusQueued,
			taskpkg.TaskRunStatusClaimed,
			taskpkg.TaskRunStatusStarting,
			taskpkg.TaskRunStatusRunning:
			active = append(active, run)
		}
	}
	return active
}

func (e *subprocessHealthEscalator) logFailure(
	operation string,
	observation session.SubprocessHealthSnapshot,
	run taskpkg.Run,
	err error,
) {
	e.logger.Error(
		"daemon: subprocess health escalation failed",
		"operation", operation,
		"workspace_id", strings.TrimSpace(observation.WorkspaceID),
		"session_id", strings.TrimSpace(observation.SessionID),
		"agent_name", strings.TrimSpace(observation.AgentName),
		"task_id", strings.TrimSpace(run.TaskID),
		"run_id", strings.TrimSpace(run.ID),
		"error", err,
	)
}
