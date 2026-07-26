package globaldb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type taskInboxScanFields struct {
	identifier               sql.NullString
	scope                    string
	workspaceID              sql.NullString
	priority                 string
	status                   string
	ownerKind                sql.NullString
	ownerRef                 sql.NullString
	approvalPolicy           string
	approvalState            string
	maxAttempts              int
	lastActivityAt           string
	priorityRank             int
	runID                    sql.NullString
	runWorkspaceID           sql.NullString
	runStatus                sql.NullString
	runAttempt               sql.NullInt64
	runRecoveryCount         sql.NullInt64
	runPreviousRunID         sql.NullString
	runFailureKind           sql.NullString
	runClaimedByKind         sql.NullString
	runClaimedByRef          sql.NullString
	runSessionID             sql.NullString
	runLeaseUntil            sql.NullString
	runHeartbeatAt           sql.NullString
	runNetworkSpecJSON       sql.NullString
	runNetworkMode           sql.NullString
	runNetworkChannel        sql.NullString
	runNetworkSource         sql.NullString
	runQueuedAt              sql.NullString
	runClaimedAt             sql.NullString
	runStartedAt             sql.NullString
	runEndedAt               sql.NullString
	runError                 sql.NullString
	lane                     string
	isUnread                 bool
	triageArchived           bool
	triageRead               bool
	triageDismissed          bool
	triageLastSeenActivityAt sql.NullString
	triageUpdatedAt          sql.NullString
}

func scanTaskInboxItem(scanner rowScanner, actor taskpkg.ActorIdentity) (taskpkg.InboxItem, error) {
	var item taskpkg.InboxItem
	var fields taskInboxScanFields
	if err := scanTaskInboxFields(scanner, &item, &fields); err != nil {
		return taskpkg.InboxItem{}, err
	}

	item.Task.Identifier = taskNullStringValue(fields.identifier)
	item.Task.Scope = taskpkg.Scope(strings.TrimSpace(fields.scope))
	item.Task.WorkspaceID = taskNullStringValue(fields.workspaceID)
	item.Task.Priority = taskpkg.Priority(strings.TrimSpace(fields.priority)).Normalize()
	item.Task.Status = taskpkg.Status(strings.TrimSpace(fields.status)).Normalize()
	item.Task.Owner = taskCatalogOwnership(fields.ownerKind, fields.ownerRef)
	item.Lane = taskpkg.InboxLane(strings.TrimSpace(fields.lane)).Normalize()
	item.ApprovalPolicy = taskpkg.ApprovalPolicy(strings.TrimSpace(fields.approvalPolicy)).Normalize()
	item.ApprovalState = taskpkg.ApprovalState(strings.TrimSpace(fields.approvalState)).Normalize()
	item.BlockingReason = taskInboxBlockingReason(item, fields.runStatus)
	item.Triage = taskpkg.TriageState{
		TaskID:    item.Task.ID,
		Actor:     actor,
		Read:      fields.triageRead,
		Archived:  fields.triageArchived,
		Dismissed: fields.triageDismissed,
	}

	var err error
	item.LatestActivityAt, err = store.ParseTimestamp(fields.lastActivityAt)
	if err != nil {
		return taskpkg.InboxItem{}, fmt.Errorf("store: parse task inbox latest_activity_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(
		&item.Triage.LastSeenActivityAt,
		fields.triageLastSeenActivityAt,
	); err != nil {
		return taskpkg.InboxItem{}, fmt.Errorf("store: parse task inbox last_seen_activity_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(&item.Triage.UpdatedAt, fields.triageUpdatedAt); err != nil {
		return taskpkg.InboxItem{}, fmt.Errorf("store: parse task inbox triage updated_at: %w", err)
	}

	runFields := taskInboxRunScanFields(&fields)
	item.Run, err = taskCatalogRunSummary(item.Task.ID, fields.maxAttempts, &runFields)
	if err != nil {
		return taskpkg.InboxItem{}, err
	}
	return item, nil
}

func scanTaskInboxFields(
	scanner rowScanner,
	item *taskpkg.InboxItem,
	fields *taskInboxScanFields,
) error {
	if err := scanner.Scan(
		&item.Task.ID,
		&fields.identifier,
		&fields.scope,
		&fields.workspaceID,
		&item.Task.Title,
		&fields.priority,
		&fields.status,
		&fields.ownerKind,
		&fields.ownerRef,
		&item.Task.LatestEventSeq,
		&fields.approvalPolicy,
		&fields.approvalState,
		&fields.maxAttempts,
		&fields.lastActivityAt,
		&fields.priorityRank,
		&fields.runID,
		&fields.runWorkspaceID,
		&fields.runStatus,
		&fields.runAttempt,
		&fields.runRecoveryCount,
		&fields.runPreviousRunID,
		&fields.runFailureKind,
		&fields.runClaimedByKind,
		&fields.runClaimedByRef,
		&fields.runSessionID,
		&fields.runLeaseUntil,
		&fields.runHeartbeatAt,
		&fields.runNetworkSpecJSON,
		&fields.runNetworkMode,
		&fields.runNetworkChannel,
		&fields.runNetworkSource,
		&fields.runQueuedAt,
		&fields.runClaimedAt,
		&fields.runStartedAt,
		&fields.runEndedAt,
		&fields.runError,
		&fields.lane,
		&fields.isUnread,
		&fields.triageArchived,
		&fields.triageRead,
		&fields.triageDismissed,
		&fields.triageLastSeenActivityAt,
		&fields.triageUpdatedAt,
	); err != nil {
		return fmt.Errorf("store: scan task inbox row: %w", err)
	}
	return nil
}

func taskInboxRunScanFields(fields *taskInboxScanFields) taskCatalogScanFields {
	return taskCatalogScanFields{
		activeRunID:              fields.runID,
		activeRunWorkspaceID:     fields.runWorkspaceID,
		activeRunStatus:          fields.runStatus,
		activeRunAttempt:         fields.runAttempt,
		activeRunRecoveryCount:   fields.runRecoveryCount,
		activeRunPreviousRunID:   fields.runPreviousRunID,
		activeRunFailureKind:     fields.runFailureKind,
		activeRunClaimedByKind:   fields.runClaimedByKind,
		activeRunClaimedByRef:    fields.runClaimedByRef,
		activeRunSessionID:       fields.runSessionID,
		activeRunLeaseUntil:      fields.runLeaseUntil,
		activeRunHeartbeatAt:     fields.runHeartbeatAt,
		activeRunNetworkSpecJSON: fields.runNetworkSpecJSON,
		activeRunNetworkMode:     fields.runNetworkMode,
		activeRunNetworkChannel:  fields.runNetworkChannel,
		activeRunNetworkSource:   fields.runNetworkSource,
		activeRunQueuedAt:        fields.runQueuedAt,
		activeRunClaimedAt:       fields.runClaimedAt,
		activeRunStartedAt:       fields.runStartedAt,
		activeRunEndedAt:         fields.runEndedAt,
		activeRunError:           fields.runError,
	}
}

func taskInboxBlockingReason(item taskpkg.InboxItem, runStatus sql.NullString) string {
	if item.ApprovalPolicy == taskpkg.ApprovalPolicyManual {
		switch item.ApprovalState {
		case taskpkg.ApprovalStatePending:
			return "awaiting_approval"
		case taskpkg.ApprovalStateRejected:
			return "approval_rejected"
		}
	}
	if taskpkg.ParseRunStatus(runStatus.String).Normalize() == taskpkg.TaskRunStatusFailed {
		return "latest_run_failed"
	}
	if item.Task.Status == taskpkg.TaskStatusBlocked {
		return "awaiting_dependencies"
	}
	return ""
}
