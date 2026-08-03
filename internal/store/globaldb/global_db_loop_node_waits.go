package globaldb

import (
	"context"
	"encoding/json"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ListNodeWaits returns workspace-scoped durable waits for one run.
func (g *LoopRepo) ListNodeWaits(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runID looppkg.RunID,
) ([]looppkg.NodeWait, error) {
	if err := g.checkReady(ctx, "list node waits"); err != nil {
		return nil, err
	}
	rows, err := g.queries.ListLoopNodeWaits(ctx, sqlcgen.ListLoopNodeWaitsParams{
		WorkspaceID: string(workspaceID),
		LoopRunID:   string(runID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list loop node waits: %w", err)
	}
	waits := make([]looppkg.NodeWait, 0, len(rows))
	for _, row := range rows {
		wait, mapErr := loopNodeWaitFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		waits = append(waits, wait)
	}
	return waits, nil
}

func loopNodeWaitFromGenerated(row sqlcgen.LoopNodeWait) (looppkg.NodeWait, error) {
	claimState := looppkg.WaitClaimState(row.ClaimState)
	if !claimState.Valid() {
		return looppkg.NodeWait{}, fmt.Errorf(
			"store: invalid loop node wait claim state %q: %w",
			row.ClaimState,
			looppkg.ErrValidation,
		)
	}
	wait := looppkg.NodeWait{
		LoopRunID:         looppkg.RunID(row.LoopRunID),
		Generation:        int(row.Generation),
		NodeID:            looppkg.NodeID(row.NodeID),
		ItemIndex:         int(row.ItemIndex),
		Kind:              row.Kind,
		ResumeAt:          loopTimePointer(row.ResumeAt),
		NextEscalationAt:  loopTimePointer(row.NextEscalationAt),
		EscalationCursor:  int(row.EscalationCursor),
		ClaimState:        claimState,
		ClaimedByKind:     row.ClaimedByKind.String,
		ClaimedByID:       row.ClaimedByID.String,
		ClaimedAt:         loopTimePointer(row.ClaimedAt),
		AdmissionFailures: int(row.AdmissionFailures),
		IssuedEpoch:       row.IssuedEpoch,
		CreatedAt:         row.CreatedAt.UTC(),
	}
	if row.ExpectJson.Valid {
		wait.Expect = json.RawMessage(row.ExpectJson.String)
	}
	if row.AheadPayloadJson.Valid {
		wait.AheadPayload = json.RawMessage(row.AheadPayloadJson.String)
	}
	return wait, nil
}
