package task

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

func summaryFromTaskRecord(record Task) Summary {
	return Summary{
		ID:                 record.ID,
		Identifier:         record.Identifier,
		Scope:              record.Scope,
		WorkspaceID:        record.WorkspaceID,
		ParentTaskID:       record.ParentTaskID,
		Title:              record.Title,
		Priority:           record.Priority,
		MaxAttempts:        record.MaxAttempts,
		AutoEnqueueOnReady: record.AutoEnqueueOnReady,
		Status:             record.Status,
		ApprovalPolicy:     record.ApprovalPolicy,
		ApprovalState:      record.ApprovalState,
		Draft:              record.Status.Normalize() == TaskStatusDraft,
		NeedsAttention:     cloneNeedsAttention(record.NeedsAttention),
		Owner:              cloneOwnership(record.Owner),
		CurrentRunID:       record.CurrentRunID,
		LatestEventSeq:     record.LatestEventSeq,
		Paused:             record.Paused,
		WakeCreator:        record.WakeCreator,
		PausedBy:           record.PausedBy,
		PausedAt:           record.PausedAt,
		PausedReason:       record.PausedReason,
		EffectivePaused:    record.Paused,
		PausedByTaskID: func() string {
			if record.Paused {
				return record.ID
			}
			return ""
		}(),
		CreatedBy:      record.CreatedBy,
		Origin:         record.Origin,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
		ClosedAt:       record.ClosedAt,
		LastActivityAt: record.UpdatedAt,
	}
}

func taskRecordFromSummary(summary *Summary) Task {
	return Task{
		ID:                 summary.ID,
		Identifier:         summary.Identifier,
		Scope:              summary.Scope,
		WorkspaceID:        summary.WorkspaceID,
		ParentTaskID:       summary.ParentTaskID,
		Title:              summary.Title,
		Priority:           summary.Priority,
		MaxAttempts:        summary.MaxAttempts,
		AutoEnqueueOnReady: summary.AutoEnqueueOnReady,
		Status:             summary.Status,
		ApprovalPolicy:     summary.ApprovalPolicy,
		ApprovalState:      summary.ApprovalState,
		NeedsAttention:     cloneNeedsAttention(summary.NeedsAttention),
		Owner:              cloneOwnership(summary.Owner),
		CurrentRunID:       summary.CurrentRunID,
		LatestEventSeq:     summary.LatestEventSeq,
		Paused:             summary.Paused,
		WakeCreator:        summary.WakeCreator,
		PausedBy:           summary.PausedBy,
		PausedAt:           summary.PausedAt,
		PausedReason:       summary.PausedReason,
		CreatedBy:          summary.CreatedBy,
		Origin:             summary.Origin,
		CreatedAt:          summary.CreatedAt,
		UpdatedAt:          summary.UpdatedAt,
		ClosedAt:           summary.ClosedAt,
	}
}

func (m *Service) taskReference(ctx context.Context, taskID string) (Reference, error) {
	record, err := m.store.GetTask(ctx, taskID)
	if err != nil {
		return Reference{}, err
	}
	dependencies, err := m.store.ListDependencies(ctx, record.ID)
	if err != nil {
		return Reference{}, err
	}
	runs, err := m.store.ListTaskRuns(ctx, RunQuery{TaskID: record.ID})
	if err != nil {
		return Reference{}, err
	}
	status, err := m.canonicalTaskStatus(ctx, record, dependencies, runs)
	if err != nil {
		return Reference{}, err
	}
	reference := taskReferenceFromTask(record, status)
	if err := m.applyEffectivePauseToReference(ctx, &reference); err != nil {
		return Reference{}, err
	}
	return reference, nil
}

func taskReferenceFromTask(record Task, status Status) Reference {
	return Reference{
		ID:              record.ID,
		Identifier:      record.Identifier,
		Title:           record.Title,
		Status:          status,
		Priority:        record.Priority,
		Owner:           cloneOwnership(record.Owner),
		Scope:           record.Scope,
		WorkspaceID:     record.WorkspaceID,
		LatestEventSeq:  record.LatestEventSeq,
		Paused:          record.Paused,
		EffectivePaused: record.Paused,
		PausedByTaskID: func() string {
			if record.Paused {
				return record.ID
			}
			return ""
		}(),
	}
}

type effectiveTaskPauseReader interface {
	IsTaskEffectivelyPaused(ctx context.Context, taskID string) (bool, string, error)
}

func (m *Service) applyEffectivePauseToSummary(ctx context.Context, summary *Summary) error {
	if summary == nil {
		return nil
	}
	summary.EffectivePaused = summary.Paused
	if summary.Paused {
		summary.PausedByTaskID = summary.ID
	}
	reader, ok := m.store.(effectiveTaskPauseReader)
	if !ok {
		return nil
	}
	paused, pausedByTaskID, err := reader.IsTaskEffectivelyPaused(ctx, summary.ID)
	if err != nil {
		return err
	}
	summary.EffectivePaused = paused
	summary.PausedByTaskID = pausedByTaskID
	return nil
}

func (m *Service) applyEffectivePauseToReference(ctx context.Context, reference *Reference) error {
	if reference == nil {
		return nil
	}
	reference.EffectivePaused = reference.Paused
	if reference.Paused {
		reference.PausedByTaskID = reference.ID
	}
	reader, ok := m.store.(effectiveTaskPauseReader)
	if !ok {
		return nil
	}
	paused, pausedByTaskID, err := reader.IsTaskEffectivelyPaused(ctx, reference.ID)
	if err != nil {
		return err
	}
	reference.EffectivePaused = paused
	reference.PausedByTaskID = pausedByTaskID
	return nil
}

func activeRunSummary(runs []Run, maxAttempts int) *RunSummary {
	var current *Run
	for idx := range runs {
		run := runs[idx]
		if isTerminalRunStatus(run.Status) {
			continue
		}
		if current == nil || prefersActiveRun(run, *current) {
			candidate := run
			current = &candidate
		}
	}
	if current == nil {
		return nil
	}
	summary := &RunSummary{
		ID:                           current.ID,
		TaskID:                       current.TaskID,
		Status:                       current.Status,
		Attempt:                      int(current.Attempt),
		RecoveryCount:                int(current.RecoveryCount),
		PreviousRunID:                current.PreviousRunID,
		FailureKind:                  current.FailureKind,
		MaxAttempts:                  maxAttempts,
		SessionID:                    current.SessionID,
		ClaimedBy:                    cloneActorIdentity(current.ClaimedBy),
		ClaimTokenHash:               current.ClaimTokenHash,
		LeaseUntil:                   current.LeaseUntil,
		HeartbeatAt:                  current.HeartbeatAt,
		ResolvedNetworkParticipation: participation.CloneSpec(current.NetworkSpecSnapshot()),
		DesignationGroupID:           current.DesignationGroupID,
		QueuedAt:                     current.QueuedAt,
		ClaimedAt:                    current.ClaimedAt,
		StartedAt:                    current.StartedAt,
		EndedAt:                      current.EndedAt,
		Error:                        current.Error,
	}
	ApplyRunDesignationSummary(summary, *current)
	return summary
}

func prefersActiveRun(candidate Run, current Run) bool {
	candidateRank := activeRunRank(candidate.Status)
	currentRank := activeRunRank(current.Status)
	if candidateRank != currentRank {
		return candidateRank > currentRank
	}

	candidateAt := latestRunActivityAt(candidate)
	currentAt := latestRunActivityAt(current)
	if !candidateAt.Equal(currentAt) {
		return candidateAt.After(currentAt)
	}
	return candidate.ID > current.ID
}

func activeRunRank(status RunStatus) int {
	switch status.Normalize() {
	case TaskRunStatusRunning:
		return 4
	case TaskRunStatusStarting:
		return 3
	case TaskRunStatusClaimed:
		return 2
	case TaskRunStatusQueued:
		return 1
	default:
		return 0
	}
}

func latestTaskActivityAt(record Task, runs []Run, events []Event) time.Time {
	latest := record.UpdatedAt
	if latest.IsZero() || (!record.CreatedAt.IsZero() && record.CreatedAt.After(latest)) {
		latest = record.CreatedAt
	}
	for _, run := range runs {
		runAt := latestRunActivityAt(run)
		if runAt.After(latest) {
			latest = runAt
		}
	}
	for _, event := range events {
		if event.Timestamp.After(latest) {
			latest = event.Timestamp
		}
	}
	return latest
}

func latestRunActivityAt(run Run) time.Time {
	latest := run.QueuedAt
	for _, candidate := range []time.Time{run.ClaimedAt, run.StartedAt, run.EndedAt} {
		if candidate.After(latest) {
			latest = candidate
		}
	}
	return latest
}

func normalizeOwnership(owner *Ownership) *Ownership {
	if owner == nil {
		return nil
	}
	normalized := *owner
	normalized.Kind = normalized.Kind.Normalize()
	normalized.Ref = strings.TrimSpace(normalized.Ref)
	if normalized.IsZero() {
		return nil
	}
	return &normalized
}

func cloneOwnership(owner *Ownership) *Ownership {
	if owner == nil {
		return nil
	}
	cloned := *owner
	return &cloned
}

func cloneActorIdentity(actor *ActorIdentity) *ActorIdentity {
	if actor == nil {
		return nil
	}
	cloned := *actor
	return &cloned
}

func cloneNeedsAttention(attention *NeedsAttention) *NeedsAttention {
	if attention == nil {
		return nil
	}
	cloned := *attention
	return &cloned
}

func sameOwnership(left *Ownership, right *Ownership) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.Kind.Normalize() == right.Kind.Normalize() &&
			strings.TrimSpace(left.Ref) == strings.TrimSpace(right.Ref)
	}
}

func normalizeRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	return json.RawMessage(trimmed)
}

func cloneRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	cloned := make(json.RawMessage, len(raw))
	copy(cloned, raw)
	return cloned
}

func sameRawJSON(left json.RawMessage, right json.RawMessage) bool {
	return bytes.Equal(normalizeRawJSON(left), normalizeRawJSON(right))
}
