package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// RecordReportIntent writes the only durable goal_report intent for the current prompt.
func (g *GoalRepo) RecordReportIntent(
	ctx context.Context,
	req goal.RecordReportIntentRequest,
) (goal.ReportIntent, error) {
	if err := g.checkReady(ctx, "record goal report intent"); err != nil {
		return goal.ReportIntent{}, err
	}
	if err := validateReportIntentRequest(req); err != nil {
		return goal.ReportIntent{}, err
	}
	now := g.now()
	var intent goal.ReportIntent
	err := g.withTaskImmediateTransaction(ctx, "record goal report intent", func(exec taskSQLExecutor) error {
		var err error
		intent, err = recordReportIntentWithExecutor(ctx, exec, req, now)
		return err
	})
	if err != nil {
		return goal.ReportIntent{}, err
	}
	return intent, nil
}

func recordReportIntentWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.RecordReportIntentRequest,
	now time.Time,
) (goal.ReportIntent, error) {
	if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
		return goal.ReportIntent{}, err
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
	if err != nil {
		return goal.ReportIntent{}, err
	}
	if checkpoint.ControlEpoch != req.ExpectedControlEpoch ||
		checkpoint.BindingEpoch != req.ExpectedBindingEpoch ||
		checkpoint.Phase != goalCheckpointPhasePrompting ||
		checkpoint.Status != goalStatusActive || checkpoint.PromptID != strings.TrimSpace(req.PromptID) {
		return goal.ReportIntent{}, goalNotActiveError("report does not target the current prompting operation")
	}
	if err := validateActiveGoalPromptBinding(
		ctx,
		exec,
		req.Key,
		checkpoint.BindingHandle,
		req.ExpectedBindingEpoch,
		checkpoint.SessionID,
	); err != nil {
		return goal.ReportIntent{}, err
	}
	if checkpoint.ReportIntent != nil {
		if reportIntentMatchesRequest(*checkpoint.ReportIntent, req) {
			return *checkpoint.ReportIntent, nil
		}
		return goal.ReportIntent{}, &looppkg.ReasonError{
			Code: looppkg.ReasonCodeGoalReportConflict,
			Err:  fmt.Errorf("%w: report intent already differs", looppkg.ErrTransitionConflict),
		}
	}
	affected, err := sqlcgen.New(exec).RecordGoalReportIntent(ctx, sqlcgen.RecordGoalReportIntentParams{
		ReportPromptID: goalNullableString(req.PromptID), ReportStatus: goalNullableString(req.Status),
		ReportEvidenceRef:  goalNullableString(req.EvidenceRef),
		ReportBindingEpoch: goalNullableInt64(&req.ExpectedBindingEpoch),
		ReportActorKind:    goalNullableString(req.ActorKind), ReportActorID: goalNullableString(req.ActorID),
		ReportRecordedAt: store.FormatTimestamp(now), UpdatedAt: store.FormatTimestamp(now),
		LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation),
		NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
		ControlEpoch: req.ExpectedControlEpoch, BindingEpoch: goalNullableInt64(&req.ExpectedBindingEpoch),
		PromptID: goalNullableString(req.PromptID),
	})
	if err != nil {
		return goal.ReportIntent{}, fmt.Errorf("store: persist goal report intent: %w", err)
	}
	if err := requireGoalAffectedCount(affected, "record goal report intent"); err != nil {
		return goal.ReportIntent{}, err
	}
	return goal.ReportIntent{
		PromptID:     strings.TrimSpace(req.PromptID),
		Status:       strings.TrimSpace(req.Status),
		EvidenceRef:  strings.TrimSpace(req.EvidenceRef),
		BindingEpoch: req.ExpectedBindingEpoch,
		ActorKind:    strings.TrimSpace(req.ActorKind),
		ActorID:      strings.TrimSpace(req.ActorID),
		RecordedAt:   now,
	}, nil
}

// BeginCompactionCancel persists timeout intent before issuing the correlated cancellation effect.
func (g *GoalRepo) BeginCompactionCancel(ctx context.Context, req goal.BeginCompactionCancelRequest) error {
	if err := g.checkReady(ctx, "begin goal compaction cancel"); err != nil {
		return err
	}
	if err := validateCompactionCancelRequest(req); err != nil {
		return err
	}
	now := g.now()
	return g.withTaskImmediateTransaction(ctx, "begin goal compaction cancel", func(exec taskSQLExecutor) error {
		if err := validateGoalRunWorkspace(ctx, exec, req.Key); err != nil {
			return err
		}
		checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.Key)
		if err != nil {
			return err
		}
		if checkpoint.ControlEpoch != req.ExpectedControlEpoch ||
			checkpoint.BindingEpoch != req.ExpectedBindingEpoch ||
			checkpoint.Phase != goalCheckpointPhaseCompacting ||
			checkpoint.TaskRunID != strings.TrimSpace(req.TaskRunID) ||
			checkpoint.QueueEntryID != strings.TrimSpace(req.QueueEntryID) ||
			checkpoint.PromptID != strings.TrimSpace(req.PromptID) {
			return goalControlStaleError("compaction cancel owner changed")
		}
		if err := validateActiveGoalPromptBinding(
			ctx,
			exec,
			req.Key,
			checkpoint.BindingHandle,
			req.ExpectedBindingEpoch,
			checkpoint.SessionID,
		); err != nil {
			return err
		}
		if checkpoint.CompactionCancel != nil {
			if checkpoint.CompactionCancel.PromptID == strings.TrimSpace(req.PromptID) &&
				checkpoint.CompactionCancel.Cause == strings.TrimSpace(req.Cause) {
				return nil
			}
			return goalControlStaleError("different compaction cancel intent already exists")
		}
		affected, err := sqlcgen.New(exec).BeginGoalCompactionCancel(ctx, sqlcgen.BeginGoalCompactionCancelParams{
			CompactionCancelPromptID:    goalNullableString(req.PromptID),
			CompactionCancelCause:       goalNullableString(req.Cause),
			CompactionCancelRequestedAt: store.FormatTimestamp(now), UpdatedAt: store.FormatTimestamp(now),
			LoopRunID: string(req.Key.LoopRunID), Generation: int64(req.Key.Generation),
			NodeID: string(req.Key.NodeID), ItemIndex: int64(req.Key.ItemIndex),
			ControlEpoch: req.ExpectedControlEpoch, BindingEpoch: goalNullableInt64(&req.ExpectedBindingEpoch),
			TaskRunID: goalNullableString(req.TaskRunID), QueueEntryID: goalNullableString(req.QueueEntryID),
			PromptID: goalNullableString(req.PromptID),
		})
		if err != nil {
			return fmt.Errorf("store: persist goal compaction cancel intent: %w", err)
		}
		return requireGoalAffectedCount(affected, "begin goal compaction cancel")
	})
}

func validateReportIntentRequest(req goal.RecordReportIntentRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	status := strings.TrimSpace(req.Status)
	if req.ExpectedControlEpoch < 1 || req.ExpectedBindingEpoch < 1 ||
		strings.TrimSpace(req.PromptID) == "" ||
		(status != goalReportStatusComplete && status != goalReportStatusBlocked) ||
		strings.TrimSpace(req.ActorKind) == "" || strings.TrimSpace(req.ActorID) == "" {
		return fmt.Errorf("%w: goal report intent is invalid", looppkg.ErrValidation)
	}
	if status == goalReportStatusBlocked && strings.TrimSpace(req.EvidenceRef) == "" {
		return fmt.Errorf("%w: blocked goal report requires evidence", looppkg.ErrValidation)
	}
	return nil
}

func reportIntentMatchesRequest(intent goal.ReportIntent, req goal.RecordReportIntentRequest) bool {
	return intent.PromptID == strings.TrimSpace(req.PromptID) &&
		intent.Status == strings.TrimSpace(req.Status) &&
		intent.EvidenceRef == strings.TrimSpace(req.EvidenceRef) &&
		intent.BindingEpoch == req.ExpectedBindingEpoch &&
		intent.ActorKind == strings.TrimSpace(req.ActorKind) &&
		intent.ActorID == strings.TrimSpace(req.ActorID)
}

func validateCompactionCancelRequest(req goal.BeginCompactionCancelRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if req.ExpectedControlEpoch < 1 || req.ExpectedBindingEpoch < 1 ||
		strings.TrimSpace(req.TaskRunID) == "" || strings.TrimSpace(req.QueueEntryID) == "" ||
		strings.TrimSpace(req.PromptID) == "" || strings.TrimSpace(req.Cause) != "timeout" {
		return fmt.Errorf("%w: goal compaction cancel intent is invalid", looppkg.ErrValidation)
	}
	return nil
}
