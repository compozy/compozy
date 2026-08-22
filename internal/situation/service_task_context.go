package situation

import (
	"context"

	"errors"
	"log/slog"

	"strings"

	"github.com/compozy/compozy/internal/api/contract"

	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"

	taskpkg "github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (s *Service) taskAndChannelContext(
	ctx context.Context,
	profileID string,
	sessionID string,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
) (contract.AgentTaskContextPayload, contract.AgentCoordinationChannelContextPayload, string, error) {
	taskStore := s.taskStoreValue()
	readScope := store.ReadScope{ProfileID: strings.TrimSpace(profileID)}
	if taskStore == nil || strings.TrimSpace(sessionID) == "" {
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
	}
	if err := readScope.Validate(); err != nil {
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", err
	}

	runs, err := taskStore.ListTaskRuns(ctx, taskpkg.RunQuery{
		ReadScope: readScope, SessionID: strings.TrimSpace(sessionID),
	})
	if err != nil {
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", err
		}
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
	}
	run, ok := selectActiveRun(runs)
	if !ok {
		return s.reviewBindingTaskAndChannelContext(ctx, taskStore, readScope, sessionID, workspaceSnapshot)
	}
	taskRecord, err := taskStore.GetTask(ctx, run.TaskID)
	if err != nil {
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", err
		}
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
	}
	if !readScope.Matches(taskRecord.ProfileID) {
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
	}

	bundle, err := s.sessionContextBundle(ctx, taskRecord, run, workspaceSnapshot, strings.TrimSpace(sessionID))
	if err != nil {
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", err
	}

	channel := coordinationChannelPayload(taskRecord, run)
	lease := contract.TaskRunLeaseSummaryPayload{
		TaskID:                       strings.TrimSpace(run.TaskID),
		RunID:                        strings.TrimSpace(run.ID),
		Status:                       run.Status.Normalize(),
		SessionID:                    strings.TrimSpace(run.SessionID),
		ClaimedBy:                    cloneActorIdentity(run.ClaimedBy),
		ClaimTokenHash:               strings.TrimSpace(run.ClaimTokenHash),
		LeaseUntil:                   optionalTimePtr(run.LeaseUntil),
		HeartbeatAt:                  optionalTimePtr(run.HeartbeatAt),
		ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
	}
	if channel.ID != "" {
		lease.CoordinationChannel = &channel
	}

	taskContext := contract.AgentTaskContextPayload{
		Available: true,
		Task:      taskReferencePayload(taskRecord),
		Lease:     &lease,
		Bundle:    bundle,
	}
	channelContext := contract.AgentCoordinationChannelContextPayload{
		Available: channel.ID != "",
		Channel:   &channel,
	}
	if !channelContext.Available {
		channelContext.Channel = nil
	}
	return taskContext, channelContext, strings.TrimSpace(channel.ID), nil
}

func (s *Service) reviewBindingTaskAndChannelContext(
	ctx context.Context,
	taskStore TaskStore,
	readScope store.ReadScope,
	sessionID string,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
) (contract.AgentTaskContextPayload, contract.AgentCoordinationChannelContextPayload, string, error) {
	review, err := taskStore.LookupRunReviewBySession(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		if errors.Is(err, taskpkg.ErrRunReviewNotFound) {
			return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
		}
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", err
		}
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
	}
	taskRecord, err := taskStore.GetTask(ctx, review.TaskID)
	if err != nil {
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", err
		}
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
	}
	if !readScope.Matches(taskRecord.ProfileID) {
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
	}
	run, err := taskStore.GetTaskRun(ctx, review.RunID)
	if err != nil {
		if isContextError(err) {
			return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", err
		}
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
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
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", nil
	}
	bundle, err := s.sessionContextBundle(ctx, taskRecord, run, workspaceSnapshot, strings.TrimSpace(sessionID))
	if err != nil {
		return contract.AgentTaskContextPayload{}, contract.AgentCoordinationChannelContextPayload{}, "", err
	}

	channel := coordinationChannelPayload(taskRecord, run)
	if reviewerChannelID := strings.TrimSpace(review.ReviewerChannelID); reviewerChannelID != "" {
		channel.ID = reviewerChannelID
		channel.DisplayName = reviewerChannelID
		channel = contract.NormalizeCoordinationChannelPayload(channel)
	}

	taskContext := contract.AgentTaskContextPayload{
		Available: true,
		Task:      taskReferencePayload(taskRecord),
		Bundle:    bundle,
	}
	channelContext := contract.AgentCoordinationChannelContextPayload{
		Available: channel.ID != "",
		Channel:   &channel,
	}
	if !channelContext.Available {
		channelContext.Channel = nil
	}
	return taskContext, channelContext, firstTrimmed(review.ReviewerChannelID, channel.ID), nil
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
