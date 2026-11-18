package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func (c *daemonClient) CheckSessionMCP(
	ctx context.Context,
	id, name, owner string,
) (contract.SessionMCPResponse, error) {
	path, err := c.sessionScopedPath(ctx, id, "/mcp/servers/"+url.PathEscape(strings.TrimSpace(name))+"/check")
	if err != nil {
		return contract.SessionMCPResponse{}, err
	}
	query := url.Values{}
	if owner = strings.TrimSpace(owner); owner != "" {
		query.Set("owner", owner)
	}
	var result contract.SessionMCPResponse
	err = c.doJSON(ctx, http.MethodPost, path, query, nil, &result)
	return result, err
}

func newSessionMCPCheckCommand(deps commandDeps) *cobra.Command {
	var owner string
	cmd := &cobra.Command{
		Use:   "check <id> <name>",
		Short: "Check one session MCP server again",
		Example: "  compozy session mcp check sess_1234 linear\n" +
			"  compozy session mcp check sess_1234 linear --owner extension:linear -o json",
		Args: exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			checker, ok := client.(interface {
				CheckSessionMCP(context.Context, string, string, string) (contract.SessionMCPResponse, error)
			})
			if !ok {
				return errors.New("session MCP check client unavailable")
			}
			result, err := checker.CheckSessionMCP(cmd.Context(), args[0], args[1], owner)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionMCPBundle(result))
		},
	}
	cmd.Flags().StringVar(&owner, "owner", "", "Definition owner: manual or extension:<name>")
	return cmd
}
