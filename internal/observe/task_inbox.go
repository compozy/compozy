package observe

import (
	"context"
	"errors"
	"fmt"

	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	TaskInboxLaneMyWork     = taskpkg.InboxLaneMyWork
	TaskInboxLaneApprovals  = taskpkg.InboxLaneApprovals
	TaskInboxLaneFailedRuns = taskpkg.InboxLaneFailedRuns
	TaskInboxLaneBlocked    = taskpkg.InboxLaneBlocked
	TaskInboxLaneArchived   = taskpkg.InboxLaneArchived

	taskInboxBlockingReasonAwaitingApproval = "awaiting_approval"
	taskInboxBlockingReasonApprovalRejected = "approval_rejected"
	taskInboxBlockingReasonAwaitingDeps     = "awaiting_dependencies"
	taskInboxBlockingReasonLatestRunFailed  = "latest_run_failed"
)

type TaskInboxLane = taskpkg.InboxLane
type TaskInboxQuery = taskpkg.InboxQuery
type TaskInboxItem = taskpkg.InboxItem
type TaskInboxLaneGroup = taskpkg.InboxLaneGroup
type TaskInboxView = taskpkg.InboxPage

// QueryTaskInbox returns one bounded actor-scoped inbox page.
func (o *Observer) QueryTaskInbox(
	ctx context.Context,
	query TaskInboxQuery,
	actor taskpkg.ActorIdentity,
) (TaskInboxView, error) {
	if ctx == nil {
		return TaskInboxView{}, errors.New("observe: task inbox context is required")
	}
	reader, ok := o.registry.(taskpkg.InboxReader)
	if !ok {
		return TaskInboxView{}, fmt.Errorf("observe: task inbox reader is not configured")
	}
	return reader.ListTaskInbox(ctx, query, actor)
}
