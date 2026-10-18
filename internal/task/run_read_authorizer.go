package task

import (
	"context"
	"strings"
)

// RunReadAuthorizer owns task run-read policy.
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
	return ErrTaskRunNotFound
}
