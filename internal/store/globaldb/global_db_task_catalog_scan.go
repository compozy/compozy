package globaldb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type taskCatalogScanFields struct {
	identifier               sql.NullString
	scope                    string
	workspaceID              sql.NullString
	parentTaskID             sql.NullString
	priority                 string
	maxAttempts              int
	autoEnqueueOnReady       bool
	status                   string
	approvalPolicy           string
	approvalState            string
	ownerKind                sql.NullString
	ownerRef                 sql.NullString
	currentRunID             sql.NullString
	createdByKind            string
	originKind               string
	createdAt                string
	updatedAt                string
	closedAt                 sql.NullString
	needsAttentionReason     sql.NullString
	needsAttentionAt         sql.NullString
	needsAttentionByKind     sql.NullString
	needsAttentionByRef      sql.NullString
	wakeCreator              bool
	childCount               int
	dependencyCount          int
	lastActivityAt           string
	priorityRank             int
	activeRunID              sql.NullString
	activeRunWorkspaceID     sql.NullString
	activeRunStatus          sql.NullString
	activeRunAttempt         sql.NullInt64
	activeRunRecoveryCount   sql.NullInt64
	activeRunPreviousRunID   sql.NullString
	activeRunFailureKind     sql.NullString
	activeRunClaimedByKind   sql.NullString
	activeRunClaimedByRef    sql.NullString
	activeRunSessionID       sql.NullString
	activeRunLeaseUntil      sql.NullString
	activeRunHeartbeatAt     sql.NullString
	activeRunNetworkSpecJSON sql.NullString
	activeRunNetworkMode     sql.NullString
	activeRunNetworkChannel  sql.NullString
	activeRunNetworkSource   sql.NullString
	activeRunQueuedAt        sql.NullString
	activeRunClaimedAt       sql.NullString
	activeRunStartedAt       sql.NullString
	activeRunEndedAt         sql.NullString
	activeRunError           sql.NullString
}

func scanTaskCatalogSummary(scanner rowScanner) (taskpkg.Summary, error) {
	var summary taskpkg.Summary
	var fields taskCatalogScanFields
	if err := scanner.Scan(
		&summary.ID,
		&fields.identifier,
		&fields.scope,
		&fields.workspaceID,
		&fields.parentTaskID,
		&summary.Title,
		&fields.priority,
		&fields.maxAttempts,
		&fields.autoEnqueueOnReady,
		&fields.status,
		&fields.approvalPolicy,
		&fields.approvalState,
		&fields.ownerKind,
		&fields.ownerRef,
		&fields.currentRunID,
		&summary.LatestEventSeq,
		&fields.createdByKind,
		&summary.CreatedBy.Ref,
		&fields.originKind,
		&summary.Origin.Ref,
		&fields.createdAt,
		&fields.updatedAt,
		&fields.closedAt,
		&fields.needsAttentionReason,
		&fields.needsAttentionAt,
		&fields.needsAttentionByKind,
		&fields.needsAttentionByRef,
		&fields.wakeCreator,
		&fields.childCount,
		&fields.dependencyCount,
		&fields.lastActivityAt,
		&fields.priorityRank,
		&fields.activeRunID,
		&fields.activeRunWorkspaceID,
		&fields.activeRunStatus,
		&fields.activeRunAttempt,
		&fields.activeRunRecoveryCount,
		&fields.activeRunPreviousRunID,
		&fields.activeRunFailureKind,
		&fields.activeRunClaimedByKind,
		&fields.activeRunClaimedByRef,
		&fields.activeRunSessionID,
		&fields.activeRunLeaseUntil,
		&fields.activeRunHeartbeatAt,
		&fields.activeRunNetworkSpecJSON,
		&fields.activeRunNetworkMode,
		&fields.activeRunNetworkChannel,
		&fields.activeRunNetworkSource,
		&fields.activeRunQueuedAt,
		&fields.activeRunClaimedAt,
		&fields.activeRunStartedAt,
		&fields.activeRunEndedAt,
		&fields.activeRunError,
	); err != nil {
		return taskpkg.Summary{}, fmt.Errorf("store: scan task catalog row: %w", err)
	}

	if err := assignTaskCatalogSummary(&summary, &fields); err != nil {
		return taskpkg.Summary{}, err
	}
	return summary, nil
}

func assignTaskCatalogSummary(summary *taskpkg.Summary, fields *taskCatalogScanFields) error {
	summary.Identifier = taskNullStringValue(fields.identifier)
	summary.Scope = taskpkg.Scope(strings.TrimSpace(fields.scope))
	summary.WorkspaceID = taskNullStringValue(fields.workspaceID)
	summary.ParentTaskID = taskNullStringValue(fields.parentTaskID)
	summary.Priority = taskpkg.Priority(strings.TrimSpace(fields.priority)).Normalize()
	summary.MaxAttempts = fields.maxAttempts
	summary.AutoEnqueueOnReady = fields.autoEnqueueOnReady
	summary.Status = taskpkg.Status(strings.TrimSpace(fields.status)).Normalize()
	summary.ApprovalPolicy = taskpkg.ApprovalPolicy(strings.TrimSpace(fields.approvalPolicy)).Normalize()
	summary.ApprovalState = taskpkg.ApprovalState(strings.TrimSpace(fields.approvalState)).Normalize()
	summary.Owner = taskCatalogOwnership(fields.ownerKind, fields.ownerRef)
	summary.CurrentRunID = taskNullStringValue(fields.currentRunID)
	summary.CreatedBy.Kind = taskpkg.ActorKind(strings.TrimSpace(fields.createdByKind)).Normalize()
	summary.Origin.Kind = taskpkg.OriginKind(strings.TrimSpace(fields.originKind)).Normalize()
	summary.Draft = summary.Status == taskpkg.TaskStatusDraft
	summary.WakeCreator = fields.wakeCreator
	summary.ChildCount = taskpkg.ClampSummaryCount(fields.childCount)
	summary.DependencyCount = taskpkg.ClampSummaryCount(fields.dependencyCount)

	var err error
	summary.CreatedAt, err = store.ParseTimestamp(fields.createdAt)
	if err != nil {
		return fmt.Errorf("store: parse task catalog created_at: %w", err)
	}
	summary.UpdatedAt, err = store.ParseTimestamp(fields.updatedAt)
	if err != nil {
		return fmt.Errorf("store: parse task catalog updated_at: %w", err)
	}
	summary.LastActivityAt, err = store.ParseTimestamp(fields.lastActivityAt)
	if err != nil {
		return fmt.Errorf("store: parse task catalog last_activity_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(&summary.ClosedAt, fields.closedAt); err != nil {
		return fmt.Errorf("store: parse task catalog closed_at: %w", err)
	}
	if err := assignTaskCatalogAttention(summary, fields); err != nil {
		return err
	}
	run, err := taskCatalogRunSummary(summary.ID, summary.MaxAttempts, fields)
	if err != nil {
		return err
	}
	summary.ActiveRun = run
	return nil
}

func taskCatalogOwnership(kind sql.NullString, ref sql.NullString) *taskpkg.Ownership {
	if !kind.Valid && !ref.Valid {
		return nil
	}
	return &taskpkg.Ownership{
		Kind: taskpkg.OwnerKind(strings.TrimSpace(kind.String)).Normalize(),
		Ref:  strings.TrimSpace(ref.String),
	}
}

func assignTaskCatalogAttention(summary *taskpkg.Summary, fields *taskCatalogScanFields) error {
	if !fields.needsAttentionReason.Valid && !fields.needsAttentionAt.Valid &&
		!fields.needsAttentionByKind.Valid && !fields.needsAttentionByRef.Valid {
		return nil
	}
	attention := &taskpkg.NeedsAttention{
		Reason: strings.TrimSpace(fields.needsAttentionReason.String),
		By: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(fields.needsAttentionByKind.String)).Normalize(),
			Ref:  strings.TrimSpace(fields.needsAttentionByRef.String),
		},
	}
	if err := assignNullableTaskTimestamp(&attention.At, fields.needsAttentionAt); err != nil {
		return fmt.Errorf("store: parse task catalog needs_attention_at: %w", err)
	}
	summary.NeedsAttention = attention
	return nil
}

func taskCatalogRunSummary(
	taskID string,
	maxAttempts int,
	fields *taskCatalogScanFields,
) (*taskpkg.RunSummary, error) {
	if !fields.activeRunID.Valid {
		return nil, nil
	}
	networkSpec, err := decodeParticipationSnapshot(
		taskNullStringValue(fields.activeRunWorkspaceID),
		fields.activeRunNetworkSpecJSON.String,
		fields.activeRunNetworkMode.String,
		fields.activeRunNetworkChannel,
		fields.activeRunNetworkSource.String,
	)
	if err != nil {
		return nil, err
	}
	run := &taskpkg.RunSummary{
		ID:                           strings.TrimSpace(fields.activeRunID.String),
		TaskID:                       strings.TrimSpace(taskID),
		Status:                       taskpkg.ParseRunStatus(fields.activeRunStatus.String).Normalize(),
		Attempt:                      int(fields.activeRunAttempt.Int64),
		RecoveryCount:                int(fields.activeRunRecoveryCount.Int64),
		PreviousRunID:                strings.TrimSpace(fields.activeRunPreviousRunID.String),
		FailureKind:                  strings.TrimSpace(fields.activeRunFailureKind.String),
		MaxAttempts:                  maxAttempts,
		SessionID:                    strings.TrimSpace(fields.activeRunSessionID.String),
		ResolvedNetworkParticipation: participation.CloneSpec(networkSpec),
		Error:                        strings.TrimSpace(fields.activeRunError.String),
	}
	if fields.activeRunClaimedByKind.Valid || fields.activeRunClaimedByRef.Valid {
		run.ClaimedBy = &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(fields.activeRunClaimedByKind.String)).Normalize(),
			Ref:  strings.TrimSpace(fields.activeRunClaimedByRef.String),
		}
	}
	if err := assignNullableTaskTimestamp(&run.LeaseUntil, fields.activeRunLeaseUntil); err != nil {
		return nil, fmt.Errorf("store: parse task catalog active run lease_until: %w", err)
	}
	if err := assignNullableTaskTimestamp(&run.HeartbeatAt, fields.activeRunHeartbeatAt); err != nil {
		return nil, fmt.Errorf("store: parse task catalog active run heartbeat_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(&run.QueuedAt, fields.activeRunQueuedAt); err != nil {
		return nil, fmt.Errorf("store: parse task catalog active run queued_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(&run.ClaimedAt, fields.activeRunClaimedAt); err != nil {
		return nil, fmt.Errorf("store: parse task catalog active run claimed_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(&run.StartedAt, fields.activeRunStartedAt); err != nil {
		return nil, fmt.Errorf("store: parse task catalog active run started_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(&run.EndedAt, fields.activeRunEndedAt); err != nil {
		return nil, fmt.Errorf("store: parse task catalog active run ended_at: %w", err)
	}
	return run, nil
}
