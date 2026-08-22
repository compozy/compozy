package task

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// ResourceAuthorizer owns task-resource visibility and mutation policy.
type ResourceAuthorizer interface {
	AuthorizeTask(ctx context.Context, actor ActorContext, task Task) error
	AuthorizeTaskScope(ctx context.Context, actor ActorContext, scope Scope, workspaceID string) error
}

type scopedTaskResourceAuthorizer struct {
	workspaceAccess workspaceaccess.Policy
}

var _ ResourceAuthorizer = scopedTaskResourceAuthorizer{}

func (a scopedTaskResourceAuthorizer) AuthorizeTask(
	ctx context.Context,
	actor ActorContext,
	taskRecord Task,
) error {
	return a.AuthorizeTaskScope(
		ctx,
		actor,
		taskRecord.Scope,
		taskRecord.WorkspaceID,
	)
}

func (a scopedTaskResourceAuthorizer) AuthorizeTaskScope(
	ctx context.Context,
	actor ActorContext,
	scope Scope,
	workspaceID string,
) error {
	if actor.Scope.Operator || actor.Actor.Kind.Normalize() == ActorKindDaemon {
		return nil
	}
	if scope.Normalize() == ScopeWorkspace {
		actorWorkspaceID := strings.TrimSpace(actor.Scope.WorkspaceID)
		targetWorkspaceID := strings.TrimSpace(workspaceID)
		if actorWorkspaceID != targetWorkspaceID {
			allowed, err := taskWorkspaceAccessAllowed(ctx, a.workspaceAccess, actor, targetWorkspaceID)
			if err != nil {
				return errors.Join(ErrPermissionDenied, err)
			}
			if !allowed {
				return ErrPermissionDenied
			}
		}
	}
	return nil
}

func taskWorkspaceAccessAllowed(
	ctx context.Context,
	policy workspaceaccess.Policy,
	actor ActorContext,
	targetWorkspaceID string,
) (bool, error) {
	if policy == nil {
		return false, nil
	}
	decision, err := policy.Authorize(ctx, workspaceaccess.Request{
		Actor: workspaceaccess.ActorRef{
			Kind:        workspaceaccess.ActorKind(actor.Actor.Kind.Normalize()),
			SessionID:   strings.TrimSpace(actor.Scope.SessionID),
			WorkspaceID: strings.TrimSpace(actor.Scope.WorkspaceID),
			Operator:    actor.Scope.Operator,
		},
		TargetWorkspaceID: strings.TrimSpace(targetWorkspaceID),
		Seam:              workspaceaccess.SeamTask,
	})
	if err != nil {
		slog.ErrorContext(
			ctx,
			"task: workspace access authorization failed",
			"error", err,
			"actor_kind", actor.Actor.Kind.Normalize(),
			"actor_ref", strings.TrimSpace(actor.Actor.Ref),
			"workspace_id", strings.TrimSpace(targetWorkspaceID),
			"seam", workspaceaccess.SeamTask,
		)
	}
	return decision.Allowed, err
}

func (m *Service) authorizeTaskResource(
	ctx context.Context,
	actor ActorContext,
	taskRecord Task,
) error {
	if actor.ReadScope != (store.ReadScope{}) && !actor.ReadScope.Matches(taskRecord.ProfileID) {
		return ErrTaskNotFound
	}
	authorizer := m.taskAuthorizer
	if authorizer == nil {
		authorizer = scopedTaskResourceAuthorizer{workspaceAccess: m.workspaceAccess}
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
		authorizer = scopedTaskResourceAuthorizer{workspaceAccess: m.workspaceAccess}
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

type profileScopedTaskResourceReader interface {
	GetTaskInReadScope(ctx context.Context, id string, readScope store.ReadScope) (Task, error)
}

func (m *Service) loadAuthorizedTask(
	ctx context.Context,
	reader taskResourceReader,
	id string,
	actor ActorContext,
) (Task, error) {
	var taskRecord Task
	var err error
	if actor.ReadScope != (store.ReadScope{}) {
		if scopedReader, ok := reader.(profileScopedTaskResourceReader); ok {
			taskRecord, err = scopedReader.GetTaskInReadScope(ctx, id, actor.ReadScope)
		} else {
			taskRecord, err = reader.GetTask(ctx, id)
			if err == nil && !actor.ReadScope.Matches(taskRecord.ProfileID) {
				err = ErrTaskNotFound
			}
		}
	} else {
		taskRecord, err = reader.GetTask(ctx, id)
	}
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
