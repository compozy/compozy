package memory

import (
	"context"

	"encoding/json"

	"fmt"

	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/diagnostics"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	memoryrecall "github.com/compozy/agh/internal/memory/recall"
	storepkg "github.com/compozy/agh/internal/store"
)

// RecordRecall persists live recall signals for later dreaming gates.
func (s *Store) RecordRecall(ctx context.Context, signals []memoryrecall.Signal) error {
	if s.catalog == nil || len(signals) == 0 {
		return nil
	}
	return s.catalog.withCatalogWriteTx(ctx, "recall signal update", func(tx *storepkg.WriteTx) error {
		for _, signal := range signals {
			if err := upsertRecallSignal(ctx, tx, signal); err != nil {
				return err
			}
		}
		return nil
	})
}

func upsertRecallSignal(ctx context.Context, tx *storepkg.WriteTx, signal memoryrecall.Signal) error {
	surfacedAt := signal.SurfacedAt.UTC()
	if surfacedAt.IsZero() {
		surfacedAt = time.Now().UTC()
	}
	surfacePayload, err := json.Marshal([]string{strings.TrimSpace(signal.SurfaceID)})
	if err != nil {
		return fmt.Errorf("memory: encode recall signal surface: %w", err)
	}
	sessionCount := 0
	if strings.TrimSpace(signal.SessionID) != "" {
		sessionCount = 1
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO memory_recall_signals (
			chunk_id, workspace_id, recall_count, last_recalled_at, recall_score,
			freshness_started_at, last_score_update_at, session_count, last_session_id,
			already_surfaced_json, updated_at
		) VALUES (?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(chunk_id) DO UPDATE SET
			workspace_id = excluded.workspace_id,
			recall_count = memory_recall_signals.recall_count + 1,
			last_recalled_at = excluded.last_recalled_at,
			recall_score = CASE
				WHEN memory_recall_signals.recall_score <= 0 THEN excluded.recall_score
				ELSE (memory_recall_signals.recall_score * 0.8) + (excluded.recall_score * 0.2)
			END,
			freshness_started_at = CASE
				WHEN memory_recall_signals.freshness_started_at = 0 THEN excluded.freshness_started_at
				ELSE memory_recall_signals.freshness_started_at
			END,
			last_score_update_at = excluded.last_score_update_at,
			session_count = memory_recall_signals.session_count + CASE
				WHEN excluded.last_session_id IS NOT NULL
					AND COALESCE(memory_recall_signals.last_session_id, '') <> excluded.last_session_id
				THEN 1 ELSE 0 END,
			last_session_id = COALESCE(excluded.last_session_id, memory_recall_signals.last_session_id),
			already_surfaced_json = excluded.already_surfaced_json,
			updated_at = excluded.updated_at`,
		strings.TrimSpace(signal.ChunkID),
		nullStringForEmpty(signal.WorkspaceID),
		timeToUnixMillis(surfacedAt),
		signal.Score,
		timeToUnixMillis(surfacedAt),
		timeToUnixMillis(surfacedAt),
		sessionCount,
		nullStringForEmpty(signal.SessionID),
		string(surfacePayload),
		timeToUnixMillis(surfacedAt),
	); err != nil {
		return fmt.Errorf("memory: upsert recall signal %q: %w", signal.ChunkID, err)
	}
	return nil
}

func (s *Store) RecordRecallExecuted(ctx context.Context, query memcontract.Query, resultCount int) error {
	return s.insertRecallEvent(ctx, memoryEventRecallExecuted, query, "", map[string]string{
		memoryEventMetadataQueryKey:       query.QueryText,
		memoryEventMetadataResultCountKey: fmt.Sprintf("%d", resultCount),
		memoryEventMetadataSummaryKey: fmt.Sprintf(
			"query=%q results=%d",
			strings.TrimSpace(query.QueryText),
			resultCount,
		),
	})
}

func (s *Store) RecordRecallSkipped(ctx context.Context, query memcontract.Query, reason string) error {
	return s.insertRecallEvent(ctx, memoryEventRecallSkipped, query, "", map[string]string{
		memoryEventMetadataQueryKey:   query.QueryText,
		memoryEventMetadataSummaryKey: strings.TrimSpace(reason),
	})
}

func (s *Store) RecordRecallSignalFailed(ctx context.Context, query memcontract.Query, cause error) error {
	if cause == nil {
		return nil
	}
	summary := diagnostics.RedactAndBound(cause.Error(), maxOperationSummaryBytes)
	return s.insertRecallEvent(ctx, memoryEventRecallSignalFailed, query, "", map[string]string{
		memoryEventMetadataQueryKey:   query.QueryText,
		memoryEventMetadataSummaryKey: summary,
	})
}

func (s *Store) RecordRecallSignalDropped(
	ctx context.Context,
	query memcontract.Query,
	signals []memoryrecall.Signal,
	queueDepth int,
) error {
	if len(signals) == 0 {
		return nil
	}
	targetID := strings.TrimSpace(signals[0].ChunkID)
	summary := fmt.Sprintf("dropped=%d queue_depth=%d", len(signals), queueDepth)
	return s.insertRecallEvent(ctx, memoryEventRecallSignalDropped, query, targetID, map[string]string{
		memoryEventMetadataQueryKey:   query.QueryText,
		memoryEventMetadataSummaryKey: summary,
		"dropped_count":               strconv.Itoa(len(signals)),
		"queue_depth":                 strconv.Itoa(queueDepth),
	})
}

func (s *Store) RecordShadow(ctx context.Context, shadow memoryrecall.Shadow) error {
	query := memcontract.Query{
		WorkspaceID: shadow.WorkspaceID,
		AgentName:   shadow.AgentName,
	}
	return s.insertRecallEvent(ctx, memoryEventWriteShadowed, query, shadow.LoserChunkID, map[string]string{
		"winner_chunk_id":        shadow.WinnerChunkID,
		"type":                   string(shadow.Type.Normalize()),
		"slug":                   strings.TrimSpace(shadow.Slug),
		recallSourceAgentTierKey: string(shadow.AgentTier.Normalize()),
	})
}

func (s *Store) insertRecallEvent(
	ctx context.Context,
	op string,
	query memcontract.Query,
	targetID string,
	metadata map[string]string,
) error {
	if s.catalog == nil {
		return nil
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("memory: encode recall event metadata: %w", err)
	}
	workspaceID := strings.TrimSpace(query.WorkspaceID)
	scope := memcontract.ScopeGlobal
	if workspaceID != "" {
		scope = memcontract.ScopeWorkspace
	}
	agentName := strings.TrimSpace(query.AgentName)
	if agentName == "" {
		agentName = catalogEventAgentName
	}
	return s.catalog.withCatalogWriteTx(ctx, "recall event insert", func(tx *storepkg.WriteTx) error {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO memory_events (
				op, scope, agent_name, agent_tier, workspace_id, session_id, actor_kind,
				decision_id, target_id, metadata, ts_ms
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			strings.TrimSpace(op),
			string(scope),
			agentName,
			nil,
			nullStringForEmpty(workspaceID),
			nil,
			"system",
			nil,
			nullStringForEmpty(targetID),
			string(payload),
			timeToUnixMillis(time.Now().UTC()),
		); err != nil {
			return fmt.Errorf("memory: write recall event %q: %w", op, err)
		}
		return nil
	})
}
