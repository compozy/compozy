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

type runtimeMCPImportClient interface {
	ImportRuntimeMCPServer(
		context.Context,
		string,
		string,
		contract.RuntimeMCPImportRequest,
	) (contract.RuntimeMCPImportResponse, error)
}

func (c *daemonClient) ImportRuntimeMCPServer(
	ctx context.Context,
	runtime, name string,
	body contract.RuntimeMCPImportRequest,
) (contract.RuntimeMCPImportResponse, error) {
	var result contract.RuntimeMCPImportResponse
	path := "/api/settings/runtime-mcp-servers/" + url.PathEscape(
		runtime,
	) + "/" + strings.ReplaceAll(
		url.PathEscape(name),
		":",
		"%3A",
	) + "/import"
	err := c.doJSON(ctx, http.MethodPost, path, nil, body, &result)
	if apiErr, ok := errors.AsType[*daemonAPIError](err); ok && apiErr.payload.Code != "" {
		hint := ""
		switch apiErr.payload.Code {
		case "mcp_import_name_taken":
			hint = ". Use --name to copy it under another name"
		case "mcp_import_unsupported_definition":
			hint = ". Add it by hand in Settings"
		}
		err = fmt.Errorf("%w%s (%s)", err, hint, apiErr.payload.Code)
		if strings.HasPrefix(apiErr.payload.Code, "mcp_import_") || apiErr.payload.Code == "mcp_server_not_found" {
			err = withCommandExitCode(1, err)
		}
	}
	return result, err
}

const mcpTransportKey = "transport"

func newMCPImportCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, scope, name string
	cmd := &cobra.Command{
		Use:   "import <runtime>/<server>",
		Short: "Copy a runtime MCP server into CompozyOS",
		Args:  exactOneNonBlankArg(),
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		runtime, server, found := strings.Cut(args[0], "/")
		if !found || runtime == "" || server == "" {
			return errors.New("cli: expected <runtime>/<server>")
		}
		client, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		importer, ok := client.(runtimeMCPImportClient)
		if !ok {
			return errors.New("cli: runtime MCP copy unavailable")
		}
		body := contract.RuntimeMCPImportRequest{Scope: contract.SettingsScopeKind(scope), Name: name}
		if workspaceRef != "" || scope == "workspace" {
			resolved, err := resolveCommandWorkspace(
				cmd.Context(),
				cmd,
				deps,
				client,
				workspaceResolutionRequest{FlagRef: workspaceRef},
			)
			if err != nil {
				return err
			}
			body.WorkspaceID = resolved.ID
		}
		if scope == "profile" {
			profiles, _, err := profileResolutionClientFromDeps(deps)
			if err != nil {
				return err
			}
			lens, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
			if err != nil {
				return err
			}
			body.Profile = lens.Profile.Name
		}
		result, err := importer.ImportRuntimeMCPServer(cmd.Context(), runtime, server, body)
		if err != nil {
			return err
		}
		return writeCommandOutput(cmd, outputBundle{jsonValue: result,
			jsonl: func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, result) },
			toon: func() (string, error) {
				return renderToonObject(
					"mcp_server",
					[]string{automationNameKey, automationScopeKey, mcpTransportKey, loopRuntimeKey, cliSourceKey},
					[]string{
						result.MCPServer.Name,
						string(result.MCPServer.Scope),
						result.MCPServer.Transport,
						result.Source.Runtime,
						result.Source.SourceLabel,
					},
				), nil
			}, human: func() (string, error) {
				display := runtime
				for _, descriptor := range runtimemcp.Descriptors() {
					if descriptor.Runtime == runtime {
						display = descriptor.DisplayName
					}
				}
				label := map[string]string{"user": "Personal", "workspace": "Project", "profile": "Profile"}[scope]
				output := []string{fmt.Sprintf(
					"Copied %s from %s (%s) to CompozyOS (%s).\n",
					result.MCPServer.Name,
					display,
					result.Source.SourceLabel,
					label,
				)}
				if result.MCPServer.AuthStatus != nil && result.MCPServer.AuthStatus.Status == "needs_login" {
					output = append(output, fmt.Sprintf(
						"%s uses OAuth: sign in with `compozy mcp auth login %s`.\n",
						result.MCPServer.Name,
						result.MCPServer.Name,
					))
				}
				for _, note := range result.Notes {
					for _, descriptor := range runtimemcp.Descriptors() {
						if descriptor.Runtime == runtime && descriptor.CanDisableRuntimeMCP {
							note += fmt.Sprintf("; turn off \"Use %s's own MCP servers\" to avoid duplicates", display)
						}
					}
					output = append(output, note+".\n")
				}
				for _, warning := range result.Warnings {
					output = append(output, warning+"\n")
				}
				return strings.Join(output, ""), nil
			}})
	}
	cmd.Flags().StringVar(&scope, "scope", "user", "Target scope (user, profile, workspace)")
	cmd.Flags().StringVar(&workspaceRef, "workspace", "", "Workspace ID, name, or path")
	cmd.Flags().StringVar(&name, "name", "", "Name of the copied CompozyOS server")
	return cmd
}
