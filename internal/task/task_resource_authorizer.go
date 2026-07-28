package task

import (
	"context"
	"strings"
)

// ResourceAuthorizer owns task-resource visibility and mutation policy.
type ResourceAuthorizer interface {
	AuthorizeTask(ctx context.Context, actor ActorContext, task Task) error
	AuthorizeTaskScope(ctx context.Context, actor ActorContext, scope Scope, workspaceID string) error
}

type scopedTaskResourceAuthorizer struct{}

var _ ResourceAuthorizer = scopedTaskResourceAuthorizer{}

func (scopedTaskResourceAuthorizer) AuthorizeTask(
	ctx context.Context,
	actor ActorContext,
	taskRecord Task,
) error {
	return scopedTaskResourceAuthorizer{}.AuthorizeTaskScope(
		ctx,
		actor,
		taskRecord.Scope,
		taskRecord.WorkspaceID,
	)
}

func (scopedTaskResourceAuthorizer) AuthorizeTaskScope(
	_ context.Context,
	actor ActorContext,
	scope Scope,
	workspaceID string,
) error {
	if actor.Scope.Operator || actor.Actor.Kind.Normalize() == ActorKindDaemon {
		return nil
	}
	if scope.Normalize() == ScopeWorkspace {
		actorWorkspaceID := strings.TrimSpace(actor.Scope.WorkspaceID)
		if actor.Actor.Kind.Normalize() == ActorKindAgentSession || actorWorkspaceID != "" {
			if actorWorkspaceID != strings.TrimSpace(workspaceID) {
				return ErrPermissionDenied
			}
		}
	}
	return nil
}

func (m *Service) authorizeTaskResource(
	ctx context.Context,
	actor ActorContext,
	taskRecord Task,
) error {
	authorizer := m.taskAuthorizer
	if authorizer == nil {
		authorizer = scopedTaskResourceAuthorizer{}
	}
	return authorizer.AuthorizeTask(ctx, actor, taskRecord)
}

func (m *Service) authorizeTaskScope(
	ctx context.Context,
	actor ActorContext,
	scope Scope,
	workspaceID string,
) error {
	authorizer := m.taskAuthorizer
	if authorizer == nil {
		authorizer = scopedTaskResourceAuthorizer{}
	}
	return authorizer.AuthorizeTaskScope(ctx, actor, scope, workspaceID)
}

func (m *Service) authorizeRunResource(
	ctx context.Context,
	actor ActorContext,
	run Run,
	taskRecord Task,
) error {
	if err := m.authorizeTaskResource(ctx, actor, taskRecord); err != nil {
		return err
	}
	if workspaceID := strings.TrimSpace(run.WorkspaceID); workspaceID != "" {
		return m.authorizeTaskScope(ctx, actor, ScopeWorkspace, workspaceID)
	}
	return nil
}

func (m *Service) loadAuthorizedRunWithTask(
	ctx context.Context,
	runID string,
	actor ActorContext,
) (Run, Task, error) {
	run, taskRecord, err := m.loadRunWithTask(ctx, runID)
	if err != nil {
		return Run{}, Task{}, err
	}
	if err := m.authorizeRunResource(ctx, actor, run, taskRecord); err != nil {
		return Run{}, Task{}, err
	}
	return run, taskRecord, nil
}

func (m *Service) authorizeRunReviewResource(
	ctx context.Context,
	actor ActorContext,
	review RunReview,
) error {
	run, taskRecord, err := m.loadRunWithTask(ctx, review.RunID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(run.TaskID) != strings.TrimSpace(review.TaskID) {
		return ErrRunReviewNotFound
	}
	return m.authorizeRunResource(ctx, actor, run, taskRecord)
}

type taskResourceReader interface {
	GetTask(ctx context.Context, id string) (Task, error)
}

func (m *Service) loadAuthorizedTask(
	ctx context.Context,
	reader taskResourceReader,
	id string,
	actor ActorContext,
) (Task, error) {
	taskRecord, err := reader.GetTask(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if err := m.authorizeTaskResource(ctx, actor, taskRecord); err != nil {
		return Task{}, err
	}
	return taskRecord, nil
}

func isTaskOperator(actor ActorContext) bool {
	return actor.Scope.Operator || actor.Actor.Kind.Normalize() == ActorKindDaemon
}
