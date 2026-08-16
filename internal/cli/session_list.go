package cli

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/spf13/cobra"
)

type sessionListFlags struct {
	includeAll      bool
	workspaceFilter string
	worktreeFilter  string
	stateFilter     string
	typeFilter      string
	agentFilter     string
	parentFilter    string
	rootFilter      string
	search          string
	resumable       bool
	attention       bool
	badgeFilters    []string
	allWorkspaces   bool
	summary         bool
	archived        bool
	includeArchived bool
	includeHealth   bool
	limit           int
	sortKey         string
	cursor          string
}

func newSessionListCommand(deps commandDeps) *cobra.Command {
	flags := sessionListFlags{}
	cmd := &cobra.Command{
		Use:   sessionListKey,
		Short: "List sessions",
		Example: `  # List active sessions
  compozy session list

  # Page all sessions for one agent in a workspace
  compozy session list --all --workspace checkout-api --agent reviewer --limit 25`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSessionListCommand(cmd, deps, flags)
		},
	}
	cmd.Flags().BoolVar(&flags.includeAll, "all", false, "Include every session state when --state is omitted")
	cmd.Flags().
		StringVar(&flags.workspaceFilter, workspaceSkillSource, "", "Override workspace filter (ID, name, or path)")
	cmd.Flags().StringVar(&flags.worktreeFilter, "worktree", "", "Filter by exact worktree ID")
	cmd.Flags().StringVar(&flags.stateFilter, "state", "", "Filter by state (starting|active|stopping|stopped)")
	cmd.Flags().StringVar(
		&flags.typeFilter,
		"type",
		"",
		"Filter by session type (user|system|coordinator|spawned)",
	)
	cmd.Flags().StringVar(&flags.agentFilter, "agent", "", "Filter by exact agent definition name")
	cmd.Flags().StringVar(&flags.parentFilter, "parent", "", "Filter by exact parent session id")
	cmd.Flags().StringVar(&flags.rootFilter, "root", "", "Filter by exact root session id (includes the root itself)")
	cmd.Flags().StringVar(&flags.search, "query", "", "Search session id, name, agent, provider, or channel")
	cmd.Flags().BoolVar(&flags.resumable, "resumable", false, "Show only sessions eligible for resume attach")
	cmd.Flags().BoolVar(&flags.attention, "attention", false, "Show only sessions that need operator attention")
	cmd.Flags().StringSliceVar(&flags.badgeFilters, "badge", nil, "Filter by exact badge (repeat or comma-separate)")
	cmd.Flags().BoolVar(&flags.allWorkspaces, "all-workspaces", false, "List sessions across every workspace")
	cmd.Flags().BoolVar(&flags.summary, "summary", false, "Show exact attention counts across workspaces")
	cmd.Flags().BoolVar(&flags.archived, "archived", false, "Show only archived sessions")
	cmd.Flags().BoolVar(&flags.includeArchived, "include-archived", false, "Include archived sessions")
	cmd.Flags().
		BoolVar(&flags.includeHealth, "include-health", false, "Include metadata-only health for returned sessions")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Sessions per page (1-100)")
	cmd.Flags().StringVar(&flags.sortKey, "sort", "", "Sort by recent, last_activity, or attention")
	cmd.Flags().StringVar(&flags.cursor, "cursor", "", "Continue from an opaque next_cursor")
	return cmd
}

func runSessionListCommand(cmd *cobra.Command, deps commandDeps, flags sessionListFlags) error {
	badges, err := validateSessionListFlags(cmd, flags)
	if err != nil {
		return err
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	if flags.summary {
		summary, summaryErr := client.GetSessionAttentionSummary(cmd.Context())
		if summaryErr != nil {
			return summaryErr
		}
		return writeCommandOutput(cmd, sessionAttentionSummaryBundle(summary))
	}
	workspaceID := ""
	if !flags.allWorkspaces {
		workspaceID, err = resolveOptionalWorkspaceOverride(
			cmd.Context(),
			cmd,
			deps,
			client,
			flags.workspaceFilter,
		)
		if err != nil {
			return err
		}
	}
	state := strings.TrimSpace(flags.stateFilter)
	if state == "" && !flags.includeAll && !flags.resumable && !flags.attention && len(badges) == 0 &&
		!flags.archived && !flags.includeArchived {
		state = string(session.StateActive)
	}
	archive := string(session.ArchiveExclude)
	if flags.archived {
		archive = string(session.ArchiveOnly)
	} else if flags.includeArchived {
		archive = string(session.ArchiveInclude)
	}
	sort := strings.TrimSpace(flags.sortKey)
	if sort == "" && flags.resumable {
		sort = session.ListSortLastActivity
	} else if sort == "" && (flags.attention || len(badges) > 0) {
		sort = session.ListSortAttention
	}
	page, err := client.ListSessions(cmd.Context(), SessionListQuery{
		Workspace:     workspaceID,
		Worktree:      flags.worktreeFilter,
		State:         state,
		Type:          flags.typeFilter,
		Agent:         flags.agentFilter,
		Parent:        flags.parentFilter,
		Root:          flags.rootFilter,
		Query:         flags.search,
		Resumable:     flags.resumable,
		Attention:     flags.attention,
		Badges:        badges,
		Archive:       archive,
		IncludeHealth: flags.includeHealth,
		Limit:         flags.limit,
		Sort:          sort,
		Cursor:        flags.cursor,
	})
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, sessionListBundle(page, deps.now))
}

func validateSessionListFlags(cmd *cobra.Command, flags sessionListFlags) ([]session.Badge, error) {
	if flags.archived && flags.includeArchived {
		return nil, errors.New("cli: --archived and --include-archived are mutually exclusive")
	}
	if flags.allWorkspaces && cmd.Flags().Changed(workspaceSkillSource) {
		return nil, errors.New("cli: --all-workspaces and --workspace are mutually exclusive")
	}
	badges, err := session.ParseBadgeFilters(flags.badgeFilters)
	if err != nil {
		return nil, withCommandExitCode(65, err)
	}
	if flags.attention && len(badges) > 0 {
		return nil, errors.New("cli: --attention and --badge are mutually exclusive")
	}
	if (flags.attention || len(badges) > 0) && flags.sortKey != "" && flags.sortKey != session.ListSortAttention {
		return nil, errors.New("cli: attention filters require --sort attention")
	}
	if flags.summary && sessionListHasRowFilters(flags) {
		return nil, errors.New("cli: --summary cannot be combined with session row filters")
	}
	return badges, nil
}

func sessionListHasRowFilters(flags sessionListFlags) bool {
	return flags.includeAll || strings.TrimSpace(flags.workspaceFilter) != "" ||
		strings.TrimSpace(flags.worktreeFilter) != "" || strings.TrimSpace(flags.stateFilter) != "" ||
		strings.TrimSpace(flags.typeFilter) != "" || strings.TrimSpace(flags.agentFilter) != "" ||
		strings.TrimSpace(flags.parentFilter) != "" || strings.TrimSpace(flags.rootFilter) != "" ||
		strings.TrimSpace(flags.search) != "" || flags.resumable || flags.attention || len(flags.badgeFilters) > 0 ||
		flags.archived || flags.includeArchived || flags.includeHealth || flags.limit != 0 ||
		strings.TrimSpace(flags.sortKey) != "" || strings.TrimSpace(flags.cursor) != ""
}

func sessionListBundle(page SessionListPage, now func() time.Time) outputBundle {
	items := page.Sessions
	bundle := listBundle(
		page,
		items,
		"Sessions",
		[]string{
			"ID", sessionNameValue, sessionAgentValue, "Parent", sessionProviderValue, sessionBackendValue,
			sessionStateValue, sessionBadgeValue, "Failure", sessionWorkspaceValue, sessionChannelValue,
			"Health State", "Health", sessionUpdatedValue,
		},
		"sessions",
		[]string{
			"id", sessionNameKey, sessionAgentNameKey, "parent_session_id", sessionProviderKey, "sandbox_backend",
			sessionStateKey, sessionBadgeKey, taskFailureKindKey, workspaceSkillSource, sessionChannelKey,
			"health_state", extensionHealthKey, sessionUpdatedAtKey,
		},
		func(item SessionRecord) []string {
			return []string{
				stringOrDash(item.ID), stringOrDash(item.Name), stringOrDash(item.AgentName),
				stringOrDash(sessionParentID(item)),
				stringOrDash(sessionRuntimeProvider(item)), stringOrDash(sessionSandboxBackend(item)),
				stringOrDash(string(item.State)), stringOrDash(string(item.Badge)),
				stringOrDash(sessionFailureKind(item)), stringOrDash(displaySessionWorkspace(item)),
				stringOrDash(sessionResolvedChannelRaw(item)), stringOrDash(sessionHealthState(item)),
				stringOrDash(sessionHealthStatus(item)), stringOrDash(formatAge(now, item.UpdatedAt)),
			}
		},
		func(item SessionRecord) []string {
			return []string{
				item.ID, item.Name, item.AgentName, sessionParentID(item),
				sessionRuntimeProvider(item), sessionSandboxBackend(item),
				string(item.State), string(item.Badge), sessionFailureKind(item),
				displaySessionWorkspace(item), sessionResolvedChannelRaw(item), sessionHealthState(item),
				sessionHealthStatus(item), formatTime(item.UpdatedAt),
			}
		},
	)
	bundle.jsonl = func(cmd *cobra.Command) error {
		for _, item := range page.Sessions {
			if err := writeJSONLine(cmd, item); err != nil {
				return err
			}
		}
		return writeJSONLine(cmd, struct {
			Type string                            `json:"type"`
			Page contract.CountedCursorPagePayload `json:"page"`
		}{Type: listPageRecordType, Page: page.Page})
	}
	baseHuman := bundle.human
	bundle.human = func() (string, error) {
		table, err := baseHuman()
		if err != nil {
			return "", err
		}
		return renderHumanBlocks(table, sessionPageHuman(page.Page)), nil
	}
	baseToon := bundle.toon
	bundle.toon = func() (string, error) {
		itemsToon, err := baseToon()
		if err != nil {
			return "", err
		}
		pageToon := renderToonObject(
			listPageRecordType,
			[]string{
				listReturnedField,
				listTotalField,
				listLimitField,
				listHasMoreField,
				listNextCursorField,
			},
			[]string{
				strconv.Itoa(len(page.Sessions)), strconv.Itoa(page.Page.Total), strconv.Itoa(page.Page.Limit),
				strconv.FormatBool(page.Page.HasMore), page.Page.NextCursor,
			},
		)
		return itemsToon + "\n" + pageToon, nil
	}
	return bundle
}

func sessionParentID(item SessionRecord) string {
	if item.Lineage == nil {
		return ""
	}
	return strings.TrimSpace(item.Lineage.ParentSessionID)
}

func sessionHealthState(item SessionRecord) string {
	if item.Health == nil {
		return ""
	}
	return string(item.Health.State)
}

func sessionHealthStatus(item SessionRecord) string {
	if item.Health == nil {
		return ""
	}
	return string(item.Health.Health)
}

func sessionPageHuman(page contract.CountedCursorPagePayload) string {
	return renderHumanSection("Page", []keyValue{
		{Label: "Total", Value: strconv.Itoa(page.Total)},
		{Label: "Limit", Value: strconv.Itoa(page.Limit)},
		{Label: "Has More", Value: strconv.FormatBool(page.HasMore)},
		{Label: "Next Cursor", Value: stringOrDash(page.NextCursor)},
	})
}
