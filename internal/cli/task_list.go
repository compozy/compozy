package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/network"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/spf13/cobra"
)

const taskLoopColumn = "LOOP"

type taskListOptions struct {
	scopeRaw                string
	workspaceRef            string
	statusRaw               string
	priorityRaw             string
	ownerKindRaw            string
	ownerRef                string
	parentTaskID            string
	worktreeID              string
	participationChannelRaw string
	includeLoop             bool
	loopRunID               string
	queryRaw                string
	sortRaw                 string
	cursor                  string
	limit                   int
}

func newTaskListCommand(deps commandDeps) *cobra.Command {
	opts := &taskListOptions{}
	cmd := &cobra.Command{
		Use:   taskListKey,
		Short: "List tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return opts.run(cmd, deps)
		},
	}
	addTaskListFlags(cmd, opts)
	return cmd
}

func (o *taskListOptions) run(cmd *cobra.Command, deps commandDeps) error {
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	query, err := parseTaskListFilters(
		cmd,
		deps,
		client,
		o.scopeRaw,
		o.workspaceRef,
		o.statusRaw,
		o.priorityRaw,
		o.ownerKindRaw,
		o.ownerRef,
		o.parentTaskID,
		o.worktreeID,
		o.includeLoop,
		o.loopRunID,
		o.participationChannelRaw,
		o.queryRaw,
		o.sortRaw,
		o.cursor,
		o.limit,
	)
	if err != nil {
		return err
	}
	page, err := client.ListTasks(cmd.Context(), query)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, taskSummaryListBundle(page))
}

func addTaskListFlags(cmd *cobra.Command, opts *taskListOptions) {
	cmd.Flags().StringVar(&opts.scopeRaw, taskScopeKey, "", "Filter by scope: global or workspace")
	cmd.Flags().
		StringVar(&opts.workspaceRef, "workspace", "", "Override workspace filter (ID, name, or path)")
	cmd.Flags().StringVar(&opts.statusRaw, taskStatusKey, "", "Filter by task status")
	cmd.Flags().StringVar(&opts.priorityRaw, "priority", "", "Filter by task priority")
	cmd.Flags().StringVar(&opts.ownerKindRaw, "owner-kind", "", "Filter by owner kind")
	cmd.Flags().StringVar(&opts.ownerRef, "owner-ref", "", "Filter by owner reference")
	cmd.Flags().StringVar(&opts.parentTaskID, "parent", "", "Filter by parent task ID")
	cmd.Flags().StringVar(&opts.worktreeID, "worktree", "", "Filter by active run worktree ID")
	cmd.Flags().BoolVar(&opts.includeLoop, "include-loop", false, "Include Loop execution records")
	cmd.Flags().StringVar(&opts.loopRunID, "loop-run", "", "Filter by Loop run ID")
	cmd.Flags().StringVar(
		&opts.participationChannelRaw,
		"participation-channel",
		"",
		"Filter by resolved participation channel",
	)
	cmd.Flags().StringVar(&opts.queryRaw, "query", "", "Search task title or identifier")
	cmd.Flags().StringVar(&opts.sortRaw, "sort", "recent", "Sort by recent or priority")
	cmd.Flags().StringVar(&opts.cursor, "cursor", "", "Continue from an opaque catalog cursor")
	cmd.Flags().IntVar(&opts.limit, "limit", 0, "Maximum tasks to return (1-200)")
}

func parseTaskListFilters(
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
	scopeRaw string,
	workspaceRef string,
	statusRaw string,
	priorityRaw string,
	ownerKindRaw string,
	ownerRef string,
	parentTaskID string,
	worktreeID string,
	includeLoop bool,
	loopRunID string,
	participationChannelRaw string,
	queryRaw string,
	sortRaw string,
	cursor string,
	limit int,
) (TaskListQuery, error) {
	catalogScope, workspace, err := resolveTaskListScopeWorkspace(cmd, deps, client, scopeRaw, workspaceRef)
	if err != nil {
		return TaskListQuery{}, err
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
	trimmedWorktreeID := strings.TrimSpace(worktreeID)
	if trimmedWorktreeID != "" && strings.TrimSpace(workspace) == "" {
		return TaskListQuery{}, errors.New("cli: --worktree requires a workspace scope or --workspace")
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
	return TaskListQuery{
		Scope:                catalogScope,
		Workspace:            workspace,
		Status:               status,
		Priority:             priority,
		OwnerKind:            ownerKind,
		OwnerRef:             trimmedOwnerRef,
		ParentTaskID:         strings.TrimSpace(parentTaskID),
		Worktree:             trimmedWorktreeID,
		IncludeLoop:          includeLoop || strings.TrimSpace(loopRunID) != "",
		LoopRunID:            strings.TrimSpace(loopRunID),
		ParticipationChannel: strings.TrimSpace(participationChannelRaw),
		Query:                strings.TrimSpace(queryRaw),
		Sort:                 sortKey,
		Cursor:               strings.TrimSpace(cursor),
		Limit:                limit,
	}, nil
}

func resolveTaskListScopeWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	client workspaceLookupClient,
	scopeRaw string,
	workspaceRef string,
) (taskpkg.CatalogScope, string, error) {
	if strings.TrimSpace(strings.ToLower(scopeRaw)) != string(taskpkg.CatalogScopeAll) {
		scope, workspace, err := resolveTaskScopeWorkspace(cmd, deps, client, scopeRaw, workspaceRef, false)
		return taskpkg.CatalogScope(scope), workspace, err
	}
	workspace := strings.TrimSpace(workspaceRef)
	if workspace == "" {
		return taskpkg.CatalogScopeAll, "", nil
	}
	resolution, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: workspace},
	)
	if err != nil {
		return "", "", err
	}
	return taskpkg.CatalogScopeAll, resolution.ID, nil
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
			strings.ToUpper(taskIdentifierValue),
			strings.ToUpper(taskScopeValue),
			strings.ToUpper(taskWorkspaceValue),
			strings.ToUpper(taskParentValue),
			strings.ToUpper(taskStatusValue),
			strings.ToUpper(taskOwnerValue),
			strings.ToUpper(taskParticipationChannelValue),
			taskLoopColumn,
			strings.ToUpper(taskTitleValue),
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
			loopLoopKey,
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
				stringOrDash(formatTaskLoopProvenance(item.Loop)),
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
				formatTaskLoopProvenance(item.Loop),
				item.Title,
			}
		},
	)
}

func formatTaskLoopProvenance(provenance *contract.LoopProvenance) string {
	if provenance == nil {
		return ""
	}
	identity := strings.TrimSpace(provenance.LoopName)
	if identity == "" {
		identity = strings.TrimSpace(provenance.RunID)
	}
	suffix := taskLoopCoordinatorLabel
	if provenance.Role == contract.LoopProvenanceRoleCell {
		suffix = "cell"
		if provenance.Generation != nil {
			suffix = fmt.Sprintf("g%d", *provenance.Generation)
		}
	}
	return identity + " · " + suffix
}

const taskLoopCoordinatorLabel = "run"
