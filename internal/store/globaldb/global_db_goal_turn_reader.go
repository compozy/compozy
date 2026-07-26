package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ goal.TurnReader = (*GoalRepo)(nil)

const goalTurnSelectColumns = `
	seq, generation, node_id, item_index, turn, session_id, binding_handle,
	binding_epoch, prompt_id, prompt_attempt, usage_base_tokens, result_status,
	stop_reason, reason_code, verdict_outcome, blocking_json, evidence_ref,
	prompt_ref, tokens_used, actor_kind, actor_id, started_at, ended_at`

// ListGoalTurns returns one stable run-wide sequence page.
func (g *GoalRepo) ListGoalTurns(
	ctx context.Context,
	query goal.TurnQuery,
) (page goal.TurnPage, err error) {
	if err := g.checkReady(ctx, "list goal turns"); err != nil {
		return goal.TurnPage{}, err
	}
	if err := query.Validate(); err != nil {
		return goal.TurnPage{}, err
	}
	if err := validateGoalRunWorkspace(ctx, g.db, goal.TurnKey{
		WorkspaceID: query.WorkspaceID,
		LoopRunID:   query.LoopRunID,
		Generation:  1,
		NodeID:      "turn-reader",
	}); err != nil {
		return goal.TurnPage{}, err
	}

	limit := query.Limit
	if limit == 0 {
		limit = 50
	}
	statement, args := goalTurnListQuery(query, limit+1)
	// dynamic-sql: optional node/item filters and cursor pagination change the predicate and argument shape.
	rows, err := g.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return goal.TurnPage{}, fmt.Errorf("store: list goal turns for run %q: %w", query.LoopRunID, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close goal turns rows: %w", closeErr))
		}
	}()

	turns := make([]goal.Turn, 0, limit)
	for rows.Next() {
		turn, scanErr := scanGoalTurn(rows, query.WorkspaceID, query.LoopRunID)
		if scanErr != nil {
			return goal.TurnPage{}, fmt.Errorf("store: scan goal turn for run %q: %w", query.LoopRunID, scanErr)
		}
		turns = append(turns, turn)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return goal.TurnPage{}, fmt.Errorf("store: iterate goal turns for run %q: %w", query.LoopRunID, rowsErr)
	}

	page.Turns = turns
	if len(page.Turns) > limit {
		page.Turns = page.Turns[:limit]
		next := page.Turns[len(page.Turns)-1].Seq
		page.NextAfterSeq = &next
	}
	return page, nil
}

// GetGoalTurnByPromptID returns the exact workspace-owned turn identity.
func (g *GoalRepo) GetGoalTurnByPromptID(
	ctx context.Context,
	key goal.TurnKey,
	promptID string,
) (goal.Turn, error) {
	if err := g.checkReady(ctx, "get goal turn by prompt ID"); err != nil {
		return goal.Turn{}, err
	}
	if err := key.Validate(); err != nil {
		return goal.Turn{}, err
	}
	normalizedPromptID := strings.TrimSpace(promptID)
	if normalizedPromptID == "" {
		return goal.Turn{}, fmt.Errorf("%w: goal prompt_id is required", looppkg.ErrValidation)
	}
	if err := validateGoalRunWorkspace(ctx, g.db, key); err != nil {
		return goal.Turn{}, err
	}

	row, err := sqlcgen.New(g.db).GetGoalTurnByPromptID(ctx, sqlcgen.GetGoalTurnByPromptIDParams{
		LoopRunID: string(key.LoopRunID), Generation: int64(key.Generation), NodeID: string(key.NodeID),
		ItemIndex: int64(key.ItemIndex), PromptID: normalizedPromptID, WorkspaceID: string(key.WorkspaceID),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return goal.Turn{}, fmt.Errorf("%w: %s", goal.ErrTurnNotFound, normalizedPromptID)
	}
	if err != nil {
		return goal.Turn{}, fmt.Errorf("store: get goal turn %q: %w", normalizedPromptID, err)
	}
	turn, err := goalTurnFromGenerated(row, key.WorkspaceID, key.LoopRunID)
	if err != nil {
		return goal.Turn{}, fmt.Errorf("store: decode goal turn %q: %w", normalizedPromptID, err)
	}
	return turn, nil
}

func goalTurnListQuery(query goal.TurnQuery, limit int) (string, []any) {
	var statement strings.Builder
	statement.WriteString(`SELECT `)
	statement.WriteString(goalTurnSelectColumns)
	statement.WriteString(` FROM loop_goal_turns WHERE loop_run_id = ? AND seq > ?
		AND EXISTS (
			SELECT 1 FROM loop_runs
			WHERE loop_runs.id = loop_goal_turns.loop_run_id AND loop_runs.workspace_id = ?
		)`)
	args := []any{string(query.LoopRunID), query.AfterSeq, string(query.WorkspaceID)}
	if strings.TrimSpace(string(query.NodeID)) != "" {
		statement.WriteString(` AND node_id = ?`)
		args = append(args, string(query.NodeID))
	}
	if query.ItemIndex != nil {
		statement.WriteString(` AND item_index = ?`)
		args = append(args, *query.ItemIndex)
	}
	statement.WriteString(` ORDER BY seq ASC LIMIT ?`)
	args = append(args, limit)
	return statement.String(), args
}

func scanGoalTurn(
	scanner rowScanner,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) (goal.Turn, error) {
	var turn goal.Turn
	var fields goalTurnScanFields
	if err := scanner.Scan(
		&turn.Seq,
		&turn.Key.Generation,
		&turn.Key.NodeID,
		&turn.Key.ItemIndex,
		&turn.Turn,
		&turn.SessionID,
		&turn.BindingHandle,
		&turn.BindingEpoch,
		&turn.PromptID,
		&turn.PromptAttempt,
		&turn.UsageBaseTokens,
		&fields.resultStatus,
		&fields.stopReason,
		&fields.reasonCode,
		&fields.verdictOutcome,
		&fields.blockingJSON,
		&fields.evidenceRef,
		&fields.promptRef,
		&fields.tokensUsed,
		&turn.ActorKind,
		&turn.ActorID,
		&fields.startedAtRaw,
		&fields.endedAtRaw,
	); err != nil {
		return goal.Turn{}, err
	}
	turn.Key.WorkspaceID = workspaceID
	turn.Key.LoopRunID = runID
	if err := fields.apply(&turn); err != nil {
		return goal.Turn{}, err
	}
	return turn, nil
}

type goalTurnScanFields struct {
	resultStatus   sql.NullString
	stopReason     sql.NullString
	reasonCode     sql.NullString
	verdictOutcome sql.NullString
	blockingJSON   string
	evidenceRef    sql.NullString
	promptRef      sql.NullString
	tokensUsed     sql.NullInt64
	startedAtRaw   any
	endedAtRaw     any
}

func (fields *goalTurnScanFields) apply(turn *goal.Turn) error {
	if fields.resultStatus.Valid {
		value := fields.resultStatus.String
		turn.ResultStatus = &value
	}
	if fields.stopReason.Valid {
		value := looppkg.ActionStopReason(fields.stopReason.String)
		turn.StopReason = &value
	}
	if fields.reasonCode.Valid {
		value := looppkg.ReasonCode(fields.reasonCode.String)
		turn.ReasonCode = &value
	}
	if fields.verdictOutcome.Valid {
		value := gate.VerdictOutcome(fields.verdictOutcome.String)
		turn.VerdictOutcome = &value
	}
	if err := json.Unmarshal([]byte(fields.blockingJSON), &turn.BlockingIssues); err != nil {
		return fmt.Errorf("store: decode goal turn blocking issues: %w", err)
	}
	if turn.BlockingIssues == nil {
		turn.BlockingIssues = []gate.BlockingIssue{}
	}
	if fields.evidenceRef.Valid {
		value := fields.evidenceRef.String
		turn.EvidenceRef = &value
	}
	if fields.promptRef.Valid {
		value := fields.promptRef.String
		turn.PromptRef = &value
	}
	if fields.tokensUsed.Valid {
		value := fields.tokensUsed.Int64
		turn.TokensUsed = &value
	}
	var err error
	turn.StartedAt, err = parseGoalTimestampValue(fields.startedAtRaw, "turn started_at")
	if err != nil {
		return err
	}
	turn.EndedAt, err = parseOptionalGoalTimestampValue(fields.endedAtRaw, "turn ended_at")
	if err != nil {
		return err
	}
	return nil
}
