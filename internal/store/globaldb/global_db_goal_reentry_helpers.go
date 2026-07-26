package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

type goalReentryRow struct {
	state         looppkg.GoalControlState
	outputStatus  string
	outputTaskRun string
	grantID       sql.NullInt64
	grantKind     sql.NullString
	grantCause    sql.NullString
	grantTurn     sql.NullInt64
	grantScope    sql.NullString
	grantConsumed sql.NullInt64
}

func goalReentryStillAwaiting(current goalReentryRow, expected looppkg.GoalControlState) bool {
	return current.state.ControlEpoch == expected.ControlEpoch &&
		current.state.RunStatus == expected.RunStatus &&
		current.outputStatus == looppkg.GenerationOutputStatusAwaitingGoal &&
		strings.TrimSpace(current.outputTaskRun) == strings.TrimSpace(expected.TaskRunID)
}

func loadAwaitingGoalControlWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (goalReentryRow, bool, error) {
	generated, err := sqlcgen.New(exec).GetAwaitingGoalControl(ctx, sqlcgen.GetAwaitingGoalControlParams{
		LoopRunID: string(runID), WorkspaceID: string(workspaceID),
		OutputStatus: looppkg.GenerationOutputStatusAwaitingGoal,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goalReentryRow{}, false, nil
	}
	if err != nil {
		return goalReentryRow{}, false, fmt.Errorf("store: load awaiting Goal control: %w", err)
	}
	row := goalReentryRowFromAwaiting(generated, workspaceID, runID)
	if err := finalizeGoalReentryState(&row); err != nil {
		return goalReentryRow{}, false, err
	}
	return row, true, nil
}

func loadGoalReentryRowWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	state looppkg.GoalControlState,
) (goalReentryRow, error) {
	generated, err := sqlcgen.New(exec).GetGoalReentryState(ctx, sqlcgen.GetGoalReentryStateParams{
		LoopRunID: string(state.LoopRunID), WorkspaceID: string(state.WorkspaceID),
		Generation: int64(state.Generation), NodeID: string(state.NodeID), ItemIndex: int64(state.ItemIndex),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goalReentryRow{}, goalReentryStaleError("Goal checkpoint no longer exists")
	}
	if err != nil {
		return goalReentryRow{}, fmt.Errorf("store: load Goal reentry checkpoint: %w", err)
	}
	return goalReentryRowFromState(generated, state.WorkspaceID, state.LoopRunID), nil
}

func goalReentryRowFromAwaiting(
	row sqlcgen.GetAwaitingGoalControlRow,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) goalReentryRow {
	return newGoalReentryRow(workspaceID, runID, row.Generation, row.NodeID, row.ItemIndex,
		row.ControlEpoch, row.GoalStatus, row.ControlCause, row.TurnsUsed, row.TurnLimit,
		row.TaskRunID, row.QueueEntryID, row.PromptID, row.PromptKind, row.OpenTurn, row.RunStatus,
		row.ActiveGateID, row.OutputStatus, row.OutputTaskRun, row.ControlGrantID,
		row.ControlGrantKind, row.ControlGrantCause, row.ControlGrantTurn, row.ControlGrantScope,
		row.ControlGrantConsumed)
}

func goalReentryRowFromState(
	row sqlcgen.GetGoalReentryStateRow,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) goalReentryRow {
	return newGoalReentryRow(workspaceID, runID, row.Generation, row.NodeID, row.ItemIndex,
		row.ControlEpoch, row.GoalStatus, row.ControlCause, row.TurnsUsed, row.TurnLimit,
		row.TaskRunID, row.QueueEntryID, row.PromptID, row.PromptKind, row.OpenTurn, row.RunStatus,
		row.ActiveGateID, row.OutputStatus, row.OutputTaskRun, row.ControlGrantID,
		row.ControlGrantKind, row.ControlGrantCause, row.ControlGrantTurn, row.ControlGrantScope,
		row.ControlGrantConsumed)
}

func newGoalReentryRow(
	workspaceID looppkg.WorkspaceID, runID looppkg.RunID,
	generation int64, nodeID string, itemIndex int64, controlEpoch int64,
	goalStatus string, cause string, turnsUsed int64, turnLimit int64,
	taskRunID string, queueEntryID string, promptID string, promptKind string,
	openTurn bool, runStatus string, gateID string, outputStatus string, outputTaskRun string,
	grantID int64, grantKind sql.NullString, grantCause sql.NullString, grantTurn sql.NullInt64,
	grantScope sql.NullString, grantConsumed int64,
) goalReentryRow {
	return goalReentryRow{
		state: looppkg.GoalControlState{
			WorkspaceID: workspaceID, LoopRunID: runID, Generation: int(generation),
			NodeID: looppkg.NodeID(nodeID), ItemIndex: int(itemIndex), ControlEpoch: controlEpoch,
			GoalStatus: goalStatus, Cause: looppkg.ReasonCode(cause), TurnsUsed: int(turnsUsed),
			TurnLimit: int(turnLimit), TaskRunID: taskRunID, QueueEntryID: queueEntryID,
			PromptID: promptID, PromptKind: promptKind, OpenTurn: openTurn,
			RunStatus: looppkg.Status(runStatus), GateID: looppkg.NodeID(gateID),
		},
		outputStatus: outputStatus, outputTaskRun: outputTaskRun,
		grantID: sql.NullInt64{Int64: grantID, Valid: true}, grantKind: grantKind,
		grantCause: grantCause, grantTurn: grantTurn, grantScope: grantScope,
		grantConsumed: sql.NullInt64{Int64: grantConsumed, Valid: true},
	}
}

func finalizeGoalReentryState(row *goalReentryRow) error {
	if row == nil {
		return fmt.Errorf("%w: Goal reentry row is required", looppkg.ErrValidation)
	}
	row.state.Cause = goalControlCause(row.state)
	if row.state.Cause == "" || strings.TrimSpace(row.state.TaskRunID) == "" ||
		strings.TrimSpace(row.outputTaskRun) != strings.TrimSpace(row.state.TaskRunID) {
		return goalReentryStaleError("Goal reentry correlation is incomplete")
	}
	return nil
}

func goalControlCause(state looppkg.GoalControlState) looppkg.ReasonCode {
	if strings.TrimSpace(string(state.Cause)) != "" {
		return looppkg.ReasonCode(strings.TrimSpace(string(state.Cause)))
	}
	if state.RunStatus == looppkg.StatusPaused {
		return "operator_resume"
	}
	if state.RunStatus != looppkg.StatusNeedsApproval {
		return ""
	}
	prefix := fmt.Sprintf(
		"goal:%s:%d:%d:",
		strings.TrimSpace(string(state.NodeID)),
		state.Generation,
		state.ItemIndex,
	)
	gateID := strings.TrimSpace(string(state.GateID))
	if !strings.HasPrefix(gateID, prefix) {
		return ""
	}
	return looppkg.ReasonCode(strings.TrimSpace(strings.TrimPrefix(gateID, prefix)))
}

func validateGoalReactivationRequest(req looppkg.GoalReactivationRequest) error {
	if !goalReactivationStateValid(req.State) {
		return fmt.Errorf("%w: Goal reactivation state is incomplete", looppkg.ErrValidation)
	}
	if err := req.Actor.Validate(); err != nil {
		return fmt.Errorf("%w: Goal reactivation actor: %w", looppkg.ErrValidation, err)
	}
	if !goalGrantScopeValid(req) {
		return fmt.Errorf("%w: Goal grant kind/scope is invalid", looppkg.ErrValidation)
	}
	return nil
}

func goalReactivationStateValid(state looppkg.GoalControlState) bool {
	return strings.TrimSpace(string(state.WorkspaceID)) != "" &&
		strings.TrimSpace(string(state.LoopRunID)) != "" &&
		state.Generation >= 1 &&
		strings.TrimSpace(string(state.NodeID)) != "" &&
		state.ItemIndex >= 0 &&
		state.ControlEpoch >= 1 &&
		strings.TrimSpace(state.TaskRunID) != "" &&
		state.Cause != ""
}

func goalGrantScopeValid(req looppkg.GoalReactivationRequest) bool {
	switch req.Kind {
	case looppkg.GoalGrantPlainResume:
		return req.Scope == looppkg.GoalGrantScopeReactivate && req.TurnIncrement == 0
	case looppkg.GoalGrantTurnExtension:
		return req.Scope == looppkg.GoalGrantScopeTurnLimit && req.TurnIncrement > 0
	case looppkg.GoalGrantBudget:
		return (req.Scope == looppkg.GoalGrantScopeSettleCurrent ||
			req.Scope == looppkg.GoalGrantScopeWorkAndSettle) && req.TurnIncrement == 0 &&
			((req.Scope == looppkg.GoalGrantScopeSettleCurrent) == req.State.OpenTurn)
	case looppkg.GoalGrantReseed:
		return req.Scope == looppkg.GoalGrantScopeRotateBinding && req.TurnIncrement == 0
	default:
		return false
	}
}

func goalReentryStaleError(detail string) error {
	return &looppkg.ReasonError{
		Code: looppkg.ReasonCodeGoalControlStale,
		Err:  fmt.Errorf("%w: %s", looppkg.ErrTransitionConflict, detail),
	}
}

func goalSuccessorMetadata(raw json.RawMessage, controlEpoch int64) (json.RawMessage, error) {
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return nil, fmt.Errorf("store: decode Goal worker metadata: %w", err)
	}
	metadata["goal_segment_epoch"] = controlEpoch
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("store: encode Goal worker metadata: %w", err)
	}
	return encoded, nil
}

func goalReentryGrantConsumed(kind looppkg.GoalGrantKind) bool {
	return kind == looppkg.GoalGrantTurnExtension || kind == looppkg.GoalGrantPlainResume
}

func goalReentryGrantTurn(req looppkg.GoalReactivationRequest) int {
	turn := req.State.TurnsUsed
	if req.Kind == looppkg.GoalGrantBudget && req.Scope == looppkg.GoalGrantScopeWorkAndSettle {
		turn++
	}
	return turn
}

func normalizeGoalReentryDecisions(
	records []looppkg.GateDecisionRecord,
	now func() time.Time,
) ([]looppkg.GateDecisionRecord, error) {
	if len(records) == 0 {
		return nil, nil
	}
	return normalizeLoopGateDecisionRecords(records, now())
}
