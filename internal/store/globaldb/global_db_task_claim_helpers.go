package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func requireCurrentRunLease(run taskpkg.Run, rawToken string, now time.Time) error {
	if strings.TrimSpace(run.ClaimTokenHash) == "" {
		return fmt.Errorf(
			"%w: task run %q has no current claim token hash",
			taskpkg.ErrInvalidClaimToken,
			run.ID,
		)
	}
	if !taskpkg.VerifyClaimToken(rawToken, run.ClaimTokenHash) {
		return fmt.Errorf("%w: task run %q token mismatch", taskpkg.ErrInvalidClaimToken, run.ID)
	}
	switch run.Status.Normalize() {
	case taskpkg.TaskRunStatusClaimed, taskpkg.TaskRunStatusStarting, taskpkg.TaskRunStatusRunning:
	default:
		return fmt.Errorf(
			"%w: task run %q is not actively leased",
			taskpkg.ErrInvalidStatusTransition,
			run.ID,
		)
	}
	if run.LeaseUntil.IsZero() || !run.LeaseUntil.After(now.UTC()) {
		return fmt.Errorf("%w: task run %q lease expired", taskpkg.ErrLeaseExpired, run.ID)
	}
	return nil
}

func requireLeaseTerminalTransition(run taskpkg.Run, target taskpkg.RunStatus) error {
	switch run.Status.Normalize() {
	case taskpkg.TaskRunStatusClaimed, taskpkg.TaskRunStatusStarting, taskpkg.TaskRunStatusRunning:
		return nil
	default:
		return fmt.Errorf(
			"%w: task run %q cannot transition from %q to %q",
			taskpkg.ErrInvalidStatusTransition,
			run.ID,
			run.Status.Normalize(),
			target.Normalize(),
		)
	}
}

func requeueLeasedRun(ctx context.Context, exec taskSQLExecutor, runID string) error {
	affected, err := sqlcgen.New(exec).RequeueTaskRunLease(ctx, sqlcgen.RequeueTaskRunLeaseParams{
		Status: taskpkg.TaskRunStatusQueued.String(), ID: runID,
	})
	if err != nil {
		return fmt.Errorf("store: requeue task run lease %q: %w", runID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: task run lease %q: %w", runID, taskpkg.ErrTaskRunNotFound)
	}
	return nil
}

func expiredLeaseRunIDs(
	ctx context.Context,
	exec taskSQLExecutor,
	recovery taskpkg.ExpiredLeaseRecovery,
) ([]string, error) {
	rows, err := sqlcgen.New(exec).ListExpiredTaskRunLeaseIDs(ctx, sqlcgen.ListExpiredTaskRunLeaseIDsParams{
		ClaimedStatus:  taskpkg.TaskRunStatusClaimed.String(),
		StartingStatus: taskpkg.TaskRunStatusStarting.String(),
		RunningStatus:  taskpkg.TaskRunStatusRunning.String(),
		Now:            nullableTaskTime(recovery.Now),
		ResultLimit:    taskQueryLimit(recovery.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query expired task run leases: %w", err)
	}
	return rows, nil
}

func requeueExpiredLease(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	snapshot taskRunLeaseSnapshot,
	recoveryIncrement int64,
) error {
	affected, err := sqlcgen.New(exec).RequeueExpiredTaskRunLease(
		ctx,
		sqlcgen.RequeueExpiredTaskRunLeaseParams{
			QueuedStatus:      taskpkg.TaskRunStatusQueued.String(),
			ID:                run.ID,
			PreviousStatus:    snapshot.status.Normalize().String(),
			SessionID:         nullableTaskString(strings.TrimSpace(snapshot.sessionID)),
			ClaimTokenHash:    nullableTaskString(strings.TrimSpace(snapshot.claimTokenHash)),
			LeaseUntil:        nullableTaskTime(snapshot.leaseUntil),
			RecoveryIncrement: recoveryIncrement,
		},
	)
	if err != nil {
		return fmt.Errorf("store: recover expired task run lease %q: %w", run.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: expired task run lease %q: %w", run.ID, taskpkg.ErrTaskRunNotFound)
	}
	return nil
}

func exhaustExpiredLease(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
	snapshot taskRunLeaseSnapshot,
	now time.Time,
) error {
	affected, err := sqlcgen.New(exec).ExhaustExpiredTaskRunLease(
		ctx,
		sqlcgen.ExhaustExpiredTaskRunLeaseParams{
			NeedsAttentionStatus: taskpkg.TaskRunStatusNeedsAttention.String(),
			EndedAt:              nullableTaskTime(now),
			Error:                nullableTaskString(taskpkg.LeaseRecoveryExhaustedReason),
			ID:                   run.ID,
			PreviousStatus:       snapshot.status.Normalize().String(),
			SessionID:            nullableTaskString(strings.TrimSpace(snapshot.sessionID)),
			ClaimTokenHash:       nullableTaskString(strings.TrimSpace(snapshot.claimTokenHash)),
			LeaseUntil:           nullableTaskTime(snapshot.leaseUntil),
		},
	)
	if err != nil {
		return fmt.Errorf("store: exhaust expired task run lease %q: %w", run.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: expired task run lease %q: %w", run.ID, taskpkg.ErrTaskRunNotFound)
	}
	return nil
}

func (g *TaskRunRepo) coordinationChannelMetadata(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
) (*taskpkg.CoordinationChannelMetadata, error) {
	networkSpec := run.NetworkSpecSnapshot()
	channelID := strings.TrimSpace(networkSpec.ChannelID)
	if channelID == "" {
		return nil, nil
	}
	metadata := &taskpkg.CoordinationChannelMetadata{
		ID:          channelID,
		DisplayName: channelID,
		WorkspaceID: run.WorkspaceID,
		TaskID:      run.TaskID,
		RunID:       run.ID,
		WorkflowID:  taskRunMetadataString(run.Metadata, "workflow_id"),
		AllowedMessageKinds: []string{
			globalDBTaskClaimStatusKey,
			"request",
			"reply",
			"blocker",
			globalDBTaskClaimHandoffKey,
			"result",
			"review_request",
		},
	}

	entry, err := getNetworkChannel(ctx, exec, store.NetworkChannelRef{
		WorkspaceID: run.WorkspaceID,
		Channel:     channelID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return metadata, nil
		}
		return nil, err
	}
	metadata.DisplayName = entry.Channel
	metadata.Purpose = entry.Purpose
	metadata.WorkspaceID = entry.WorkspaceID
	metadata.LastActivityAt = entry.UpdatedAt
	return metadata, nil
}

func missingCapabilityPredicate(capabilities []string) string {
	return missingCapabilityPredicateFor("req.capability_id", capabilities)
}

func missingCapabilityPredicateFor(column string, capabilities []string) string {
	if len(capabilities) == 0 {
		return ""
	}
	return " AND " + column + " NOT IN (" + claimPlaceholders(len(capabilities)) + ")"
}

func missingCapabilityArgs(capabilities []string) []any {
	if len(capabilities) == 0 {
		return nil
	}
	args := make([]any, 0, len(capabilities))
	for _, capability := range capabilities {
		args = append(args, capability)
	}
	return args
}

func preferredCapabilityOrder(capabilities []string) string {
	if len(capabilities) == 0 {
		return "(SELECT 0) DESC,"
	}
	return `(SELECT COUNT(1)
		          FROM task_run_preferred_capabilities pref
		         WHERE pref.run_id = tr.id
		           AND pref.capability_id IN (` + claimPlaceholders(len(capabilities)) + `)) DESC,`
}

func preferredCapabilityArgs(capabilities []string) []any {
	return missingCapabilityArgs(capabilities)
}

func claimPlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	values := make([]string, 0, count)
	for range count {
		values = append(values, "?")
	}
	return strings.Join(values, ", ")
}

func taskRunMetadataString(raw []byte, key string) string {
	if len(raw) == 0 || strings.TrimSpace(key) == "" {
		return ""
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return ""
	}
	value, ok := decoded[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
