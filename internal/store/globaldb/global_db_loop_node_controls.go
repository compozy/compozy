package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ListNodeControls returns workspace-scoped control rows for one run.
func (g *LoopRepo) ListNodeControls(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]looppkg.NodeControl, error) {
	if err := g.checkReady(ctx, "list node controls"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListLoopNodeControls(ctx, sqlcgen.ListLoopNodeControlsParams{
		WorkspaceID: string(workspaceID),
		LoopRunID:   string(runID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop node controls: %w", err)
	}
	controls := make([]looppkg.NodeControl, 0, len(rows))
	for _, row := range rows {
		control, mapErr := loopNodeControlFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		controls = append(controls, control)
	}
	return controls, nil
}

func loopNodeControlFromGenerated(row sqlcgen.LoopNodeControl) (looppkg.NodeControl, error) {
	cancelState := looppkg.CancelState(row.CancelState)
	if !cancelState.Valid() {
		return looppkg.NodeControl{}, fmt.Errorf(
			"store: invalid loop node cancel state %q: %w",
			row.CancelState,
			looppkg.ErrValidation,
		)
	}
	control := looppkg.NodeControl{
		LoopRunID:               looppkg.RunID(row.LoopRunID),
		NodeID:                  looppkg.NodeID(row.NodeID),
		Paused:                  row.Paused != 0,
		Quarantined:             row.Quarantined != 0,
		AttentionFlag:           row.AttentionFlag,
		AttentionReason:         row.AttentionReason,
		AttentionProducerNodeID: row.AttentionProducerNodeID,
		CancelState:             cancelState,
		LastEvidenceAt:          loopTimePointer(row.LastEvidenceAt),
		DeathResumeStreak:       int(row.DeathResumeStreak),
		Revision:                row.Revision,
		UpdatedAt:               row.UpdatedAt.UTC(),
		QuarantinedAt:           loopTimePointer(row.QuarantinedAt),
	}
	if err := json.Unmarshal([]byte(row.GateRevisionsJson), &control.GateRevisions); err != nil {
		return looppkg.NodeControl{}, fmt.Errorf("store: decode gate revision counters: %w", err)
	}
	for itemIndex, revision := range control.GateRevisions {
		if itemIndex < 0 || revision < 1 {
			return looppkg.NodeControl{}, fmt.Errorf(
				"store: invalid gate revision counter for item %d: %w",
				itemIndex,
				looppkg.ErrValidation,
			)
		}
	}
	if row.QuarantineEntryJson.Valid {
		control.QuarantineEntry = json.RawMessage(row.QuarantineEntryJson.String)
	}
	control.PauseProvenance = controlProvenance(
		row.PauseActorKind.String,
		row.PauseActorID.String,
		row.PauseReason.String,
		row.PauseRuleID.String,
		row.PauseRequestedAt,
	)
	control.CancelProvenance = controlProvenance(
		row.CancelActorKind.String,
		row.CancelActorID.String,
		row.CancelReason.String,
		"",
		row.CancelRequestedAt,
	)
	return control, nil
}

func controlProvenance(actorKind, actorID, reason, ruleID string, requestedAt sql.NullTime) *looppkg.ControlProvenance {
	if !requestedAt.Valid && actorKind == "" && actorID == "" && reason == "" && ruleID == "" {
		return nil
	}
	return &looppkg.ControlProvenance{
		ActorKind:   actorKind,
		ActorID:     actorID,
		Reason:      reason,
		RuleID:      ruleID,
		RequestedAt: requestedAt.Time.UTC(),
	}
}
