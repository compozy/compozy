package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/spf13/cobra"
)

func newTaskListCommand(deps commandDeps) *cobra.Command {
	var (
		scopeRaw                string
		workspaceRef            string
		statusRaw               string
		priorityRaw             string
		ownerKindRaw            string
		ownerRef                string
		parentTaskID            string
		participationChannelRaw string
		queryRaw                string
		sortRaw                 string
		cursor                  string
		limit                   int
	)

	cmd := &cobra.Command{
		Use:   taskListKey,
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			query, err := parseTaskListFilters(
				scopeRaw,
				workspaceRef,
				statusRaw,
				priorityRaw,
				ownerKindRaw,
				ownerRef,
				parentTaskID,
				participationChannelRaw,
				queryRaw,
				sortRaw,
				cursor,
				limit,
			)
			if err != nil {
				return err
			}

			page, err := client.ListTasks(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, taskSummaryListBundle(page))
		},
	}
	cmd.Flags().StringVar(&scopeRaw, taskScopeKey, "", "Filter by scope: global or workspace")
	cmd.Flags().StringVar(&workspaceRef, "workspace", "", "Filter by workspace path, name, or ID")
	cmd.Flags().StringVar(&statusRaw, taskStatusKey, "", "Filter by task status")
	cmd.Flags().StringVar(&priorityRaw, "priority", "", "Filter by task priority")
	cmd.Flags().StringVar(&ownerKindRaw, "owner-kind", "", "Filter by owner kind")
	cmd.Flags().StringVar(&ownerRef, "owner-ref", "", "Filter by owner reference")
	cmd.Flags().StringVar(&parentTaskID, "parent", "", "Filter by parent task ID")
	cmd.Flags().StringVar(
		&participationChannelRaw,
		"participation-channel",
		"",
		"Filter by resolved participation channel",
	)
	cmd.Flags().StringVar(&queryRaw, "query", "", "Search task title or identifier")
	cmd.Flags().StringVar(&sortRaw, "sort", "recent", "Sort by recent or priority")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue from an opaque catalog cursor")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum tasks to return (1-200)")
	return cmd
}

func parseTaskListFilters(
	scopeRaw string,
	workspaceRef string,
	statusRaw string,
	priorityRaw string,
	ownerKindRaw string,
	ownerRef string,
	parentTaskID string,
	participationChannelRaw string,
	queryRaw string,
	sortRaw string,
	cursor string,
	limit int,
) (TaskListQuery, error) {
	scope, workspace, err := resolveTaskScopeWorkspace(scopeRaw, workspaceRef, false)
	if err != nil {
		if strings.TrimSpace(strings.ToLower(scopeRaw)) != string(taskpkg.CatalogScopeAll) {
			return TaskListQuery{}, err
		}
		scope = ""
		workspace = strings.TrimSpace(workspaceRef)
	}
	status, err := parseOptionalTaskStatus(statusRaw)
	if err != nil {
		return TaskListQuery{}, err
	}
	var priority taskpkg.Priority
	if strings.TrimSpace(priorityRaw) != "" {
		priority, err = parseOptionalTaskPriority(priorityRaw)
		if err != nil {
			return TaskListQuery{}, err
		}
	}
	ownerKind, err := parseOptionalTaskOwnerKind(ownerKindRaw)
	if err != nil {
		return TaskListQuery{}, err
	}
	trimmedOwnerRef := strings.TrimSpace(ownerRef)
	if (ownerKind != "" && trimmedOwnerRef == "") || (ownerKind == "" && trimmedOwnerRef != "") {
		return TaskListQuery{}, errors.New(
			"cli: --owner-kind and --owner-ref must be provided together",
		)
	}
	if err := validateTaskParticipationChannelFlag(participationChannelRaw); err != nil {
		return TaskListQuery{}, err
	}
	if err := validateTaskLast(limit); err != nil {
		return TaskListQuery{}, err
	}
	sortKey := taskpkg.CatalogSort(strings.ToLower(strings.TrimSpace(sortRaw))).Normalize()
	if sortKey == "" {
		sortKey = taskpkg.CatalogSortRecent
	}
	if sortKey != taskpkg.CatalogSortRecent && sortKey != taskpkg.CatalogSortPriority {
		return TaskListQuery{}, errors.New("cli: --sort must be recent or priority")
	}
	catalogScope := taskpkg.CatalogScope(scope)
	if strings.TrimSpace(strings.ToLower(scopeRaw)) == string(taskpkg.CatalogScopeAll) {
		catalogScope = taskpkg.CatalogScopeAll
	}

	return TaskListQuery{
		Scope:                catalogScope,
		Workspace:            workspace,
		Status:               status,
		Priority:             priority,
		OwnerKind:            ownerKind,
		OwnerRef:             trimmedOwnerRef,
		ParentTaskID:         strings.TrimSpace(parentTaskID),
		ParticipationChannel: strings.TrimSpace(participationChannelRaw),
		Query:                strings.TrimSpace(queryRaw),
		Sort:                 sortKey,
		Cursor:               strings.TrimSpace(cursor),
		Limit:                limit,
	}, nil
}

func validateTaskParticipationChannelFlag(channel string) error {
	trimmed := strings.TrimSpace(channel)
	if trimmed == "" {
		return nil
	}
	if err := network.ValidateChannel(trimmed); err != nil {
		return fmt.Errorf("cli: invalid --participation-channel value %q: %w", trimmed, err)
	}
	return nil
}

func taskSummaryListBundle(page TaskListRecord) outputBundle {
	return listBundle(
		page,
		page.Tasks,
		"Tasks",
		[]string{
			"ID",
			taskIdentifierValue,
			taskScopeValue,
			taskWorkspaceValue,
			taskParentValue,
			taskStatusValue,
			taskOwnerValue,
			taskParticipationChannelValue,
			taskTitleValue,
		},
		"tasks",
		[]string{
			"id",
			taskIdentifierKey,
			taskScopeKey,
			taskWorkspaceIDKey,
			"parent_task_id",
			taskStatusKey,
			taskOwnerKey,
			taskParticipationChannelKey,
			taskTitleKey,
		},
		func(item TaskCatalogItemRecord) []string {
			return []string{
				stringOrDash(item.ID),
				stringOrDash(item.Identifier),
				stringOrDash(string(item.Scope)),
				stringOrDash(item.WorkspaceID),
				stringOrDash(item.ParentTaskID),
				stringOrDash(string(item.Status)),
				stringOrDash(formatTaskOwnership(item.Owner)),
				stringOrDash(resolvedParticipationChannel(item.ResolvedNetworkParticipation)),
				stringOrDash(item.Title),
			}
		},
		func(item TaskCatalogItemRecord) []string {
			return []string{
				item.ID,
				item.Identifier,
				string(item.Scope),
				item.WorkspaceID,
				item.ParentTaskID,
				string(item.Status),
				formatTaskOwnership(item.Owner),
				resolvedParticipationChannelRaw(item.ResolvedNetworkParticipation),
				item.Title,
			}
		},
	)
}
