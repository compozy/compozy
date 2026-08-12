package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/observe"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

// ParseTaskRunListQuery parses the shared task-run list query parameters.
func ParseTaskRunListQuery(c *gin.Context) (contract.TaskRunListQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return contract.TaskRunListQuery{}, NewTaskValidationError(err)
	}

	return contract.TaskRunListQuery{
		Status:               taskpkg.ParseRunStatus(c.Query("status")).Normalize(),
		SessionID:            strings.TrimSpace(c.Query("session_id")),
		ParticipationChannel: strings.TrimSpace(c.Query("participation_channel")),
		Limit:                limit,
	}, nil
}

// ParseTaskTimelineQuery parses the shared task timeline query parameters.
func ParseTaskTimelineQuery(c *gin.Context) (contract.TaskTimelineQuery, error) {
	afterSequence, err := ParseOptionalInt64(c.Query("after_sequence"))
	if err != nil {
		return contract.TaskTimelineQuery{}, NewTaskValidationError(err)
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return contract.TaskTimelineQuery{}, NewTaskValidationError(err)
	}

	return contract.TaskTimelineQuery{
		AfterSequence: afterSequence,
		Limit:         limit,
	}, nil
}

// ParseTaskStreamQuery parses the shared task stream query parameters.
func ParseTaskStreamQuery(c *gin.Context) (contract.TaskStreamQuery, error) {
	afterSequence, err := ParseOptionalInt64(c.Query("after_sequence"))
	if err != nil {
		return contract.TaskStreamQuery{}, NewTaskValidationError(err)
	}

	return contract.TaskStreamQuery{AfterSequence: afterSequence}, nil
}

// ParseTaskDashboardQuery parses the shared task dashboard query parameters.
func ParseTaskDashboardQuery(c *gin.Context) (contract.TaskDashboardQuery, error) {
	query := contract.TaskDashboardQuery{
		Scope:                taskpkg.Scope(strings.TrimSpace(c.Query("scope"))).Normalize(),
		Workspace:            strings.TrimSpace(c.Query("workspace")),
		Worktree:             strings.TrimSpace(c.Query("worktree")),
		OwnerKind:            taskpkg.OwnerKind(strings.TrimSpace(c.Query("owner_kind"))).Normalize(),
		OwnerRef:             strings.TrimSpace(c.Query("owner_ref")),
		ParticipationChannel: strings.TrimSpace(c.Query("participation_channel")),
		OriginKind:           taskpkg.OriginKind(strings.TrimSpace(c.Query("origin_kind"))).Normalize(),
	}
	if err := validateParsedTaskDashboardQuery(query); err != nil {
		return contract.TaskDashboardQuery{}, err
	}
	return query, nil
}

func validateParsedTaskDashboardQuery(query contract.TaskDashboardQuery) error {
	if strings.TrimSpace(query.Worktree) != "" && strings.TrimSpace(query.Workspace) == "" {
		return NewTaskValidationError(fmt.Errorf(
			"%w: task_dashboard_query.workspace is required with worktree",
			taskpkg.ErrValidation,
		))
	}
	summaryQuery := observe.TaskSummaryQuery{
		Scope:                query.Scope,
		OwnerKind:            query.OwnerKind,
		OwnerRef:             query.OwnerRef,
		ParticipationChannel: query.ParticipationChannel,
		OriginKind:           query.OriginKind,
	}
	if err := summaryQuery.Validate(); err != nil {
		return NewTaskValidationError(err)
	}
	return nil
}

func taskRunListDomainQuery(query contract.TaskRunListQuery) (taskpkg.RunQuery, error) {
	domainQuery := taskpkg.RunQuery{
		Status:               query.Status.Normalize(),
		SessionID:            strings.TrimSpace(query.SessionID),
		ParticipationChannel: strings.TrimSpace(query.ParticipationChannel),
		Limit:                query.Limit,
	}
	if err := domainQuery.Validate("task_run_query"); err != nil {
		return taskpkg.RunQuery{}, err
	}
	return domainQuery, nil
}

func parseTaskRunListQuery(c *gin.Context) (taskpkg.RunQuery, error) {
	query, err := ParseTaskRunListQuery(c)
	if err != nil {
		return taskpkg.RunQuery{}, err
	}
	return taskRunListDomainQuery(query)
}

func taskTimelineDomainQuery(query contract.TaskTimelineQuery) (taskpkg.TimelineQuery, error) {
	domainQuery := taskpkg.TimelineQuery{
		AfterSequence: query.AfterSequence,
		Limit:         query.Limit,
	}
	if err := domainQuery.Validate("task_timeline_query"); err != nil {
		return taskpkg.TimelineQuery{}, err
	}
	return domainQuery, nil
}

func (h *BaseHandlers) taskStreamDomainQuery(
	c *gin.Context,
	query contract.TaskStreamQuery,
) (taskpkg.StreamQuery, error) {
	domainQuery := taskpkg.StreamQuery{AfterSequence: query.AfterSequence}

	if rawLastEventID := strings.TrimSpace(c.GetHeader("Last-Event-ID")); rawLastEventID != "" {
		afterSequence, err := parseLastEventID(rawLastEventID, h.transportName())
		if err != nil {
			return taskpkg.StreamQuery{}, NewTaskValidationError(err)
		}
		domainQuery.AfterSequence = afterSequence
	}

	if err := domainQuery.Validate("task_stream_query"); err != nil {
		return taskpkg.StreamQuery{}, err
	}
	return domainQuery, nil
}

func (h *BaseHandlers) taskDashboardDomainQuery(
	ctx context.Context,
	query contract.TaskDashboardQuery,
) (observe.TaskDashboardQuery, error) {
	domainQuery := observe.TaskDashboardQuery{
		Scope:                query.Scope.Normalize(),
		WorktreeID:           strings.TrimSpace(query.Worktree),
		OwnerKind:            query.OwnerKind.Normalize(),
		OwnerRef:             strings.TrimSpace(query.OwnerRef),
		ParticipationChannel: strings.TrimSpace(query.ParticipationChannel),
		OriginKind:           query.OriginKind.Normalize(),
	}

	if workspaceRef := strings.TrimSpace(query.Workspace); workspaceRef != "" {
		if err := taskpkg.ValidateScopeBinding(
			domainQuery.Scope,
			workspaceRef,
			"task_dashboard_query",
			"workspace",
		); err != nil {
			return observe.TaskDashboardQuery{}, err
		}
		if domainQuery.Scope.Normalize() == taskpkg.ScopeWorkspace {
			workspaceID, err := h.lookupWorkspaceID(ctx, workspaceRef)
			if err != nil {
				return observe.TaskDashboardQuery{}, err
			}
			domainQuery.WorkspaceID = workspaceID
		}
	}

	if err := validateParticipationChannel(
		"task_dashboard_query.participation_channel",
		domainQuery.ParticipationChannel,
	); err != nil {
		return observe.TaskDashboardQuery{}, err
	}
	if err := domainQuery.Validate(); err != nil {
		return observe.TaskDashboardQuery{}, err
	}
	return domainQuery, nil
}
