package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (h *BaseHandlers) taskRunReviewPayload(
	ctx context.Context,
	manager TaskService,
	review taskpkg.RunReview,
	actor taskpkg.ActorContext,
) (contract.TaskRunReviewPayload, error) {
	payloads, err := h.taskRunReviewPayloads(ctx, manager, []taskpkg.RunReview{review}, actor)
	if err != nil {
		return contract.TaskRunReviewPayload{}, err
	}
	if len(payloads) != 1 {
		return contract.TaskRunReviewPayload{}, fmt.Errorf("api: task run review projection is empty")
	}
	return payloads[0], nil
}

func (h *BaseHandlers) taskRunReviewPayloads(
	ctx context.Context,
	manager TaskService,
	reviews []taskpkg.RunReview,
	actor taskpkg.ActorContext,
) ([]contract.TaskRunReviewPayload, error) {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return nil, err
	}
	profileIDs := make(map[string]string)
	payloads := make([]contract.TaskRunReviewPayload, 0, len(reviews))
	for _, review := range reviews {
		taskID := strings.TrimSpace(review.TaskID)
		profileID, found := profileIDs[taskID]
		if !found {
			view, getErr := manager.GetTask(ctx, taskID, actor)
			if getErr != nil {
				return nil, getErr
			}
			profileID = strings.TrimSpace(view.Task.ProfileID)
			profileIDs[taskID] = profileID
		}
		owner, ownerErr := taskProfileOwner(owners, review.ReviewID, profileID)
		if ownerErr != nil {
			return nil, ownerErr
		}
		payloads = append(payloads, contract.TaskRunReviewPayload{
			RunReview: review, ProfileID: owner.ID, ProfileName: owner.Name,
			ProfileColor: owner.Color, ProfileIcon: owner.Icon, ProfileEmoji: owner.Emoji,
			ProfileArchived: owner.Archived,
		})
	}
	return payloads, nil
}

func (h *BaseHandlers) taskRunReviewVerdictResponse(
	ctx context.Context,
	manager TaskService,
	result *taskpkg.RunReviewResult,
	actor taskpkg.ActorContext,
) (contract.TaskRunReviewVerdictResponse, error) {
	if result == nil {
		return contract.TaskRunReviewVerdictResponse{}, nil
	}
	review, err := h.taskRunReviewPayload(ctx, manager, result.Review, actor)
	if err != nil {
		return contract.TaskRunReviewVerdictResponse{}, err
	}
	response := contract.TaskRunReviewVerdictResponse{
		Review: review, CircuitOpened: result.CircuitOpened,
	}
	if result.ContinuationRun != nil {
		continuations := []contract.TaskRunPayload{TaskRunPayloadFromRun(result.ContinuationRun)}
		if err := h.decorateTaskRunOwners(ctx, continuations); err != nil {
			return contract.TaskRunReviewVerdictResponse{}, err
		}
		response.ContinuationRun = &continuations[0]
	}
	return response, nil
}
