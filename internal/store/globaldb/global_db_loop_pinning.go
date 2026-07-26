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
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

type activeGateCriterionPayload struct {
	ID      string `json:"id"`
	Type    string `json:"type,omitempty"`
	Outcome string `json:"outcome,omitempty"`
}

func upsertLoopDefinitionSnapshot(
	ctx context.Context,
	exec taskSQLExecutor,
	run looppkg.Run,
	now time.Time,
) error {
	usedAt := now.UTC()
	if usedAt.IsZero() {
		usedAt = run.StartedAt.UTC()
	}
	if usedAt.IsZero() {
		usedAt = run.CreatedAt.UTC()
	}
	affected, err := sqlcgen.New(exec).UpsertLoopDefinitionSnapshot(ctx, sqlcgen.UpsertLoopDefinitionSnapshotParams{
		WorkspaceID: string(run.WorkspaceID), DefinitionDigest: run.DefinitionDigest,
		DefinitionVersion: int64(run.DefinitionVersion), DefinitionJson: string(run.DefinitionSnapshot),
		ByteSize: int64(len(run.DefinitionSnapshot)), CreatedAt: store.FormatTimestamp(usedAt),
		LastUsedAt: store.FormatTimestamp(usedAt),
	})
	if err != nil {
		return fmt.Errorf("store: upsert loop definition snapshot %q: %w", run.DefinitionDigest, err)
	}
	if affected != 1 {
		return fmt.Errorf(
			"%w: loop definition snapshot %q already has different content",
			looppkg.ErrValidation,
			run.DefinitionDigest,
		)
	}
	return nil
}

func (g *LoopRepo) GetLoopDefinitionSnapshot(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	digest string,
) (looppkg.DefinitionSnapshot, error) {
	if err := g.checkReady(ctx, "get loop definition snapshot"); err != nil {
		return looppkg.DefinitionSnapshot{}, err
	}
	trimmedDigest := strings.TrimSpace(digest)
	if strings.TrimSpace(string(ws)) == "" {
		return looppkg.DefinitionSnapshot{}, fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	if trimmedDigest == "" {
		return looppkg.DefinitionSnapshot{}, fmt.Errorf("%w: definition_digest is required", looppkg.ErrValidation)
	}
	row, err := g.queries.GetLoopDefinitionSnapshot(ctx, sqlcgen.GetLoopDefinitionSnapshotParams{
		WorkspaceID: string(ws), DefinitionDigest: trimmedDigest,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return looppkg.DefinitionSnapshot{}, looppkg.ErrRunNotFound
		}
		return looppkg.DefinitionSnapshot{}, fmt.Errorf("store: scan loop definition snapshot: %w", err)
	}
	createdAt, err := parseLoopRunTimestamp(row.CreatedAt)
	if err != nil {
		return looppkg.DefinitionSnapshot{}, fmt.Errorf("store: parse snapshot created_at: %w", err)
	}
	lastUsedAt, err := parseLoopRunTimestamp(row.LastUsedAt)
	if err != nil {
		return looppkg.DefinitionSnapshot{}, fmt.Errorf("store: parse snapshot last_used_at: %w", err)
	}
	snapshot := looppkg.DefinitionSnapshot{
		WorkspaceID: looppkg.WorkspaceID(row.WorkspaceID), Digest: row.DefinitionDigest,
		Version: int(row.DefinitionVersion), Definition: json.RawMessage(row.DefinitionJson),
		ByteSize: int(row.ByteSize), CreatedAt: createdAt, LastUsedAt: lastUsedAt,
	}
	if !json.Valid(snapshot.Definition) {
		return looppkg.DefinitionSnapshot{}, fmt.Errorf(
			"%w: loop definition snapshot %q is invalid JSON",
			looppkg.ErrValidation,
			trimmedDigest,
		)
	}
	return snapshot, nil
}

func activateLoopApprovalWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID looppkg.RunID,
	gateID looppkg.NodeID,
	criteria json.RawMessage,
	budget bool,
) error {
	trimmedGateID := strings.TrimSpace(string(gateID))
	if trimmedGateID == "" {
		return fmt.Errorf("%w: active gate id is required", looppkg.ErrValidation)
	}
	if len(criteria) == 0 {
		criteria = json.RawMessage(`[]`)
	}
	if !json.Valid(criteria) {
		return fmt.Errorf("%w: active human criteria must be valid JSON", looppkg.ErrValidation)
	}
	budgetDelta := 0
	if budget {
		budgetDelta = 1
	}
	affected, err := sqlcgen.New(exec).ActivateLoopApproval(ctx, sqlcgen.ActivateLoopApprovalParams{
		ActiveGateID: trimmedGateID, ActiveHumanCriteriaJson: string(criteria),
		BudgetDelta: int64(budgetDelta), ID: string(runID),
	})
	if err != nil {
		return fmt.Errorf("store: activate loop run %q approval gate: %w", runID, err)
	}
	if affected == 0 {
		return fmt.Errorf("%w: loop run %s", looppkg.ErrRunNotFound, runID)
	}
	return nil
}

func activeHumanCriteriaFromTerminal(terminal *taskpkg.CoordinatorTerminal) (json.RawMessage, error) {
	if terminal == nil || len(terminal.Details) == 0 {
		return json.RawMessage(`[]`), nil
	}
	var verdict struct {
		Criteria []activeGateCriterionPayload `json:"criteria"`
	}
	if err := json.Unmarshal(terminal.Details, &verdict); err != nil {
		return nil, fmt.Errorf("store: decode gate verdict details: %w", err)
	}
	out := make([]activeGateCriterionPayload, 0, len(verdict.Criteria))
	for _, criterion := range verdict.Criteria {
		if strings.TrimSpace(criterion.ID) == "" || criterion.Type != "human" {
			continue
		}
		if criterion.Outcome != "awaiting_approval" {
			continue
		}
		out = append(out, activeGateCriterionPayload{
			ID:      strings.TrimSpace(criterion.ID),
			Type:    criterion.Type,
			Outcome: criterion.Outcome,
		})
	}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, fmt.Errorf("store: marshal active human criteria: %w", err)
	}
	return json.RawMessage(data), nil
}

func (g *LoopRepo) RecordLoopGateDecisions(
	ctx context.Context,
	records []looppkg.GateDecisionRecord,
) error {
	if err := g.checkReady(ctx, "record loop gate decisions"); err != nil {
		return err
	}
	normalized, err := normalizeLoopGateDecisionRecords(records, g.now())
	if err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, "record loop gate decisions", func(exec taskSQLExecutor) error {
		for _, record := range normalized {
			if err := insertLoopGateDecision(ctx, exec, record); err != nil {
				return err
			}
		}
		return nil
	})
}

func insertLoopGateDecision(
	ctx context.Context,
	exec taskSQLExecutor,
	record looppkg.GateDecisionRecord,
) error {
	err := sqlcgen.New(exec).UpsertLoopGateDecision(ctx, sqlcgen.UpsertLoopGateDecisionParams{
		WorkspaceID: string(record.WorkspaceID), LoopRunID: string(record.RunID),
		Generation: int64(record.Generation), GateID: string(record.GateID), CriterionID: record.CriterionID,
		Decision: string(record.Decision), ActorKind: string(record.Actor.Actor.Kind), ActorRef: record.Actor.Actor.Ref,
		OriginKind: string(record.Actor.Origin.Kind), OriginRef: record.Actor.Origin.Ref,
		Note: record.Note, DecidedAt: store.FormatTimestamp(record.DecidedAt),
	})
	if err != nil {
		return fmt.Errorf("store: insert loop gate decision %q/%q: %w", record.GateID, record.CriterionID, err)
	}
	return nil
}

func (g *LoopRepo) ListLoopGateDecisions(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	runID looppkg.RunID,
	generation int,
	gateID looppkg.NodeID,
) (decisions map[string]gate.HumanDecision, err error) {
	if err := g.checkReady(ctx, "list loop gate decisions"); err != nil {
		return nil, err
	}
	trimmedGateID := strings.TrimSpace(string(gateID))
	if strings.TrimSpace(string(ws)) == "" {
		return nil, fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
	}
	if strings.TrimSpace(string(runID)) == "" {
		return nil, fmt.Errorf("%w: run_id is required", looppkg.ErrValidation)
	}
	if generation < 0 {
		return nil, fmt.Errorf("%w: generation must be non-negative", looppkg.ErrValidation)
	}
	if trimmedGateID == "" {
		return nil, fmt.Errorf("%w: gate_id is required", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListLoopGateDecisions(ctx, sqlcgen.ListLoopGateDecisionsParams{
		WorkspaceID: string(ws), LoopRunID: string(runID), Generation: int64(generation), GateID: trimmedGateID,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop gate decisions: %w", err)
	}
	decisions = map[string]gate.HumanDecision{}
	for _, row := range rows {
		decisions[row.CriterionID] = gate.HumanDecision{
			Decision: gate.HumanDecisionKind(row.Decision),
			Actor: taskpkg.ActorContext{
				Actor: taskpkg.ActorIdentity{
					Kind: taskpkg.ActorKind(row.ActorKind),
					Ref:  row.ActorRef,
				},
				Origin: taskpkg.Origin{
					Kind: taskpkg.OriginKind(row.OriginKind),
					Ref:  row.OriginRef,
				},
				Scope: taskpkg.CallerScope{
					WorkspaceID: strings.TrimSpace(string(ws)),
					Operator:    true,
				},
			},
			Note: row.Note,
		}
	}
	return decisions, nil
}

func normalizeLoopGateDecisionRecords(
	records []looppkg.GateDecisionRecord,
	defaultDecidedAt time.Time,
) ([]looppkg.GateDecisionRecord, error) {
	normalized := make([]looppkg.GateDecisionRecord, 0, len(records))
	for _, record := range records {
		next := record
		next.WorkspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(record.WorkspaceID)))
		next.RunID = looppkg.RunID(strings.TrimSpace(string(record.RunID)))
		next.GateID = looppkg.NodeID(strings.TrimSpace(string(record.GateID)))
		next.CriterionID = strings.TrimSpace(record.CriterionID)
		next.Note = strings.TrimSpace(record.Note)
		if next.DecidedAt.IsZero() {
			next.DecidedAt = defaultDecidedAt.UTC()
		} else {
			next.DecidedAt = next.DecidedAt.UTC()
		}
		if next.WorkspaceID == "" {
			return nil, fmt.Errorf("%w: workspace_id is required", looppkg.ErrValidation)
		}
		if next.RunID == "" {
			return nil, fmt.Errorf("%w: run_id is required", looppkg.ErrValidation)
		}
		if next.GateID == "" {
			return nil, fmt.Errorf("%w: gate_id is required", looppkg.ErrValidation)
		}
		if next.CriterionID == "" {
			return nil, fmt.Errorf("%w: criterion_id is required", looppkg.ErrValidation)
		}
		if err := validateLoopGateDecision(next.Decision); err != nil {
			return nil, err
		}
		if err := next.Actor.Validate(); err != nil {
			return nil, fmt.Errorf("%w: actor context: %w", looppkg.ErrValidation, err)
		}
		normalized = append(normalized, next)
	}
	return normalized, nil
}

func validateLoopGateDecision(decision looppkg.GateDecision) error {
	switch decision {
	case looppkg.GateDecisionApprove, looppkg.GateDecisionRequestChanges, looppkg.GateDecisionReject:
		return nil
	default:
		return fmt.Errorf("%w: gate decision is invalid: %q", looppkg.ErrValidation, decision)
	}
}
