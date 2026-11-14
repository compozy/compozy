package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/runtimemcp"
	"github.com/spf13/cobra"
)

func (c *daemonClient) GetSessionMCP(ctx context.Context, id string) (contract.SessionMCPResponse, error) {
	path, err := c.sessionScopedPath(ctx, id, "/mcp")
	if err != nil {
		return contract.SessionMCPResponse{}, err
	}
	var result contract.SessionMCPResponse
	err = c.doJSON(ctx, http.MethodGet, path, nil, nil, &result)
	return result, err
}

type sessionMCPClient interface {
	GetSessionMCP(context.Context, string) (contract.SessionMCPResponse, error)
}

func newSessionMCPCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "mcp <id>", Short: "Show MCP servers available to a session",
		Example: "  compozy session mcp sess_1234\n  compozy session mcp sess_1234 -o json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			reader, ok := client.(sessionMCPClient)
			if !ok {
				return errors.New("session MCP client unavailable")
			}
			result, err := reader.GetSessionMCP(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionMCPBundle(result))
		},
	}
}

func sessionMCPBundle(result contract.SessionMCPResponse) outputBundle {
	return outputBundle{jsonValue: result,
		json:  func(cmd *cobra.Command) error { return writeJSONWithoutWorkspaceResolution(cmd, result) },
		jsonl: func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, result) },
		toon: func() (string, error) {
			rows := make([][]string, 0, len(result.CompozyServers)+len(result.RuntimeServers.Servers))
			for _, server := range result.CompozyServers {
				rows = append(rows, []string{"compozy", server.Name, server.State, server.Reason})
			}
			for _, server := range result.RuntimeServers.Servers {
				rows = append(rows, []string{result.Runtime, server.Name, server.State, ""})
			}
			return renderToonArray(
				"mcp_servers",
				[]string{loopRuntimeKey, automationNameKey, appTokenStateFlag, loopReasonKey},
				rows,
			), nil
		},
		human: func() (string, error) {
			var out strings.Builder
			runtimeName := result.Runtime
			for _, descriptor := range runtimemcp.Descriptors() {
				if descriptor.Runtime == result.Runtime {
					runtimeName = descriptor.DisplayName
					break
				}
			}
			fmt.Fprintf(
				&out,
				"SESSION %s · %s · %d usable\nCOMPOZYOS SERVERS\n",
				result.SessionID,
				result.Runtime,
				result.Usable,
			)
			for _, server := range result.CompozyServers {
				fmt.Fprintf(
					&out,
					"  %-24s %-28s %d tools\n",
					server.Name,
					sessionMCPStateLabel(server.State),
					server.CallableCount,
				)
				if server.Reason != "" {
					fmt.Fprintf(&out, "    %s\n", server.Reason)
				}
			}
			fmt.Fprintf(&out, "RUNTIME SERVERS (%s)\n", runtimeName)
			for _, server := range result.RuntimeServers.Servers {
				label := sessionMCPStateLabel(server.State)
				if server.State == "loaded_by_runtime" {
					label = "Loaded by " + runtimeName
				}
				fmt.Fprintf(&out, "  %-24s %s\n", server.Name, label)
			}
			return out.String(), nil
		},
	}
}

func sessionMCPStateLabel(state string) string {
	switch state {
	case "ready":
		return "Ready"
	case "applies_next_message":
		return "Applies on your next message"
	case "needs_sign_in":
		return "Needs sign-in"
	case "waiting_approval":
		return "Waiting for approval"
	case string(bootstrapPhaseFailed):
		return "Couldn't connect"
	case "not_available":
		return "Not available in this session"
	case "loaded_by_runtime":
		return "Loaded by runtime"
	case "off_in_compozy_sessions":
		return "Off in CompozyOS sessions"
	default:
		return state
	}
}
