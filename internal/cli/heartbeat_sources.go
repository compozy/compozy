package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/spf13/cobra"
)

type heartbeatSourcesClient interface {
	HeartbeatSources(context.Context, string, AgentQuery) (contract.HeartbeatSourceCatalogPayload, error)
	HeartbeatScriptApproval(context.Context, string, string, AgentQuery, string, bool) error
}

var _ heartbeatSourcesClient = (*daemonClient)(nil)

func (c *daemonClient) HeartbeatSources(
	ctx context.Context,
	name string,
	query AgentQuery,
) (contract.HeartbeatSourceCatalogPayload, error) {
	var result contract.HeartbeatSourceCatalogPayload
	err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/agents/"+url.PathEscape(name)+"/heartbeat/sources",
		agentValues(query),
		nil,
		&result,
	)
	return result, err
}

func (c *daemonClient) HeartbeatScriptApproval(
	ctx context.Context,
	name, script string,
	query AgentQuery,
	hash string,
	revoke bool,
) error {
	method := http.MethodPost
	if revoke {
		method = http.MethodDelete
	}
	return c.doJSON(
		ctx,
		method,
		"/api/agents/"+url.PathEscape(name)+"/heartbeat/scripts/"+url.PathEscape(script)+"/approve",
		agentValues(query),
		contract.HeartbeatScriptApprovalRequest{SHA256: hash},
		nil,
	)
}
func newAgentHeartbeatSourcesCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources <agent>",
		Short: "List available check-in sources",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			sources, ok := client.(heartbeatSourcesClient)
			if !ok {
				return errors.New("check-in source client unavailable")
			}
			query, err := agentQueryFromCommand(cmd, deps, client)
			if err != nil {
				return err
			}
			catalog, err := sources.HeartbeatSources(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, outputBundle{jsonValue: catalog, human: func() (string, error) {
				var b strings.Builder
				if len(catalog.Kinds) > 0 {
					fmt.Fprintln(&b, "KIND\tTITLE\tFROM")
					for _, kind := range catalog.Kinds {
						fmt.Fprintf(&b, "%s\t%s\textension %s\n", kind.Kind, kind.Title, kind.Extension)
					}
				}
				fmt.Fprintln(&b, "SCRIPT\tEXECUTABLE\tAPPROVED")
				if len(catalog.Scripts) == 0 {
					fmt.Fprintln(&b, "(none)")
				}
				for _, script := range catalog.Scripts {
					fmt.Fprintf(&b, "%s\t%t\t%t\n", script.Name, script.Executable, script.Approved)
				}
				state := "enabled"
				if !catalog.ScriptsEnabled {
					state = "disabled"
				}
				fmt.Fprintf(
					&b,
					"Scripts: %s · Limits: %d sources, %d items per check\n",
					state,
					catalog.Limits.MaxPerAgent,
					catalog.Limits.MaxItems,
				)
				return b.String(), nil
			}})
		},
	}
	addWorkspaceFlag(cmd)
	for _, revoke := range []bool{false, true} {
		cmd.AddCommand(newHeartbeatScriptApprovalCommand(deps, revoke))
	}
	return cmd
}
func newHeartbeatScriptApprovalCommand(deps commandDeps, revoke bool) *cobra.Command {
	verb := loopApproveKey
	if revoke {
		verb = "revoke"
	}
	var yes bool
	var hash string
	cmd := &cobra.Command{
		Use:   verb + " <agent> <script>",
		Short: verb + " a check-in script (operator only)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentIdentityEnvironmentPresent(deps) {
				return source.Failure(
					"source_approval_operator_only",
					"only the operator can approve or revoke source scripts",
				)
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			sources, ok := client.(heartbeatSourcesClient)
			if !ok {
				return errors.New("check-in source client unavailable")
			}
			query, err := agentQueryFromCommand(cmd, deps, client)
			if err != nil {
				return err
			}
			if !revoke {
				hash, err = reviewHeartbeatScript(cmd, sources, args[0], args[1], query, hash)
				if err != nil {
					return err
				}
			}
			if !yes {
				if !supportBundleInputIsTerminal(cmd.InOrStdin()) {
					return errors.New("script approval requires --yes when stdin is not interactive")
				}
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%s this exact script? [y/N] ", verb); err != nil {
					return err
				}
				line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if err != nil {
					return err
				}
				if strings.ToLower(strings.TrimSpace(line)) != "y" {
					return nil
				}
			}
			if err := sources.HeartbeatScriptApproval(
				cmd.Context(),
				args[0],
				args[1],
				query,
				hash,
				revoke,
			); err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				outputBundle{
					jsonValue: map[string]string{"script": args[1], "action": verb},
					human:     func() (string, error) { return verb + "d " + args[1] + "\n", nil },
				},
			)
		},
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm the operation")
	if !revoke {
		cmd.Flags().StringVar(&hash, "sha256", "", "Expected script hash")
	}
	return cmd
}

func reviewHeartbeatScript(
	cmd *cobra.Command,
	sources heartbeatSourcesClient,
	agent, name string,
	query AgentQuery,
	hash string,
) (string, error) {
	catalog, err := sources.HeartbeatSources(cmd.Context(), agent, query)
	if err != nil {
		return "", err
	}
	found := false
	for _, script := range catalog.Scripts {
		if script.Name == name {
			found = true
			if hash == "" {
				hash = script.SHA256
			}
			if _, err := fmt.Fprintf(
				cmd.ErrOrStderr(),
				"Script %s (%s)\n  %s\n",
				script.Name,
				agent,
				script.SHA256,
			); err != nil {
				return "", err
			}
		}
	}
	if !found {
		return "", errors.New("source_script_missing: script is not in the catalog")
	}
	return hash, nil
}
