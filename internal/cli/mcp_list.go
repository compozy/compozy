package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/runtimemcp"
	"github.com/spf13/cobra"
)

type runtimeMCPClient interface {
	ListRuntimeMCPServers(context.Context, string, string, string) (contract.RuntimeMCPServersResponse, error)
}

func (c *daemonClient) ListRuntimeMCPServers(ctx context.Context, workspace, profile, runtime string) (contract.RuntimeMCPServersResponse, error) {
	query := url.Values{}
	if workspace != "" {
		query.Set("workspace_id", workspace)
	}
	if profile != "" {
		query.Set("profile", profile)
	}
	if runtime != "" {
		query.Set("runtime", runtime)
	}
	var result contract.RuntimeMCPServersResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/settings/runtime-mcp-servers", query, nil, &result)
	return result, err
}

func newMCPListCommand(deps commandDeps) *cobra.Command {
	var runtime, workspace string
	var compozyOnly, runtimeOnly bool
	cmd := &cobra.Command{Use: "list", Short: "List CompozyOS and runtime MCP servers", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if compozyOnly && runtimeOnly {
			return errors.New("--compozy-only and --runtime-only cannot be combined")
		}
		client, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		if workspace != "" {
			resolution, err := resolveCommandWorkspace(cmd.Context(), cmd, deps, client, workspaceResolutionRequest{FlagRef: workspace})
			if err != nil {
				return err
			}
			workspace = resolution.ID
		}
		result := contract.MCPListResponse{Compozy: []contract.SettingsMCPServerItemPayload{}, Runtimes: []runtimemcp.Snapshot{}}
		if !compozyOnly {
			reader, ok := client.(runtimeMCPClient)
			if !ok {
				return errors.New("runtime MCP inventory client unavailable")
			}
			inventory, err := reader.ListRuntimeMCPServers(cmd.Context(), workspace, "", runtime)
			if err != nil {
				return err
			}
			result.Runtimes = inventory.Runtimes
		}
		if !runtimeOnly && runtime == "" {
			scope := contract.SettingsLayeredScopeUser
			if workspace != "" {
				scope = contract.SettingsLayeredScopeWorkspace
			}
			collection, err := client.ListSettingsMCPServers(cmd.Context(), scope, workspace, "")
			if err != nil {
				return err
			}
			result.Compozy = collection.MCPServers
		}
		return writeCommandOutput(cmd, mcpListBundle(result))
	}
	cmd.Flags().StringVar(&runtime, "runtime", "", "Filter by runtime ID")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace ID, name, or path")
	cmd.Flags().BoolVar(&compozyOnly, "compozy-only", false, "Show only CompozyOS servers")
	cmd.Flags().BoolVar(&runtimeOnly, "runtime-only", false, "Show only runtime servers")
	return cmd
}

func mcpListBundle(result contract.MCPListResponse) outputBundle {
	return outputBundle{jsonValue: result, json: func(cmd *cobra.Command) error { return writeJSONWithoutWorkspaceResolution(cmd, result) },
		jsonl: func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, result) },
		human: func() (string, error) {
			var out strings.Builder
			if len(result.Compozy) > 0 {
				fmt.Fprintf(&out, "COMPOZYOS · %d servers\n", len(result.Compozy))
				for _, s := range result.Compozy {
					fmt.Fprintf(&out, "  %-24s %-10s %-8s\n", s.Name, s.Scope, s.Transport)
				}
			}
			for _, group := range result.Runtimes {
				fmt.Fprintf(&out, "\n%s · %d servers\n", strings.ToUpper(group.DisplayName), len(group.Servers))
				if group.ReadError != "" {
					fmt.Fprintln(&out, group.ReadError)
				}
				for _, s := range group.Servers {
					state := "—"
					if !s.EnabledInRuntime {
						state = "Off in " + group.DisplayName
					} else if s.AlsoInCompozy {
						state = "Also in CompozyOS"
					} else if s.Approval == "pending" {
						state = "Waiting for approval"
					}
					fmt.Fprintf(&out, "  %-24s %-10s %-8s %-24s %s\n", s.Name, s.Scope, s.Transport, state, s.SourceLabel)
				}
				for _, note := range group.Notes {
					fmt.Fprintf(&out, "  %s\n", note)
				}
			}
			return out.String(), nil
		}, toon: func() (string, error) {
			rows := [][]string{}
			for _, s := range result.Compozy {
				rows = append(rows, []string{"compozy", s.Name, string(s.Scope), s.Transport, "", ""})
			}
			for _, group := range result.Runtimes {
				for _, s := range group.Servers {
					rows = append(rows, []string{group.Runtime, s.Name, s.Scope, s.Transport, s.Approval, s.SourceLabel})
				}
			}
			return renderToonArray("mcp_servers", []string{"runtime", "name", "scope", "transport", "approval", "source"}, rows), nil
		}}
}
