package memory

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/diagnostics"
	memoryextractor "github.com/compozy/agh/internal/memory/extractor"
	storepkg "github.com/compozy/agh/internal/store"
)

// RecordExtractorEvent persists canonical extractor telemetry into memory_events.
func (s *Store) RecordExtractorEvent(ctx context.Context, event memoryextractor.Event) error {
	if ctx == nil {
		return errors.New("memory: extractor event context is required")
	}
	if s == nil || s.catalog == nil {
		return nil
	}
	normalized := event.Normalize(func() time.Time {
		return time.Now().UTC()
	})
	if !isExtractorEventOp(normalized.Op) {
		return fmt.Errorf("memory: unsupported extractor event op %q", normalized.Op)
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return err
	}
	metadata, err := extractorEventMetadata(normalized)
	if err != nil {
		return err
	}
	return s.catalog.withCatalogWriteTx(ctx, "extractor event insert", func(tx *storepkg.WriteTx) error {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO memory_events (
				op, scope, agent_name, agent_tier, workspace_id, session_id,
				actor_kind, decision_id, target_id, metadata, ts_ms
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			normalized.Op,
			nil,
			nullStringForEmpty(strings.TrimSpace(normalized.AgentID)),
			nil,
			nullStringForEmpty(strings.TrimSpace(normalized.WorkspaceID)),
			nullStringForEmpty(strings.TrimSpace(normalized.SessionID)),
			firstNonEmpty(normalized.ActorKind, "system"),
			nullStringForEmpty(strings.TrimSpace(normalized.DecisionID)),
			nullStringForEmpty(strings.TrimSpace(normalized.TargetID)),
			metadata,
			timeToUnixMillis(normalized.At),
		); err != nil {
			return fmt.Errorf("memory: insert extractor event: %w", err)
		}
		return nil
	})
}

func isExtractorEventOp(op string) bool {
	switch strings.TrimSpace(op) {
	case memoryextractor.EventStarted,
		memoryextractor.EventCompleted,
		memoryextractor.EventFailed,
		memoryextractor.EventCoalesced,
		memoryextractor.EventDropped:
		return true
	default:
		return false
	}
}

func extractorEventMetadata(event memoryextractor.Event) (string, error) {
	metadata := make(map[string]string, len(event.Metadata)+5)
	for key, value := range event.Metadata {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		metadata[key] = diagnostics.RedactAndBound(value, maxOperationSummaryBytes)
	}
	if event.Turn.SinceMessageSeq > 0 {
		metadata["since_message_seq"] = fmt.Sprintf("%d", event.Turn.SinceMessageSeq)
	}
	if event.Turn.UntilMessageSeq > 0 {
		metadata["until_message_seq"] = fmt.Sprintf("%d", event.Turn.UntilMessageSeq)
	}
	if trigger := event.Turn.Trigger.Normalize(); trigger != "" {
		metadata["trigger"] = string(trigger)
	}
	if strings.TrimSpace(event.Error) != "" {
		metadata["error"] = diagnostics.RedactAndBound(event.Error, maxOperationSummaryBytes)
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("memory: encode extractor event metadata: %w", err)
	}
	return string(payload), nil
}
