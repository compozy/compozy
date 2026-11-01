package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type aggregateCatalogClient interface {
	ReadAggregateCatalog(context.Context, string, url.Values, any) error
}

func (c *daemonClient) ReadAggregateCatalog(ctx context.Context, path string, values url.Values, result any) error {
	values.Set("all_workspaces", toolBoolTrue)
	return c.doJSON(ctx, http.MethodGet, path, values, nil, result)
}

func writeAggregateCatalog(cmd *cobra.Command, deps commandDeps, kind string) error {
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	reader, ok := client.(aggregateCatalogClient)
	if !ok {
		return fmt.Errorf("cli: aggregate catalog client unavailable")
	}
	labels, err := aggregateProjectLabels(cmd, client)
	if err != nil {
		return err
	}
	label := func(id string) string { return aggregateProjectName(labels, id) }
	values := url.Values{}
	var bundle outputBundle
	rows := [][]string{}
	headers := []string{cliProjectHeader, "NAME", cliStateHeader}
	switch kind {
	case "terminals":
		var response contract.TerminalListResponse
		if err := reader.ReadAggregateCatalog(cmd.Context(), "/api/terminals", values, &response); err != nil {
			return err
		}
		bundle = terminalListBundle(cmd, response.Terminals, deps.now)
		for _, item := range response.Terminals {
			rows = append(rows, []string{label(item.WorkspaceID), item.Title, string(item.State)})
		}
	case "worktrees":
		var response contract.WorktreesResponse
		if err := reader.ReadAggregateCatalog(cmd.Context(), "/api/worktrees", values, &response); err != nil {
			return err
		}
		bundle = worktreeListBundle(response.Worktrees)
		for _, item := range response.Worktrees {
			rows = append(rows, []string{label(item.WorkspaceID), item.Name, item.State})
		}
	case "agents":
		items, agentRows, err := readAggregateAgents(cmd, reader, label)
		if err != nil {
			return err
		}
		rows = agentRows
		headers[2] = "PROVIDER"
		bundle = agentListBundle(items)
	case "suggestions":
		status, err := cmd.Flags().GetString("status")
		if err != nil {
			return err
		}
		values.Set("status", status)
		var response contract.AutomationSuggestionsResponse
		if err := reader.ReadAggregateCatalog(
			cmd.Context(),
			"/api/automation/suggestions",
			values,
			&response,
		); err != nil {
			return err
		}
		bundle = automationSuggestionListBundle(response)
		for _, item := range response.Suggestions {
			rows = append(rows, []string{label(item.WorkspaceID), item.Payload.Name, string(item.Status)})
		}
	default:
		return fmt.Errorf("cli: unsupported aggregate %q", kind)
	}
	bundle.human = func() (string, error) { return renderHumanTable(kind, headers, rows), nil }
	return writeCommandOutput(cmd, bundle)
}

// readAggregateAgents pages through every aggregate agent row.
func readAggregateAgents(
	cmd *cobra.Command,
	reader aggregateCatalogClient,
	label func(string) string,
) ([]AgentRecord, [][]string, error) {
	values := url.Values{}
	var items []AgentRecord
	rows := [][]string{}
	for {
		var response contract.AgentCatalogResponse
		if err := reader.ReadAggregateCatalog(cmd.Context(), "/api/agents/aggregate", values, &response); err != nil {
			return nil, nil, err
		}
		for index := range response.Agents {
			item := &response.Agents[index]
			items = append(items, item.Agent)
			rows = append(rows, []string{label(item.Agent.WorkspaceID), item.Agent.Name, item.Agent.Provider})
		}
		if !response.Page.HasMore {
			return items, rows, nil
		}
		values.Set("cursor", response.Page.NextCursor)
	}
}

func addAllWorkspacesFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("all-workspaces", false, "List every project and User scope")
	cmd.MarkFlagsMutuallyExclusive("workspace", "all-workspaces")
}

func wantsAllWorkspaces(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup("all-workspaces")
	return flag != nil && flag.Value.String() == toolBoolTrue
}

type workspaceLister interface {
	ListWorkspaces(context.Context) ([]WorkspaceRecord, error)
}

// writeProjectLabeledOutput writes a listing, labeling rows by project only under All projects.
func writeProjectLabeledOutput(
	cmd *cobra.Command,
	client workspaceLister,
	allWorkspaces bool,
	bundle func(projectLabels ...map[string]string) outputBundle,
) error {
	if !allWorkspaces {
		return writeCommandOutput(cmd, bundle())
	}
	labels, err := aggregateProjectLabels(cmd, client)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, bundle(labels))
}

func aggregateProjectLabels(cmd *cobra.Command, client workspaceLister) (map[string]string, error) {
	labels := map[string]string{"": cliUserScopeLabel}
	items, err := client.ListWorkspaces(cmd.Context())
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		labels[item.ID] = item.Name
	}
	return labels, nil
}
func aggregateProjectName(labels map[string]string, id string) string {
	if name, ok := labels[id]; ok {
		return name
	}
	return id
}
