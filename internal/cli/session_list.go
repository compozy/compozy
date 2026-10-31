package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/spf13/cobra"
)

const allWorkspacesFlagName = "all-workspaces"
const sessionSubagentsInclude = "include"

type sessionListFlags struct {
	columns         []string
	hideChildren    bool
	subagents       string
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
		Long: "List sessions. Verdict-only system judges are hidden by default; " +
			"use --type system to include them for audit. Other system sessions remain visible.",
		Example: `  # List active sessions
  compozy session list

  # Page all sessions for one agent in a workspace
  compozy session list --all --workspace checkout-api --agent reviewer --limit 25`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSessionListCommand(cmd, deps, flags)
		},
	}
	cmd.Flags().StringVar(
		&flags.subagents, "subagents", sessionSubagentsInclude,
		"Subagent session visibility (include|exclude|only)",
	)
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
	cmd.Flags().StringVar(&flags.search, "query", "", "Search session id, name, agent, or provider")
	cmd.Flags().BoolVar(&flags.resumable, "resumable", false, "Show only sessions eligible for resume attach")
	cmd.Flags().BoolVar(&flags.attention, "attention", false, "Show only sessions that need operator attention")
	cmd.Flags().StringSliceVar(&flags.badgeFilters, "badge", nil, "Filter by exact badge (repeat or comma-separate)")
	cmd.Flags().BoolVar(&flags.allWorkspaces, allWorkspacesFlagName, false, "List sessions across every workspace")
	cmd.Flags().BoolVar(&flags.summary, "summary", false, "Show exact attention counts across workspaces")
	cmd.Flags().BoolVar(&flags.archived, "archived", false, "Show only archived sessions")
	cmd.Flags().BoolVar(&flags.includeArchived, "include-archived", false, "Include archived sessions")
	cmd.Flags().
		BoolVar(&flags.includeHealth, "include-health", false, "Include metadata-only health for returned sessions")
	cmd.Flags().IntVar(&flags.limit, "limit", 0, "Sessions per page (1-100)")
	cmd.Flags().
		StringVar(&flags.sortKey, "sort", "", "Sort by recent, created, name, last_activity, attention, navigator, or manual")
	cmd.Flags().StringVar(&flags.cursor, "cursor", "", "Continue from an opaque next_cursor")
	cmd.Flags().
		StringSliceVar(&flags.columns, "column", nil, "Filter board columns (needs_you|working|waiting|in_review|done)")
	cmd.Flags().
		BoolVar(&flags.hideChildren, "hide-children", false, "Exclude child sessions; forks and continuations stay")
	configureProfileReadCommand(cmd, deps)
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
	page, err := client.ListSessions(cmd.Context(), buildSessionListQuery(flags, badges, workspaceID))
	if err != nil {
		return err
	}
	format, err := resolveInheritedOutputFormat(cmd)
	if err != nil {
		return err
	}
	if flags.allWorkspaces && len(page.Sessions) > 0 && format != OutputJSON && format != OutputJSONL {
		labels, err := aggregateProjectLabels(cmd, client)
		if err != nil {
			return err
		}
		return writeCommandOutput(cmd, sessionListBundle(page, deps.now, labels))
	}
	return writeCommandOutput(cmd, sessionListBundle(page, deps.now))
}

func buildSessionListQuery(flags sessionListFlags, badges []session.Badge, workspaceID string) SessionListQuery {
	return SessionListQuery{
		Columns:       flags.columns,
		HideChildren:  flags.hideChildren,
		Subagents:     flags.subagents,
		Workspace:     workspaceID,
		Worktree:      flags.worktreeFilter,
		State:         sessionListState(flags, badges),
		Type:          flags.typeFilter,
		Agent:         flags.agentFilter,
		Parent:        flags.parentFilter,
		Root:          flags.rootFilter,
		Query:         flags.search,
		Resumable:     flags.resumable,
		Attention:     flags.attention,
		Badges:        badges,
		Archive:       sessionListArchive(flags),
		IncludeHealth: flags.includeHealth,
		Limit:         flags.limit,
		Sort:          sessionListSort(flags, badges),
		Cursor:        flags.cursor,
	}
}

func sessionListState(flags sessionListFlags, badges []session.Badge) string {
	state := strings.TrimSpace(flags.stateFilter)
	if state == "" && !flags.includeAll && !flags.resumable && !flags.attention && len(badges) == 0 &&
		len(flags.columns) == 0 &&
		!flags.archived &&
		!flags.includeArchived {
		return string(session.StateActive)
	}
	return state
}

func sessionListArchive(flags sessionListFlags) string {
	if flags.archived {
		return string(session.ArchiveOnly)
	}
	if flags.includeArchived {
		return string(session.ArchiveInclude)
	}
	return string(session.ArchiveExclude)
}

func sessionListSort(flags sessionListFlags, badges []session.Badge) string {
	sort := strings.TrimSpace(flags.sortKey)
	if sort == "" && flags.resumable {
		return session.ListSortLastActivity
	}
	if sort == "" && (flags.attention || len(badges) > 0) {
		return session.ListSortAttention
	}
	return sort
}

func validateSessionListFlags(cmd *cobra.Command, flags sessionListFlags) ([]session.Badge, error) {
	for _, column := range flags.columns {
		if _, err := session.ParseBoardColumns([]string{column}); err != nil {
			return nil, withCommandExitCode(
				2,
				fmt.Errorf("invalid --column %q (allowed: needs_you, working, waiting, in_review, done)", column),
			)
		}
	}
	switch flags.subagents {
	case "", sessionSubagentsInclude, "exclude", "only":
	default:
		return nil, errors.New("cli: --subagents must be include, exclude, or only")
	}
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
	return sessionListHasVisibilityFilters(flags) || strings.TrimSpace(flags.workspaceFilter) != "" ||
		strings.TrimSpace(flags.worktreeFilter) != "" || strings.TrimSpace(flags.stateFilter) != "" ||
		strings.TrimSpace(flags.typeFilter) != "" || strings.TrimSpace(flags.agentFilter) != "" ||
		strings.TrimSpace(flags.parentFilter) != "" || strings.TrimSpace(flags.rootFilter) != "" ||
		strings.TrimSpace(flags.search) != "" || flags.resumable || flags.attention || len(flags.badgeFilters) > 0 ||
		flags.archived || flags.includeArchived ||
		flags.includeHealth ||
		flags.limit != 0 ||
		strings.TrimSpace(flags.sortKey) != "" ||
		strings.TrimSpace(flags.cursor) != ""
}

func sessionListBundle(page SessionListPage, now func() time.Time, projectLabels ...map[string]string) outputBundle {
	headers := sessionListHumanHeaders()
	row := sessionListHumanRow(now)
	if len(projectLabels) > 0 {
		headers = append([]string{cliProjectHeader}, headers...)
		baseRow := row
		row = func(item SessionRecord) []string {
			return append([]string{aggregateProjectName(projectLabels[0], item.WorkspaceID)}, baseRow(item)...)
		}
	}
	items := page.Sessions
	bundle := listBundle(
		page,
		items,
		"Sessions",
		headers,
		"sessions",
		sessionListToonFields(),
		row,
		sessionListToonRow,
	)
	return decorateSessionListBundle(bundle, page)
}

func sessionListHumanHeaders() []string {
	return []string{
		"ID", sessionProfileValue, sessionNameValue, sessionAgentValue, "Parent", sessionProviderValue,
		sessionStateValue, sessionBadgeValue, "Failure", sessionWorkspaceValue,
		"Health State", "Health", sessionUpdatedValue, "BRANCH", "PR", "CHECKS", "REVIEW",
	}
}

func sessionListToonFields() []string {
	return []string{
		"id", profileNameOutputKey, sessionNameKey, sessionAgentNameKey, "parent_session_id", sessionProviderKey,
		sessionStateKey, sessionBadgeKey, taskFailureKindKey, workspaceSkillSource,
		"health_state", extensionHealthKey, sessionUpdatedAtKey, "branch", "pr", "checks", "review",
	}
}

func sessionListHumanRow(now func() time.Time) func(SessionRecord) []string {
	return func(item SessionRecord) []string {
		return []string{
			stringOrDash(item.ID),
			stringOrDash(item.ProfileName),
			stringOrDash(item.Name),
			stringOrDash(item.AgentName),
			stringOrDash(sessionParentID(&item)),
			stringOrDash(sessionRuntimeProvider(&item)),
			stringOrDash(string(item.State)),
			stringOrDash(string(item.Badge)),
			stringOrDash(sessionFailureKind(&item)),
			stringOrDash(displaySessionWorkspace(&item)),
			stringOrDash(sessionHealthState(&item)),
			stringOrDash(sessionHealthStatus(&item)),
			stringOrDash(formatAge(now, item.UpdatedAt)),
			stringOrDash(
				item.Branch,
			),
			stringOrDash(sessionPRNumber(&item)),
			stringOrDash(sessionPRChecks(&item)),
			stringOrDash(sessionPRReview(&item)),
		}
	}
}

//nolint:gocritic // listBundle's generic row-renderer contract passes list items by value.
func sessionListToonRow(item SessionRecord) []string {
	return []string{
		item.ID, item.ProfileName, item.Name, item.AgentName, sessionParentID(&item),
		sessionRuntimeProvider(&item), string(item.State), string(item.Badge),
		sessionFailureKind(&item), displaySessionWorkspace(&item),
		sessionHealthState(&item), sessionHealthStatus(&item), formatTime(item.UpdatedAt),
		item.Branch, sessionPRNumber(&item), sessionPRChecks(&item), sessionPRReview(&item),
	}
}

func decorateSessionListBundle(bundle outputBundle, page SessionListPage) outputBundle {
	bundle.jsonl = func(cmd *cobra.Command) error {
		if err := writeProfileResolutionFrame(cmd); err != nil {
			return err
		}
		for index := range page.Sessions {
			if err := writeJSONLine(cmd, &page.Sessions[index]); err != nil {
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

func sessionParentID(item *SessionRecord) string {
	if item.Lineage == nil {
		return ""
	}
	return strings.TrimSpace(item.Lineage.ParentSessionID)
}

func sessionHealthState(item *SessionRecord) string {
	if item.Health == nil {
		return ""
	}
	return string(item.Health.State)
}

func sessionHealthStatus(item *SessionRecord) string {
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

func sessionPRNumber(item *SessionRecord) string {
	if item.PullRequest == nil {
		return ""
	}
	return "#" + strconv.Itoa(item.PullRequest.Number)
}
func sessionPRChecks(item *SessionRecord) string {
	if item.PullRequest == nil {
		return ""
	}
	return string(item.PullRequest.Checks.State)
}
func sessionPRReview(item *SessionRecord) string {
	if item.PullRequest == nil {
		return ""
	}
	return string(item.PullRequest.ReviewDecision)
}

func sessionListHasVisibilityFilters(flags sessionListFlags) bool {
	return flags.subagents != "" && flags.subagents != sessionSubagentsInclude ||
		len(flags.columns) > 0 || flags.hideChildren || flags.includeAll || flags.archived || flags.includeArchived
}
