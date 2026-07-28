package task

import (
	"context"
	"strings"
)

// RunReadAuthorizer owns task-backed and taskless run-read policy.
type RunReadAuthorizer interface {
	AuthorizeRunRead(ctx context.Context, actor ActorContext, run Run, task *Task) error
}

type taskRunReadAuthorizer struct {
	tasks ResourceAuthorizer
}

var _ RunReadAuthorizer = taskRunReadAuthorizer{}

func (a taskRunReadAuthorizer) AuthorizeRunRead(
	ctx context.Context,
	actor ActorContext,
	run Run,
	taskRecord *Task,
) error {
	if taskRecord != nil {
		if err := a.tasks.AuthorizeTask(ctx, actor, *taskRecord); err != nil {
			return ErrTaskRunNotFound
		}
		if workspaceID := strings.TrimSpace(run.WorkspaceID); workspaceID != "" {
			if err := a.tasks.AuthorizeTaskScope(ctx, actor, ScopeWorkspace, workspaceID); err != nil {
				return ErrTaskRunNotFound
			}
		}
		return nil
	}
	return authorizeTasklessRunRead(actor, run)
}

func authorizeTasklessRunRead(actor ActorContext, run Run) error {
	if actor.Scope.Operator || actor.Actor.Kind.Normalize() == ActorKindDaemon {
		return nil
	}
	if run.RunKind.Normalize() != RunKindNetworkWake || actor.Actor.Kind.Normalize() != ActorKindAgentSession {
		return ErrPermissionDenied
	}

	_, targetSessionID, _ := run.NetworkWakeCorrelation()
	if strings.TrimSpace(actor.Scope.SessionID) != strings.TrimSpace(targetSessionID) ||
		strings.TrimSpace(actor.Scope.WorkspaceID) != strings.TrimSpace(run.WorkspaceID) {
		return ErrTaskRunNotFound
	}
	return nil
}
