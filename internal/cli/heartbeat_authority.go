package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/spf13/cobra"
)

type heartbeatAuthorityClient interface {
	HeartbeatAuthority(context.Context, string, AgentQuery) (contract.HeartbeatAuthorityPayload, error)
	MutateHeartbeatAuthority(context.Context, string, AgentQuery, string, bool) error
}

func (c *daemonClient) HeartbeatAuthority(
	ctx context.Context,
	name string,
	q AgentQuery,
) (contract.HeartbeatAuthorityPayload, error) {
	var result contract.HeartbeatAuthorityPayload
	err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/agents/"+url.PathEscape(name)+"/heartbeat/authority",
		agentValues(q),
		nil,
		&result,
	)
	return result, err
}

func (c *daemonClient) MutateHeartbeatAuthority(
	ctx context.Context,
	name string,
	q AgentQuery,
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
		"/api/agents/"+url.PathEscape(name)+"/heartbeat/authority/approve",
		agentValues(q),
		contract.HeartbeatScriptApprovalRequest{SHA256: hash},
		nil,
	)
}

func confirmHeartbeatAuthority(
	cmd *cobra.Command,
	authority contract.HeartbeatAuthorityPayload,
	yes bool,
) (bool, error) {
	block, err := json.MarshalIndent(authority, "", "  ")
	if err != nil {
		return false, err
	}
	fmt.Fprintln(cmd.ErrOrStderr(), string(block))
	if yes {
		return true, nil
	}
	fmt.Fprint(cmd.ErrOrStderr(), "Approve these check-in permissions? [y/N] ")
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(answer), "y"), nil
}
func newAgentHeartbeatAuthorityCommand(deps commandDeps) *cobra.Command {
	parent := &cobra.Command{Use: "authority", Short: "Review and approve check-in authority"}
	for _, action := range []string{"show", "approve", "revoke"} {
		var yes bool
		var hash string
		cmd := &cobra.Command{
			Use:   action + " <agent>",
			Short: action + " check-in authority",
			Args:  exactOneNonBlankArg(),
			RunE: func(cmd *cobra.Command, args []string) error {
				if action != "show" && agentIdentityEnvironmentPresent(deps) {
					return source.Failure(
						"heartbeat_authority_operator_only",
						"only the operator can approve or revoke check-in authority",
					)
				}
				client, err := clientFromDeps(deps)
				if err != nil {
					return err
				}
				authorityClient, ok := client.(heartbeatAuthorityClient)
				if !ok {
					return errors.New("check-in authority client unavailable")
				}
				query, err := agentQueryFromCommand(cmd, deps, client)
				if err != nil {
					return err
				}
				authority, err := authorityClient.HeartbeatAuthority(cmd.Context(), args[0], query)
				if err != nil {
					return err
				}
				if action == "show" {
					return writeCommandOutput(cmd, outputBundle{jsonValue: authority, human: func() (string, error) {
						data, err := json.MarshalIndent(authority, "", "  ")
						return string(data) + "\n", err
					}})
				}
				if hash != "" && hash != authority.Digest {
					return source.Failure(
						"heartbeat_authority_changed",
						"check-in authority changed; review the current block",
					)
				}
				if action == "approve" && yes && hash == "" {
					return errors.New("--yes requires --sha256 for authority approval")
				}
				confirmed, err := confirmHeartbeatAuthority(cmd, authority, yes)
				if err != nil {
					return err
				}
				if !confirmed {
					return nil
				}
				return authorityClient.MutateHeartbeatAuthority(
					cmd.Context(),
					args[0],
					query,
					authority.Digest,
					action == "revoke",
				)
			},
		}
		cmd.Flags().BoolVar(&yes, "yes", false, "Confirm the operation")
		cmd.Flags().StringVar(&hash, "sha256", "", "Expected authority digest")
		addWorkspaceFlag(cmd)
		parent.AddCommand(cmd)
	}
	return parent
}

func reviewHeartbeatWriteAuthority(
	cmd *cobra.Command,
	deps commandDeps,
	body string,
	approve, yes bool,
) (string, error) {
	// The server validates configured caps; local parsing only prepares the review block.
	parsed, err := heartbeat.Parse(
		cmd.Context(),
		heartbeat.ParseRequest{
			Content:       []byte(body),
			Config:        compozyconfig.DefaultHeartbeatConfig(),
			CheckInLimits: &heartbeat.CheckInLimits{MaxToolCalls: 100, MaxWriteCalls: 50},
		},
	)
	if err != nil {
		return "", nil
	}
	hash, err := heartbeat.AuthorityDigest(
		parsed.Frontmatter.CheckIns,
		heartbeat.AuthorityActions(parsed.Frontmatter.Actions),
	)
	if err != nil || hash == "" || !approve && yes {
		return "", err
	}
	if agentIdentityEnvironmentPresent(deps) {
		if approve {
			return "", source.Failure(
				"heartbeat_authority_operator_only",
				"only the operator can approve check-in authority",
			)
		}
		return "", nil
	}
	confirmed, err := confirmHeartbeatAuthority(
		cmd,
		contract.HeartbeatAuthorityPayload{
			Digest:   hash,
			Status:   "unapproved",
			CheckIns: parsed.Frontmatter.CheckIns,
			Actions:  parsed.Frontmatter.Actions,
		},
		yes && approve,
	)
	if err != nil || !confirmed {
		return "", err
	}
	return hash, nil
}
