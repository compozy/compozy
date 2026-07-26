package memory

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/diagnostics"

	storepkg "github.com/compozy/agh/internal/store"
)

func (s *Store) markDreamPromoted(
	ctx context.Context,
	candidates []DreamCandidate,
	runID string,
	promotedAt time.Time,
) (int, error) {
	if s == nil || s.catalog == nil || len(candidates) == 0 {
		return 0, nil
	}
	if strings.TrimSpace(runID) == "" {
		return 0, errors.New("memory: dream run id is required")
	}
	db, err := s.catalog.ensureDB(ctx)
	if err != nil {
		return 0, err
	}
	if db == nil {
		return 0, nil
	}
	chunkIDs := dreamCandidateChunkIDs(candidates)
	if len(chunkIDs) == 0 {
		return 0, nil
	}
	args := []any{timeToUnixMillis(promotedAt), strings.TrimSpace(runID)}
	placeholders := make([]string, 0, len(chunkIDs))
	for _, chunkID := range chunkIDs {
		placeholders = append(placeholders, "?")
		args = append(args, chunkID)
	}
	query := `UPDATE memory_recall_signals
SET promoted_at = ?, promotion_run_id = ?
WHERE promoted_at IS NULL AND chunk_id IN (` + strings.Join(placeholders, ",") + `)`

	var promoted int64
	err = s.catalog.withCatalogWriteTx(ctx, "dream mark promoted", func(tx *storepkg.WriteTx) error {
		result, execErr := tx.ExecContext(ctx, query, args...)
		if execErr != nil {
			return fmt.Errorf("memory: mark dream signals promoted: %w", execErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return fmt.Errorf("memory: count promoted dream signals: %w", rowsErr)
		}
		promoted = rows
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int(promoted), nil
}

func dreamCandidateChunkIDs(candidates []DreamCandidate) []string {
	ids := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		chunkID := strings.TrimSpace(candidate.ChunkID)
		if chunkID == "" {
			continue
		}
		if _, exists := seen[chunkID]; exists {
			continue
		}
		seen[chunkID] = struct{}{}
		ids = append(ids, chunkID)
	}
	return ids
}

func (s *Store) startDreamRun(
	ctx context.Context,
	run dreamSignalGateResult,
	workspace dreamRunWorkspace,
	at time.Time,
) error {
	return s.upsertDreamRun(ctx, run, workspace, "running", 0, "", at)
}

func (s *Store) deleteDreamRun(ctx context.Context, runID string) error {
	if s == nil || s.catalog == nil {
		return nil
	}
	trimmed := strings.TrimSpace(runID)
	if trimmed == "" {
		return errors.New("memory: dream run id is required")
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return err
	}
	return s.catalog.withCatalogWriteTx(ctx, "dream run delete", func(tx *storepkg.WriteTx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM memory_consolidations WHERE id = ?`, trimmed); err != nil {
			return fmt.Errorf("memory: delete dream run %q: %w", trimmed, err)
		}
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM memory_events WHERE target_id = ? AND op = ?`,
			trimmed,
			memoryEventDreamStarted,
		); err != nil {
			return fmt.Errorf("memory: delete dream run event %q: %w", trimmed, err)
		}
		return nil
	})
}

func (s *Store) completeDreamRun(
	ctx context.Context,
	run dreamSignalGateResult,
	workspace dreamRunWorkspace,
	promoted int,
	at time.Time,
) error {
	return s.upsertDreamRun(ctx, run, workspace, "completed", promoted, "", at)
}

func (s *Store) failDreamRun(
	ctx context.Context,
	run dreamSignalGateResult,
	workspace dreamRunWorkspace,
	cause error,
	at time.Time,
) error {
	errText := ""
	if cause != nil {
		errText = diagnostics.RedactAndBound(cause.Error(), maxOperationSummaryBytes)
	}
	return s.upsertDreamRun(ctx, run, workspace, "failed", 0, errText, at)
}

func (s *Store) upsertDreamRun(
	ctx context.Context,
	run dreamSignalGateResult,
	workspace dreamRunWorkspace,
	status string,
	promoted int,
	errorText string,
	at time.Time,
) error {
	if s == nil || s.catalog == nil {
		return nil
	}
	if strings.TrimSpace(run.runID) == "" {
		return errors.New("memory: dream run id is required")
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return err
	}
	metadata, err := dreamRunMetadata(run, status)
	if err != nil {
		return err
	}
	return s.catalog.withCatalogWriteTx(ctx, "dream run upsert", func(tx *storepkg.WriteTx) error {
		if err := upsertDreamConsolidationTx(
			ctx,
			tx,
			run,
			workspace,
			status,
			promoted,
			errorText,
			metadata,
			at,
		); err != nil {
			return err
		}
		return insertDreamEventTx(ctx, tx, run, workspace, status, promoted, errorText, metadata, at)
	})
}

func dreamRunMetadata(run dreamSignalGateResult, status string) (string, error) {
	metadata := map[string]string{
		dreamV2PromptVersionKey: dreamPromptVersion,
		"candidate_count": fmt.Sprintf(
			"%d",
			len(run.candidates),
		),
		"status": strings.TrimSpace(status),
	}
	if reason := strings.TrimSpace(run.reason); reason != "" {
		metadata["reason"] = reason
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("memory: encode dream run metadata: %w", err)
	}
	return string(payload), nil
}
