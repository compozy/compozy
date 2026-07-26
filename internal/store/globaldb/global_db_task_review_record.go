package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (g *TaskRunRepo) recordRunReviewWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req taskpkg.RecordRunReviewRequest,
	actor taskpkg.ActorContext,
	recordedAt time.Time,
	continuationRunID string,
) (taskpkg.RunReviewResult, error) {
	current, err := getRunReviewByID(ctx, exec, req.ReviewID)
	if err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	if strings.TrimSpace(current.RunID) != strings.TrimSpace(req.RunID) {
		return taskpkg.RunReviewResult{}, fmt.Errorf(
			"%w: review %q belongs to run %q, not run %q",
			taskpkg.ErrValidation,
			current.ReviewID,
			current.RunID,
			req.RunID,
		)
	}
	run, err := g.tasks.getTaskRunWithExecutor(ctx, exec, current.RunID)
	if err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	if !taskpkg.IsTerminalRunStatus(run.Status) {
		return taskpkg.RunReviewResult{}, fmt.Errorf(
			"%w: run %q is %q and cannot record review verdict until terminal",
			taskpkg.ErrInvalidStatusTransition,
			run.ID,
			run.Status.Normalize(),
		)
	}
	if current.Status.Normalize() == taskpkg.RunReviewStatusRecorded {
		return replayRecordedRunReview(ctx, exec, current, req, actor.Actor)
	}
	if err := validateRunReviewRecordTransition(current, actor.Actor); err != nil {
		return taskpkg.RunReviewResult{}, err
	}

	recorded := applyRunReviewVerdict(current, req.Verdict, actor.Actor, recordedAt)
	if err := updateRunReviewVerdict(ctx, exec, recorded); err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	if err := updateTaskReviewRollup(ctx, exec, recorded, recordedAt); err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	result := taskpkg.RunReviewResult{Review: cloneRunReviewForStore(recorded)}
	if recorded.Outcome.Normalize() != taskpkg.RunReviewOutcomeRejected {
		return result, nil
	}
	taskRecord, err := g.tasks.getTaskWithExecutor(ctx, exec, recorded.TaskID)
	if err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	continuation, err := g.createReviewContinuationRun(
		ctx,
		exec,
		taskRecord,
		run,
		recorded,
		actor,
		continuationRunID,
		recordedAt,
	)
	if err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	result.ContinuationRun = &continuation
	return result, nil
}

func replayRecordedRunReview(
	ctx context.Context,
	exec taskSQLExecutor,
	current taskpkg.RunReview,
	req taskpkg.RecordRunReviewRequest,
	actor taskpkg.ActorIdentity,
) (taskpkg.RunReviewResult, error) {
	if !matchesRecordedRunReviewReplay(current, req.Verdict, actor) {
		return taskpkg.RunReviewResult{}, fmt.Errorf(
			"%w: review %q is already recorded with a different verdict",
			taskpkg.ErrConflict,
			current.ReviewID,
		)
	}
	result := taskpkg.RunReviewResult{Review: cloneRunReviewForStore(current)}
	if current.Outcome.Normalize() != taskpkg.RunReviewOutcomeRejected {
		return result, nil
	}
	continuation, err := getTaskRunByReviewID(ctx, exec, current.ReviewID)
	if err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	result.ContinuationRun = &continuation
	return result, nil
}

func validateRunReviewRecordTransition(review taskpkg.RunReview, actor taskpkg.ActorIdentity) error {
	switch review.Status.Normalize() {
	case taskpkg.RunReviewStatusRequested,
		taskpkg.RunReviewStatusRouted,
		taskpkg.RunReviewStatusInReview:
	default:
		return fmt.Errorf(
			"%w: task run review %q with status %q cannot record a verdict",
			taskpkg.ErrInvalidStatusTransition,
			review.ReviewID,
			review.Status.Normalize(),
		)
	}
	if actor.Kind.Normalize() != taskpkg.ActorKindAgentSession {
		return nil
	}
	if strings.TrimSpace(review.ReviewerSessionID) == "" {
		return nil
	}
	if strings.TrimSpace(actor.Ref) == strings.TrimSpace(review.ReviewerSessionID) {
		return nil
	}
	return fmt.Errorf(
		"%w: reviewer session %q cannot record review %q bound to session %q",
		taskpkg.ErrPermissionDenied,
		actor.Ref,
		review.ReviewID,
		review.ReviewerSessionID,
	)
}

func applyRunReviewVerdict(
	review taskpkg.RunReview,
	verdict taskpkg.RunReviewVerdict,
	actor taskpkg.ActorIdentity,
	recordedAt time.Time,
) taskpkg.RunReview {
	recorded := cloneRunReviewForStore(review)
	recorded.Status = taskpkg.RunReviewStatusRecorded
	recorded.Outcome = verdict.Outcome.Normalize()
	recorded.Confidence = verdict.Confidence
	recorded.Reason = verdict.Reason
	recorded.DeliveryID = verdict.DeliveryID
	recorded.MissingWork = cloneTaskRawJSON(verdict.MissingWork)
	recorded.NextRoundGuidance = verdict.NextRoundGuidance
	recorded.ReviewText = verdict.ReviewText
	recorded.ReviewedBy = &taskpkg.ActorIdentity{Kind: actor.Kind.Normalize(), Ref: strings.TrimSpace(actor.Ref)}
	recorded.ReviewedAt = recordedAt
	recorded.UpdatedAt = recordedAt
	return recorded
}

func updateRunReviewVerdict(ctx context.Context, exec taskSQLExecutor, review taskpkg.RunReview) error {
	affected, err := sqlcgen.New(exec).UpdateRunReviewVerdict(ctx, updateRunReviewVerdictParams(review))
	if err != nil {
		return mapRunReviewRecordError(review.ReviewID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: task run review %q: %w", review.ReviewID, taskpkg.ErrRunReviewNotFound)
	}
	return nil
}

func updateTaskReviewRollup(
	ctx context.Context,
	exec taskSQLExecutor,
	review taskpkg.RunReview,
	recordedAt time.Time,
) error {
	nextRound := review.ReviewRound
	circuitOpenedAt := nullableTaskTime(time.Time{})
	circuitReason := ""
	if review.Outcome.Normalize() == taskpkg.RunReviewOutcomeRejected {
		nextRound = review.ReviewRound + 1
	}
	if review.Outcome.Normalize() == taskpkg.RunReviewOutcomeBlocked {
		circuitOpenedAt = nullableTaskTime(recordedAt)
		circuitReason = review.Reason
	}

	affected, err := sqlcgen.New(exec).UpdateTaskReviewRollup(ctx, sqlcgen.UpdateTaskReviewRollupParams{
		ReviewRound: int64(nextRound), LastReviewID: nullableTaskString(review.ReviewID),
		LastReviewOutcome:     nullableTaskString(string(review.Outcome)),
		ReviewCircuitOpenedAt: circuitOpenedAt,
		ReviewCircuitReason:   nullableTaskString(circuitReason),
		UpdatedAt:             store.FormatTimestamp(recordedAt), ID: review.TaskID,
	})
	if err != nil {
		return fmt.Errorf("store: update task %q review rollup: %w", review.TaskID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: task %q: %w", review.TaskID, taskpkg.ErrTaskNotFound)
	}
	return nil
}

func (g *TaskRunRepo) createReviewContinuationRun(
	ctx context.Context,
	exec taskSQLExecutor,
	taskRecord taskpkg.Task,
	parentRun taskpkg.Run,
	review taskpkg.RunReview,
	actor taskpkg.ActorContext,
	runID string,
	queuedAt time.Time,
) (taskpkg.Run, error) {
	openRunID, err := g.tasks.findOpenRunIDForQueuedRunReservation(ctx, exec, taskRecord.ID, "")
	if err != nil {
		return taskpkg.Run{}, err
	}
	if openRunID != "" {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task %q has open run %q; finish or cancel it before enqueueing review continuation",
			taskpkg.ErrInvalidStatusTransition,
			taskRecord.ID,
			openRunID,
		)
	}
	nextAttempt, err := nextTaskRunAttemptWithExecutor(ctx, exec, taskRecord)
	if err != nil {
		return taskpkg.Run{}, err
	}
	metadata, err := reviewContinuationMetadata(review, &parentRun)
	if err != nil {
		return taskpkg.Run{}, err
	}
	runAttempt, err := taskRunAttemptFromInt(nextAttempt)
	if err != nil {
		return taskpkg.Run{}, err
	}
	run := taskpkg.Run{
		ID:          strings.TrimSpace(runID),
		TaskID:      taskRecord.ID,
		WorkspaceID: taskRecord.WorkspaceID,
		Status:      taskpkg.TaskRunStatusQueued,
		Attempt:     runAttempt,
		Origin:      actor.Origin,
		Review: &taskpkg.RunReviewLineage{
			ParentRunID:        parentRun.ID,
			ReviewID:           review.ReviewID,
			ReviewRound:        review.ReviewRound + 1,
			ContinuationReason: review.Reason,
			MissingWork:        cloneTaskRawJSON(review.MissingWork),
			NextRoundGuidance:  review.NextRoundGuidance,
		},
		Metadata: metadata,
		QueuedAt: queuedAt,
	}
	run.SetNetworkState(parentRun.NetworkSpecSnapshot(), "", "", "")
	normalized, err := g.tasks.normalizeTaskRunForCreate(run)
	if err != nil {
		return taskpkg.Run{}, err
	}
	normalized, err = bindTaskRunWorkspace(normalized, taskRecord)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if err := insertQueuedTaskRun(ctx, exec, normalized); err != nil {
		return taskpkg.Run{}, mapReviewContinuationInsertError(review.ReviewID, err)
	}
	return g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.ID)
}

func reviewContinuationMetadata(review taskpkg.RunReview, parentRun *taskpkg.Run) (json.RawMessage, error) {
	payload := map[string]string{
		globalDBTaskReviewSourceKey:   "task_run_review",
		globalDBTaskReviewReviewIDKey: review.ReviewID,
		globalDBOutcomeKey:            string(review.Outcome),
	}
	if parentRun != nil {
		payload["parent_run_id"] = parentRun.ID
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("store: encode review continuation metadata: %w", err)
	}
	return raw, nil
}

func getTaskRunByReviewID(
	ctx context.Context,
	exec taskSQLExecutor,
	reviewID string,
) (taskpkg.Run, error) {
	runID, err := sqlcgen.New(exec).GetTaskRunIDByReviewID(ctx, nullableTaskString(reviewID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
		}
		return taskpkg.Run{}, err
	}
	row, err := sqlcgen.New(exec).GetTaskRun(ctx, runID)
	if err != nil {
		return taskpkg.Run{}, fmt.Errorf("store: get task run %q for review %q: %w", runID, reviewID, err)
	}
	return taskRunFromGenerated(&row)
}

func matchesRecordedRunReviewReplay(
	current taskpkg.RunReview,
	verdict taskpkg.RunReviewVerdict,
	actor taskpkg.ActorIdentity,
) bool {
	return current.DeliveryID == verdict.DeliveryID &&
		current.Outcome.Normalize() == verdict.Outcome.Normalize() &&
		actorsEqual(current.ReviewedBy, actor) &&
		current.Reason == verdict.Reason &&
		string(current.MissingWork) == string(verdict.MissingWork) &&
		current.NextRoundGuidance == verdict.NextRoundGuidance &&
		current.ReviewText == verdict.ReviewText
}

func actorsEqual(left *taskpkg.ActorIdentity, right taskpkg.ActorIdentity) bool {
	if left == nil {
		return false
	}
	return left.Kind.Normalize() == right.Kind.Normalize() &&
		strings.TrimSpace(left.Ref) == strings.TrimSpace(right.Ref)
}

func mapRunReviewRecordError(reviewID string, err error) error {
	if err == nil {
		return nil
	}
	if isTaskRunReviewSQLiteUniqueConstraint(err) {
		return fmt.Errorf("%w: delivery id is already recorded for review %q", taskpkg.ErrConflict, reviewID)
	}
	return fmt.Errorf("store: record task run review %q: %w", reviewID, err)
}

func mapReviewContinuationInsertError(reviewID string, err error) error {
	if err == nil {
		return nil
	}
	if isTaskRunReviewSQLiteUniqueConstraint(err) {
		return fmt.Errorf("%w: continuation run already exists for review %q", taskpkg.ErrConflict, reviewID)
	}
	return err
}
