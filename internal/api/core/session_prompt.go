package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/session"
	"github.com/gin-gonic/gin"
)

// PromptResponseFromSession selects the direct Goal body or ordinary prompt wrapper and status.
func PromptResponseFromSession(result session.SendPromptResult) (int, any, error) {
	if result.Goal != nil {
		payload, err := GoalCommandResultPayloadFromSession(result.Goal)
		if err != nil {
			return 0, nil, err
		}
		return GoalCommandHTTPStatus(payload), payload, nil
	}
	payload, err := PromptResultPayloadFromSession(result)
	if err != nil {
		return 0, nil, err
	}
	status := http.StatusOK
	if result.Queued || result.Staged {
		status = http.StatusAccepted
	}
	return status, contract.SendPromptResultResponse{Prompt: payload}, nil
}

// RespondPromptResult writes one validated non-streaming prompt or Goal response.
func RespondPromptResult(c *gin.Context, result session.SendPromptResult, maskInternal bool) bool {
	status, body, err := PromptResponseFromSession(result)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, maskInternal)
		return false
	}
	c.JSON(status, body)
	return true
}

// PromptResultPayloadFromSession converts runtime prompt admission state into the shared DTO.
func PromptResultPayloadFromSession(result session.SendPromptResult) (contract.SendPromptResultPayload, error) {
	payload := contract.SendPromptResultPayload{
		Status:                strings.TrimSpace(result.Status),
		Mode:                  contract.PromptMode(strings.TrimSpace(string(result.Mode))),
		Queued:                result.Queued,
		Staged:                result.Staged,
		Interrupted:           result.Interrupted,
		QueueEntryID:          strings.TrimSpace(result.QueueEntryID),
		QueuePosition:         result.QueuePosition,
		QueueGeneration:       result.QueueGeneration,
		EstimatedSendAt:       result.EstimatedSendAt,
		PreviousTurnID:        strings.TrimSpace(result.PreviousTurnID),
		NewTurnID:             strings.TrimSpace(result.NewTurnID),
		CanceledQueuedEntries: result.CanceledQueuedEntries,
	}
	if fallbackMode := strings.TrimSpace(result.FallbackModeIfNoToolResult); fallbackMode != "" {
		payload.FallbackModeIfNoToolResult = contract.PromptMode(fallbackMode)
	}
	if result.Goal != nil {
		goal, err := GoalCommandResultPayloadFromSession(result.Goal)
		if err != nil {
			return contract.SendPromptResultPayload{}, err
		}
		payload.Goal = &goal
	}
	return payload, nil
}

// GoalCommandResultPayloadFromSession maps and validates one structured Goal command result.
func GoalCommandResultPayloadFromSession(result *session.GoalCommandResult) (contract.GoalCommandResult, error) {
	if result == nil {
		return contract.GoalCommandResult{}, fmt.Errorf("%w: Goal command result is required", looppkg.ErrValidation)
	}
	payload := contract.GoalCommandResult{Outcome: contract.GoalCommandOutcome(result.Outcome)}
	if result.ReasonCode != nil {
		reason := contract.GoalReasonCode(*result.ReasonCode)
		payload.ReasonCode = &reason
	}
	snapshot, err := goalSnapshotPayload(result.Snapshot)
	if err != nil {
		return contract.GoalCommandResult{}, err
	}
	payload.Snapshot = snapshot
	if result.ReplacedRunID != nil {
		value := strings.TrimSpace(*result.ReplacedRunID)
		if value == "" {
			return contract.GoalCommandResult{}, goalContractValidationError(
				"Goal replacement run ID must not be blank",
			)
		}
		payload.ReplacedRunID = &value
	}
	if err := validateGoalCommandResultPayload(payload); err != nil {
		return contract.GoalCommandResult{}, err
	}
	return payload, nil
}

// GoalCommandHTTPStatus returns the closed transport status for one valid Goal result.
func GoalCommandHTTPStatus(result contract.GoalCommandResult) int {
	switch result.Outcome {
	case contract.GoalOutcomeStarted, contract.GoalOutcomeReplaced:
		return 202
	case contract.GoalOutcomeStatus, contract.GoalOutcomePaused,
		contract.GoalOutcomeResumed, contract.GoalOutcomeCleared:
		return 200
	case contract.GoalOutcomeError:
		if result.ReasonCode == nil {
			return 422
		}
		switch *result.ReasonCode {
		case contract.GoalReasonNotActive:
			return 404
		case contract.GoalReasonReplaceRequired, contract.GoalReasonReplaceStale,
			contract.GoalReasonDraftRequiresIdle, contract.GoalReasonControlStale,
			contract.GoalReasonPromptFenced:
			return 409
		default:
			return 422
		}
	default:
		return 422
	}
}

func validateGoalCommandResultPayload(result contract.GoalCommandResult) error {
	hasSnapshot := result.Snapshot != nil
	hasReason := result.ReasonCode != nil
	hasReplaced := result.ReplacedRunID != nil && strings.TrimSpace(*result.ReplacedRunID) != ""
	switch result.Outcome {
	case contract.GoalOutcomeStarted, contract.GoalOutcomeStatus,
		contract.GoalOutcomePaused, contract.GoalOutcomeResumed:
		if !hasSnapshot || hasReason || hasReplaced {
			return goalContractValidationError(fmt.Sprintf(
				"Goal %s result requires a snapshot and no reason or replacement run ID",
				result.Outcome,
			))
		}
	case contract.GoalOutcomeReplaced:
		if !hasSnapshot || hasReason || !hasReplaced {
			return goalContractValidationError(
				"Goal replaced result requires a snapshot and replacement run ID without a reason",
			)
		}
	case contract.GoalOutcomeCleared:
		if hasSnapshot || hasReason || hasReplaced {
			return goalContractValidationError(
				"Goal cleared result cannot include a snapshot, reason, or replacement run ID",
			)
		}
	case contract.GoalOutcomeError:
		if !hasReason || hasReplaced {
			return goalContractValidationError(
				"Goal error result requires a reason and no replacement run ID",
			)
		}
		requiresSnapshot := *result.ReasonCode == contract.GoalReasonReplaceRequired ||
			*result.ReasonCode == contract.GoalReasonReplaceStale
		if hasSnapshot != requiresSnapshot {
			return goalContractValidationError(fmt.Sprintf(
				"Goal error reason %q requires current snapshot: %t",
				*result.ReasonCode,
				requiresSnapshot,
			))
		}
	default:
		return goalContractValidationError(fmt.Sprintf("Goal command outcome is invalid: %q", result.Outcome))
	}
	return nil
}

// PromptCallerForWorkspace derives the authenticated prompt caller for external ingress.
func (h *BaseHandlers) PromptCallerForWorkspace(
	c *gin.Context,
	workspaceID string,
) (session.PromptCaller, error) {
	actor, err := h.taskActorContextForWorkspace(c, "session_prompt", workspaceID)
	if err != nil {
		return session.PromptCaller{}, err
	}
	caller := session.PromptCaller{
		Kind:   string(actor.Actor.Kind.Normalize()),
		ID:     strings.TrimSpace(actor.Actor.Ref),
		Source: string(actor.Origin.Kind),
	}
	if err := caller.Validate(); err != nil {
		return session.PromptCaller{}, err
	}
	return caller, nil
}
