package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func claimRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	criteria taskpkg.ClaimCriteria,
	claimToken string,
	claimHash string,
	leaseUntil time.Time,
) error {
	metadata, err := claimRunMetadata(ctx, exec, runID, criteria)
	if err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).ClaimSelectedTaskRun(ctx, sqlcgen.ClaimSelectedTaskRunParams{
		ClaimedStatus:  taskpkg.TaskRunStatusClaimed.String(),
		ClaimedByKind:  nullableTaskActorKind(criteria.ClaimedBy),
		ClaimedByRef:   nullableTaskActorRef(criteria.ClaimedBy),
		SessionID:      nullableTaskString(criteria.ClaimerSessionID),
		ClaimToken:     nullableTaskString(claimToken),
		ClaimTokenHash: nullableTaskString(claimHash),
		LeaseUntil:     nullableTaskTime(leaseUntil),
		HeartbeatAt:    nullableTaskTime(criteria.Now),
		ClaimedAt:      nullableTaskTime(criteria.Now),
		MetadataJson:   nullableTaskRawJSON(metadata),
		ID:             runID,
		QueuedStatus:   taskpkg.TaskRunStatusQueued.String(),
	})
	if err != nil {
		return fmt.Errorf("store: claim task run %q: %w", runID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: task run claim %q: %w", runID, taskpkg.ErrNoClaimableRun)
	}
	return nil
}

func claimRunMetadata(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	criteria taskpkg.ClaimCriteria,
) (json.RawMessage, error) {
	raw, err := sqlcgen.New(exec).GetTaskRunMetadataForClaim(ctx, strings.TrimSpace(runID))
	if err != nil {
		return nil, fmt.Errorf("store: load task run metadata for claim %q: %w", runID, err)
	}
	metadata, err := decodeTaskJSON(raw, "task_run.metadata_json")
	if err != nil {
		return nil, err
	}
	if criteria.Soul == nil || strings.TrimSpace(criteria.Soul.Digest) == "" {
		return normalizeTaskJSON(metadata), nil
	}
	merged, err := mergeClaimSoulMetadata(metadata, *criteria.Soul)
	if err != nil {
		return nil, fmt.Errorf("store: merge soul claim metadata for run %q: %w", runID, err)
	}
	return merged, nil
}

func mergeClaimSoulMetadata(
	current json.RawMessage,
	provenance taskpkg.SoulClaimProvenance,
) (json.RawMessage, error) {
	normalized := normalizeTaskJSON(current)
	fields := make(map[string]json.RawMessage)
	if len(normalized) > 0 {
		if err := json.Unmarshal(normalized, &fields); err != nil {
			return nil, fmt.Errorf(
				"%w: task_run.metadata_json must be a JSON object for soul provenance",
				taskpkg.ErrValidation,
			)
		}
		if fields == nil {
			fields = make(map[string]json.RawMessage)
		}
	}
	payload := struct {
		SnapshotID string    `json:"snapshot_id,omitempty"`
		Digest     string    `json:"digest,omitempty"`
		AgentName  string    `json:"agent_name,omitempty"`
		CapturedAt time.Time `json:"captured_at"`
	}{
		SnapshotID: strings.TrimSpace(provenance.SnapshotID),
		Digest:     strings.TrimSpace(provenance.Digest),
		AgentName:  strings.TrimSpace(provenance.AgentName),
		CapturedAt: provenance.CapturedAt.UTC(),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("store: marshal soul claim metadata: %w", err)
	}
	fields["soul"] = encoded
	merged, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("store: marshal task run claim metadata: %w", err)
	}
	result := normalizeTaskJSON(merged)
	if err := taskpkg.ValidateMetadataSize(result, "task_run.metadata_json"); err != nil {
		return nil, err
	}
	return result, nil
}
