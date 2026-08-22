package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// ListLoopRosterOutputs returns every generation output for one workspace-owned run.
func (g *LoopRepo) ListLoopRosterOutputs(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]looppkg.GenerationOutput, error) {
	if err := g.checkReady(ctx, "list loop roster outputs"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(workspaceID)) == "" || strings.TrimSpace(string(runID)) == "" {
		return nil, fmt.Errorf("%w: roster output scope is invalid", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListLoopRosterOutputs(ctx, sqlcgen.ListLoopRosterOutputsParams{
		LoopRunID: string(runID), WorkspaceID: string(workspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop roster outputs: %w", err)
	}
	outputs := make([]looppkg.GenerationOutput, 0, len(rows))
	for _, row := range rows {
		output, mapErr := generationOutputFromGenerated(sqlcgen.ListLoopGenerationOutputsRow{
			Generation: row.Generation, NodeID: row.NodeID, ItemIndex: row.ItemIndex, Status: row.Status,
			OutputID: row.OutputID, ArtifactName: row.ArtifactName,
			OutputRef: row.OutputRef, TaskRunID: row.TaskRunID, ChildLoopRunID: row.ChildLoopRunID,
			ResolvedRuntimeJson: row.ResolvedRuntimeJson, Attempt: row.Attempt, NextAttemptAt: row.NextAttemptAt,
			FirstScheduledAt: row.FirstScheduledAt, Epoch: row.Epoch, SessionID: row.SessionID,
		})
		if mapErr != nil {
			return nil, mapErr
		}
		output.TaskRunStatus = taskpkg.ParseRunStatus(row.TaskRunStatus).Normalize()
		output.TaskRunTokensUsed = row.TaskRunTokensUsed
		outputs = append(outputs, output)
	}
	return outputs, nil
}

// ListLoopRosterRouteCauses returns every durable route decision for one workspace-owned run.
func (g *LoopRepo) ListLoopRosterRouteCauses(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]looppkg.RouteCause, error) {
	if err := g.checkReady(ctx, "list loop roster route causes"); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(workspaceID)) == "" || strings.TrimSpace(string(runID)) == "" {
		return nil, fmt.Errorf("%w: roster route-cause scope is invalid", looppkg.ErrValidation)
	}
	rows, err := g.queries.ListLoopRosterRouteCauses(ctx, sqlcgen.ListLoopRosterRouteCausesParams{
		LoopRunID: string(runID), WorkspaceID: string(workspaceID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop roster route causes: %w", err)
	}
	causes := make([]looppkg.RouteCause, 0, len(rows))
	for _, row := range rows {
		cause, decodeErr := decodeStoredRouteCause(row.PayloadJson, row.At, 0)
		if decodeErr != nil {
			return nil, decodeErr
		}
		causes = append(causes, cause)
	}
	return causes, nil
}

func decodeStoredRouteCause(raw string, at time.Time, expectedGeneration int64) (looppkg.RouteCause, error) {
	payload, err := decodeStoredRouteEvidence(raw, expectedGeneration)
	if err != nil {
		return looppkg.RouteCause{}, err
	}
	if payload.ItemIndex < 0 || payload.Route == "" || payload.Cause == "" {
		return looppkg.RouteCause{}, fmt.Errorf("%w: persisted route cause is invalid", looppkg.ErrValidation)
	}
	return looppkg.RouteCause{
		Generation: payload.Generation,
		NodeID:     looppkg.NodeID(payload.NodeID), ItemIndex: payload.ItemIndex,
		Route: looppkg.NodeID(payload.Route), Cause: payload.Cause,
		MatchedWhen: payload.MatchedWhen, Default: payload.Default, At: at,
	}, nil
}

type storedRouteEvidence struct {
	Generation  int64  `json:"generation"`
	NodeID      string `json:"node_id"`
	ItemIndex   int    `json:"item_index"`
	Route       string `json:"route"`
	Cause       string `json:"cause"`
	MatchedWhen string `json:"matched_when"`
	Default     bool   `json:"default"`
}

func decodeStoredRouteEvidence(raw string, expectedGeneration int64) (storedRouteEvidence, error) {
	var payload storedRouteEvidence
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return storedRouteEvidence{}, fmt.Errorf("store: decode loop route evidence: %w", err)
	}
	payload.NodeID = strings.TrimSpace(payload.NodeID)
	payload.Route = strings.TrimSpace(payload.Route)
	payload.Cause = strings.TrimSpace(payload.Cause)
	payload.MatchedWhen = strings.TrimSpace(payload.MatchedWhen)
	if payload.Generation < 1 || (expectedGeneration > 0 && payload.Generation != expectedGeneration) ||
		payload.NodeID == "" {
		return storedRouteEvidence{}, fmt.Errorf("%w: persisted route evidence is invalid", looppkg.ErrValidation)
	}
	return payload, nil
}
