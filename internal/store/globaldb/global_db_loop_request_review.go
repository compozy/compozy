package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
)

const reviewRejectedRouteOutputRefPrefix = "review_rejected_route:"

func requestDecisionPayload(
	ctx context.Context,
	exec taskSQLExecutor,
	stored storedRequest,
	decision string,
	payload json.RawMessage,
) (json.RawMessage, json.RawMessage, error) {
	if stored.request.Kind == looppkg.RequestKindAsk {
		return cloneRawJSON(payload), stored.answerSchema, nil
	}
	switch decision {
	case looppkg.RequestDecisionApprove:
		if stored.proposedRef == "" {
			return nil, nil, fmt.Errorf("%w: review proposal is missing", looppkg.ErrTransitionConflict)
		}
		proposed, err := getLoopOutputByRefWithExecutor(ctx, exec, stored.proposedRef)
		if err != nil {
			return nil, nil, fmt.Errorf("store: load review proposal: %w", err)
		}
		return proposed, nil, nil
	case looppkg.RequestDecisionEdit:
		return cloneRawJSON(payload), stored.editSchema, nil
	case looppkg.RequestDecisionRespond:
		return cloneRawJSON(payload), stored.respondSchema, nil
	case looppkg.RequestDecisionReject:
		return json.RawMessage(`null`), nil, nil
	default:
		return cloneRawJSON(payload), nil, nil
	}
}

func claimRequestDecision(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation looppkg.WaitResumeMutation,
	wait looppkg.NodeWait,
	requestKind string,
	decision string,
	answerRef string,
	rejectRoute looppkg.NodeID,
) error {
	if requestKind != looppkg.RequestKindReview || decision == looppkg.RequestDecisionRespond {
		return claimAndResolveWait(ctx, exec, mutation, wait, answerRef)
	}
	result, err := exec.ExecContext(ctx, `UPDATE loop_node_waits SET claim_state = 'resumed',
		claimed_by_kind = ?, claimed_by_id = ?, claimed_at = ?, ahead_payload_json = ?
		WHERE loop_run_id = ? AND generation = ? AND node_id = ? AND item_index = ?
		AND claim_state = 'waiting' AND issued_epoch = ?`, strings.TrimSpace(mutation.ClaimedByKind),
		strings.TrimSpace(mutation.ClaimedByID), mutation.RequestedAt.UTC(), string(mutation.Payload),
		mutation.RunID, mutation.Generation, mutation.NodeID, mutation.ItemIndex, wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: claim Loop review wait: %w", err)
	}
	if err := requireSingleWaitMutation(result); err != nil {
		return err
	}
	status := goalContextStatePending
	outputRef := answerRef
	if decision == looppkg.RequestDecisionReject {
		if rejectRoute != "" {
			status = loopNodeOutputSucceeded
			outputRef = reviewRejectedRouteOutputRefPrefix + strings.TrimSpace(string(rejectRoute))
		} else {
			status = loopNodeOutputFailed
			failure := looppkg.NewActionFailure(
				string(looppkg.FailureQualityRejection),
				"The action was rejected during review.",
				"Revise the proposed action inputs, then start a new generation.",
			)
			encoded, ok := looppkg.ActionFailureOutputRef(failure)
			if !ok {
				return fmt.Errorf("%w: encode review rejection", looppkg.ErrValidation)
			}
			outputRef = encoded
		}
	}
	result, err = exec.ExecContext(ctx, `UPDATE loop_generation_outputs SET status = ?,
		output_ref = ?, task_run_id = NULL, next_attempt_at = NULL, first_scheduled_at = NULL,
		epoch = epoch + 1 WHERE loop_run_id = ? AND generation = ? AND node_id = ?
		AND item_index = ? AND status = 'waiting' AND epoch = ?`, status, outputRef,
		mutation.RunID, mutation.Generation, mutation.NodeID, mutation.ItemIndex, wait.IssuedEpoch)
	if err != nil {
		return fmt.Errorf("store: admit Loop review decision: %w", err)
	}
	return requireSingleWaitMutation(result)
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	return append(json.RawMessage(nil), raw...)
}
