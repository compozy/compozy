package situation

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (s *Service) taskContext(
	ctx context.Context,
	profileID string,
	sessionID string,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
) (contract.AgentTaskContextPayload, error) {
	taskStore := s.taskStoreValue()
	readScope := profileReadScope(profileID)
	if taskStore == nil || strings.TrimSpace(sessionID) == "" {
		return contract.AgentTaskContextPayload{}, nil
	}
	if err := readScope.Validate(); err != nil {
		return contract.AgentTaskContextPayload{}, err
	}

	runs, err := taskStore.ListTaskRuns(ctx, taskpkg.RunQuery{
		ReadScope: readScope, SessionID: strings.TrimSpace(sessionID),
	})
	if err != nil {
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, err
		}
		return contract.AgentTaskContextPayload{}, nil
	}
	run, ok := selectActiveRun(runs)
	if !ok {
		return s.reviewBindingTaskContext(ctx, taskStore, readScope, sessionID, workspaceSnapshot)
	}
	taskRecord, err := taskStore.GetTask(ctx, run.TaskID)
	if err != nil {
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, err
		}
		return contract.AgentTaskContextPayload{}, nil
	}
	if !readScope.Matches(taskRecord.ProfileID) {
		return contract.AgentTaskContextPayload{}, nil
	}

	bundle, err := s.sessionContextBundle(ctx, taskRecord, run, workspaceSnapshot, strings.TrimSpace(sessionID))
	if err != nil {
		return contract.AgentTaskContextPayload{}, err
	}

	lease := contract.TaskRunLeaseSummaryPayload{
		TaskID:         strings.TrimSpace(run.TaskID),
		RunID:          strings.TrimSpace(run.ID),
		Status:         run.Status.Normalize(),
		SessionID:      strings.TrimSpace(run.SessionID),
		ClaimedBy:      cloneActorIdentity(run.ClaimedBy),
		ClaimTokenHash: strings.TrimSpace(run.ClaimTokenHash),
		LeaseUntil:     optionalTimePtr(run.LeaseUntil),
		HeartbeatAt:    optionalTimePtr(run.HeartbeatAt),
	}

	taskContext := contract.AgentTaskContextPayload{
		Available: true,
		Task:      taskReferencePayload(taskRecord),
		Lease:     &lease,
		Bundle:    bundle,
	}
	return taskContext, nil
}

func (s *Service) reviewBindingTaskContext(
	ctx context.Context,
	taskStore TaskStore,
	readScope store.ReadScope,
	sessionID string,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
) (contract.AgentTaskContextPayload, error) {
	review, err := taskStore.LookupRunReviewBySession(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		if errors.Is(err, taskpkg.ErrRunReviewNotFound) {
			return contract.AgentTaskContextPayload{}, nil
		}
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, err
		}
		return contract.AgentTaskContextPayload{}, nil
	}
	taskRecord, err := taskStore.GetTask(ctx, review.TaskID)
	if err != nil {
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, err
		}
		return contract.AgentTaskContextPayload{}, nil
	}
	if !readScope.Matches(taskRecord.ProfileID) {
		return contract.AgentTaskContextPayload{}, nil
	}
	run, err := taskStore.GetTaskRun(ctx, review.RunID)
	if err != nil {
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, err
		}
		return contract.AgentTaskContextPayload{}, nil
	}
	if strings.TrimSpace(run.TaskID) != strings.TrimSpace(taskRecord.ID) {
		slog.Warn(
			"situation: skip review-bound context for mismatched task and run",
			"session_id", strings.TrimSpace(sessionID),
			"review_id", strings.TrimSpace(review.ReviewID),
			"task_id", strings.TrimSpace(taskRecord.ID),
			"run_id", strings.TrimSpace(run.ID),
			"run_task_id", strings.TrimSpace(run.TaskID),
		)
		return contract.AgentTaskContextPayload{}, nil
	}
	bundle, err := s.sessionContextBundle(ctx, taskRecord, run, workspaceSnapshot, strings.TrimSpace(sessionID))
	if err != nil {
		return contract.AgentTaskContextPayload{}, err
	}

	taskContext := contract.AgentTaskContextPayload{
		Available: true,
		Task:      taskReferencePayload(taskRecord),
		Bundle:    bundle,
	}
	return taskContext, nil
}

func (s *Service) sessionContextBundle(
	ctx context.Context,
	taskRecord taskpkg.Task,
	run taskpkg.Run,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
	sessionID string,
) (*taskpkg.ContextBundle, error) {
	bundle, err := s.bundleForRun(ctx, taskRecord, run, workspaceSnapshot, nil)
	if err == nil {
		return &bundle, nil
	}
	if isContextError(err) {
		return nil, err
	}

	slog.Warn(
		"situation: skip task context bundle enrichment",
		"session_id", strings.TrimSpace(sessionID),
		"task_id", strings.TrimSpace(taskRecord.ID),
		"run_id", strings.TrimSpace(run.ID),
		"error", safeTaskContextText(err.Error(), 240),
	)
	return nil, nil
}
