package memory

import (
	"context"

	"encoding/json"

	"fmt"

	"strings"
	"time"

	storepkg "github.com/compozy/agh/internal/store"
)

func upsertDreamConsolidationTx(
	ctx context.Context,
	tx *storepkg.WriteTx,
	run dreamSignalGateResult,
	workspace dreamRunWorkspace,
	status string,
	promoted int,
	errorText string,
	metadata string,
	at time.Time,
) error {
	finishedAt := any(nil)
	if status != "running" {
		finishedAt = timeToUnixMillis(at)
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO memory_consolidations (
			id, workspace_id, scope, agent_name, agent_tier, started_at, finished_at,
			status, input_count, promoted_count, error, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			finished_at = excluded.finished_at,
			status = excluded.status,
			promoted_count = excluded.promoted_count,
			error = excluded.error,
			metadata = excluded.metadata`,
		strings.TrimSpace(run.runID),
		nullStringForEmpty(workspace.id),
		string(workspace.scope.Normalize()),
		catalogEventAgentName,
		nil,
		timeToUnixMillis(at),
		finishedAt,
		status,
		len(run.candidates),
		promoted,
		errorText,
		metadata,
	); err != nil {
		return fmt.Errorf("memory: upsert dream consolidation %q: %w", run.runID, err)
	}
	return nil
}

func insertDreamEventTx(
	ctx context.Context,
	tx *storepkg.WriteTx,
	run dreamSignalGateResult,
	workspace dreamRunWorkspace,
	status string,
	promoted int,
	errorText string,
	metadata string,
	at time.Time,
) error {
	op := memoryEventDreamStarted
	switch status {
	case "completed":
		op = memoryEventDreamPromoted
	case "failed":
		op = memoryEventDreamFailed
	}
	eventMetadata := metadata
	if promoted > 0 || strings.TrimSpace(errorText) != "" {
		payload, err := mergeDreamEventMetadata(metadata, promoted, errorText)
		if err != nil {
			return err
		}
		eventMetadata = payload
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO memory_events (
			op, scope, agent_name, agent_tier, workspace_id, session_id, actor_kind,
			decision_id, target_id, metadata, ts_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		op,
		nullStringForEmpty(workspace.scope.Normalize()),
		catalogEventAgentName,
		nil,
		nullStringForEmpty(workspace.id),
		nil,
		"system",
		nil,
		nullStringForEmpty(run.runID),
		eventMetadata,
		timeToUnixMillis(at),
	); err != nil {
		return fmt.Errorf("memory: insert dream event %q: %w", op, err)
	}
	return nil
}

func mergeDreamEventMetadata(metadata string, promoted int, errorText string) (string, error) {
	values := map[string]string{}
	if strings.TrimSpace(metadata) != "" {
		if err := json.Unmarshal([]byte(metadata), &values); err != nil {
			return "", fmt.Errorf("memory: parse dream event metadata: %w", err)
		}
	}
	if promoted > 0 {
		values["promoted_count"] = fmt.Sprintf("%d", promoted)
	}
	if strings.TrimSpace(errorText) != "" {
		values[dreamErrorFieldKey] = errorText
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("memory: encode dream event metadata: %w", err)
	}
	return string(payload), nil
}
