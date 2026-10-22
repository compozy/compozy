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
	return a.authorizeTaskTarget(ctx, actor, taskRecord.Scope, taskRecord.WorkspaceID, taskRecord.ProfileID)
}

func (a scopedTaskResourceAuthorizer) AuthorizeTaskScope(
	ctx context.Context,
	actor ActorContext,
	scope Scope,
	workspaceID string,
) error {
	return a.authorizeTaskTarget(ctx, actor, scope, workspaceID, actor.ReadScope.ProfileID)
}

// authorizeTaskTarget sends agent-session access through the workspace policy (USW BR-6). Non-session
// actors keep their existing authority: they act for the operator-authored automation or extension.
func (a scopedTaskResourceAuthorizer) authorizeTaskTarget(
	ctx context.Context,
	actor ActorContext,
	scope Scope,
	workspaceID string,
	targetProfileID string,
) error {
	if actor.Scope.Operator || actor.Actor.Kind.Normalize() == ActorKindDaemon {
		return nil
	}
	targetWorkspaceID := strings.TrimSpace(workspaceID)
	if scope.Normalize() == ScopeGlobal {
		targetWorkspaceID = ""
	}
	if actor.Actor.Kind.Normalize() != ActorKindAgentSession {
		if targetWorkspaceID == "" || strings.TrimSpace(actor.Scope.WorkspaceID) == targetWorkspaceID {
			return nil
		}
		return a.requirePolicy(ctx, actor, targetWorkspaceID, targetProfileID)
	}
	targetProfileID = strings.TrimSpace(targetProfileID)
	if a.workspaceAccess == nil {
		// Without a policy (isolated task services) only the caller's own scope is reachable.
		if strings.TrimSpace(actor.Scope.WorkspaceID) != targetWorkspaceID {
			return ErrPermissionDenied
		}
		if userProfileID := strings.TrimSpace(actor.Scope.UserProfileID); targetWorkspaceID == "" &&
			userProfileID != "" && targetProfileID != "" && userProfileID != targetProfileID {
			return ErrPermissionDenied
		}
		return nil
	}
	// Agent sessions otherwise reach the policy: only it knows the authenticated session profile,
	// and it answers same-scope access on its fast path without an audit record.
	if targetProfileID == "" {
		targetProfileID = strings.TrimSpace(actor.Scope.UserProfileID)
	}
	if targetProfileID == "" {
		targetProfileID = strings.TrimSpace(actor.ReadScope.ProfileID)
	}
	return a.requirePolicy(ctx, actor, targetWorkspaceID, targetProfileID)
}

func (a scopedTaskResourceAuthorizer) requirePolicy(
	ctx context.Context,
	actor ActorContext,
	targetWorkspaceID string,
	targetProfileID string,
) error {
	actor.ReadScope = store.ReadScope{ProfileID: targetProfileID}
	allowed, err := taskWorkspaceAccessAllowed(ctx, a.workspaceAccess, actor, targetWorkspaceID)
	if err != nil {
		return errors.Join(ErrPermissionDenied, err)
	}
	if !allowed {
		return ErrPermissionDenied
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
		Target: workspaceaccess.ScopeTarget(strings.TrimSpace(targetWorkspaceID), actor.ReadScope.ProfileID),
		Seam:   workspaceaccess.SeamTask,
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
	run.ProfileID = taskRecord.ProfileID
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

func canBypassTaskReadAuthorization(actor ActorContext) bool {
	return isTaskOperator(actor) && actor.ReadScope.AllProfiles
}
