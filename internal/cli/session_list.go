package cli

import (
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/spf13/cobra"
)

func newSessionListCommand(deps commandDeps) *cobra.Command {
	var (
		includeAll      bool
		workspaceFilter string
		stateFilter     string
		typeFilter      string
		agentFilter     string
		search          string
		resumable       bool
		includeHealth   bool
		limit           int
		sortKey         string
		cursor          string
	)

	cmd := &cobra.Command{
		Use:   sessionListKey,
		Short: "List sessions",
		Example: `  # List active sessions
  agh session list

  # Page all sessions for one agent in a workspace
  agh session list --all --workspace checkout-api --agent reviewer --limit 25`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			state := strings.TrimSpace(stateFilter)
			if state == "" && !includeAll && !resumable {
				state = string(session.StateActive)
			}
			sort := strings.TrimSpace(sortKey)
			if sort == "" && resumable {
				sort = session.ListSortLastActivity
			}
			page, err := client.ListSessions(cmd.Context(), SessionListQuery{
				Workspace:     workspaceFilter,
				State:         state,
				Type:          typeFilter,
				Agent:         agentFilter,
				Query:         search,
				Resumable:     resumable,
				IncludeHealth: includeHealth,
				Limit:         limit,
				Sort:          sort,
				Cursor:        cursor,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionListBundle(page, deps.now))
		},
	}
	cmd.Flags().BoolVar(&includeAll, "all", false, "Include every session state when --state is omitted")
	cmd.Flags().StringVar(&workspaceFilter, workspaceSkillSource, "", "Filter by workspace name or ID")
	cmd.Flags().StringVar(&stateFilter, "state", "", "Filter by state (starting|active|stopping|stopped)")
	cmd.Flags().StringVar(
		&typeFilter,
		"type",
		"",
		"Filter by session type (user|system|coordinator|spawned)",
	)
	cmd.Flags().StringVar(&agentFilter, "agent", "", "Filter by exact agent definition name")
	cmd.Flags().StringVar(&search, "query", "", "Search session id, name, agent, provider, or channel")
	cmd.Flags().BoolVar(&resumable, "resumable", false, "Show only sessions eligible for resume attach")
	cmd.Flags().BoolVar(&includeHealth, "include-health", false, "Include metadata-only health for returned sessions")
	cmd.Flags().IntVar(&limit, "limit", 0, "Sessions per page (1-100)")
	cmd.Flags().StringVar(&sortKey, "sort", "", "Sort by recent or last_activity")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue from an opaque next_cursor")
	return cmd
}

func sessionListBundle(page SessionListPage, now func() time.Time) outputBundle {
	items := page.Sessions
	bundle := listBundle(
		page,
		items,
		"Sessions",
		[]string{
			"ID", sessionNameValue, sessionAgentValue, sessionProviderValue, sessionBackendValue,
			sessionStateValue, sessionBadgeValue, "Failure", sessionWorkspaceValue, sessionChannelValue,
			"Health State", "Health", sessionUpdatedValue,
		},
		"sessions",
		[]string{
			"id", sessionNameKey, sessionAgentNameKey, sessionProviderKey, "sandbox_backend",
			sessionStateKey, sessionBadgeKey, taskFailureKindKey, workspaceSkillSource, sessionChannelKey,
			"health_state", extensionHealthKey, sessionUpdatedAtKey,
		},
		func(item SessionRecord) []string {
			return []string{
				stringOrDash(item.ID), stringOrDash(item.Name), stringOrDash(item.AgentName),
				stringOrDash(item.Provider), stringOrDash(sessionSandboxBackend(item)),
				stringOrDash(string(item.State)), stringOrDash(string(item.Badge)),
				stringOrDash(sessionFailureKind(item)), stringOrDash(displaySessionWorkspace(item)),
				stringOrDash(sessionResolvedChannelRaw(item)), stringOrDash(sessionHealthState(item)),
				stringOrDash(sessionHealthStatus(item)), stringOrDash(formatAge(now, item.UpdatedAt)),
			}
		},
		func(item SessionRecord) []string {
			return []string{
				item.ID, item.Name, item.AgentName, item.Provider, sessionSandboxBackend(item),
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
