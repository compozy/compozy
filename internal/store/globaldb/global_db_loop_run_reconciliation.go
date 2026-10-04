package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ looppkg.ReconciliationStore = (*LoopRepo)(nil)

// NeutralizeLoopRunOrphans removes claim eligibility before daemon recovery starts.
func (g *LoopRepo) NeutralizeLoopRunOrphans(ctx context.Context) (looppkg.SweepReport, error) {
	return g.reconcileLoopRunOrphans(ctx, "neutralize Loop run orphans")
}

// SweepLoopRunOrphans converges terminal execution records during daemon operation.
func (g *LoopRepo) SweepLoopRunOrphans(ctx context.Context) (looppkg.SweepReport, error) {
	return g.reconcileLoopRunOrphans(ctx, "sweep Loop run orphans")
}

func (g *LoopRepo) reconcileLoopRunOrphans(
	ctx context.Context,
	action string,
) (looppkg.SweepReport, error) {
	if err := g.checkReady(ctx, action); err != nil {
		return looppkg.SweepReport{}, err
	}
	candidates, err := g.queries.ListLoopReconciliationCandidates(ctx)
	if err != nil {
		return looppkg.SweepReport{}, fmt.Errorf("store: list Loop reconciliation candidates: %w", err)
	}
	report := looppkg.SweepReport{RunsExamined: len(candidates)}
	for _, runID := range candidates {
		var outcome loopSettlementOutcome
		err := g.withTaskImmediateTransaction(ctx, action, func(exec taskSQLExecutor) error {
			status, err := sqlcgen.New(exec).GetLoopReconciliationStatus(ctx, runID.String)
			cause, reason := looppkg.TerminalCauseRunMissing, runMissingReason
			switch {
			case errors.Is(err, sql.ErrNoRows):
			case err != nil:
				return err
			case !looppkg.Status(status).Terminal():
				return nil
			default:
				cause, err = terminalCauseForLoopStatus(looppkg.Status(status))
				if err != nil {
					return err
				}
				reason = reconciledRunTerminalReason
			}
			outcome, err = settleLoopRunTerminalWithReason(ctx, exec, runID.String, cause, reason)
			return err
		})
		if err != nil {
			return report, err
		}
		report.RecordsSettled += outcome.recordsSettled
		if outcome.recordsSettled > 0 || outcome.result.RunsCanceled > 0 {
			report.OrphansRepaired++
		}
	}
	return report, nil
}

// BackfillLoopProvenance repairs coordinator metadata from relational ownership only.
func (g *LoopRepo) BackfillLoopProvenance(ctx context.Context) (int, error) {
	if err := g.checkReady(ctx, "backfill Loop provenance"); err != nil {
		return 0, err
	}
	records, err := g.queries.ListLoopProvenance(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: list Loop provenance rows: %w", err)
	}
	repaired := 0
	for _, record := range records {
		encoded, err := loopProvenanceMetadata(record.WorkspaceID.String,
			record.LoopRunID.String, record.LoopName, []byte(record.MetadataJson.String))
		if err != nil {
			return repaired, fmt.Errorf("store: decode Loop coordinator %q metadata: %w", record.ID, err)
		}
		if encoded == nil {
			continue
		}
		var changed int64
		err = g.withTaskImmediateTransaction(ctx, "backfill Loop provenance", func(exec taskSQLExecutor) error {
			var updateErr error
			changed, updateErr = sqlcgen.New(exec).UpdateLoopProvenance(ctx, sqlcgen.UpdateLoopProvenanceParams{
				ID: record.ID, Metadata: nullString(string(encoded)), PreviousMetadata: record.MetadataJson,
			})
			return updateErr
		})
		if err != nil {
			return repaired, err
		}
		if changed == 0 {
			return repaired, fmt.Errorf("store: Loop coordinator %q metadata changed during backfill", record.ID)
		}
		repaired++
	}
	return repaired, nil
}

func loopProvenanceMetadata(workspaceID, loopRunID string, loopName sql.NullString, raw []byte) ([]byte, error) {
	metadata := map[string]any{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return nil, err
		}
	}
	changed := setMetadataString(metadata, "loop_run_id", loopRunID)
	changed = setMetadataString(metadata, "workspace_id", workspaceID) || changed
	if loopName.Valid {
		changed = setMetadataString(metadata, "loop_name", loopName.String) || changed
	} else if _, exists := metadata["loop_name"]; exists {
		delete(metadata, "loop_name")
		changed = true
	}
	if !changed {
		return nil, nil
	}
	return json.Marshal(metadata)
}

func setMetadataString(metadata map[string]any, key string, value string) bool {
	trimmed := strings.TrimSpace(value)
	if current, ok := metadata[key].(string); ok && current == trimmed {
		return false
	}
	metadata[key] = trimmed
	return true
}
