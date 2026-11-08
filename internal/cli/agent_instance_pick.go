package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type agentInstancePickClient interface {
	PickAgentInstanceProposal(context.Context, string, string, int) (contract.AgentInstancePickResponse, error)
}

func (c *daemonClient) PickAgentInstanceProposal(
	ctx context.Context,
	id, report string,
	index int,
) (contract.AgentInstancePickResponse, error) {
	var result contract.AgentInstancePickResponse
	err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/agent-instances/"+url.PathEscape(id)+"/reports/"+url.PathEscape(report)+"/pick",
		nil,
		contract.AgentInstancePickRequest{Proposal: &index},
		&result,
	)
	return result, err
}
func newAgentInstancePickCommand(deps commandDeps) *cobra.Command {
	var options agentInstanceOptions
	cmd := &cobra.Command{
		Use:   "pick <agent-or-instance> <report-id> <n>",
		Short: "Pick a proposal (1-based index)",
		Args:  cobra.ExactArgs(3),
	}
	cmd.Flags().String(workspaceSkillSource, "", "Workspace ID, name, or path")
	cmd.Flags().BoolVar(&options.global, "global", false, "Use the global scope")
	cmd.MarkFlagsMutuallyExclusive("global", workspaceSkillSource)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		index, err := strconv.Atoi(args[2])
		if err != nil || index < 1 {
			return errors.New("proposal_out_of_range: use a positive 1-based index")
		}
		base, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		client, ok := base.(agentInstancesClient)
		if !ok {
			return errors.New("agent instances unavailable")
		}
		picker, ok := base.(agentInstancePickClient)
		if !ok {
			return errors.New("proposal picks unavailable")
		}
		query, err := options.query(cmd, deps, base, "show")
		if err != nil {
			return err
		}
		id, err := resolveInstanceID(cmd, client, args[0], query.Get(workspaceSkillSource))
		if err != nil {
			return err
		}
		result, err := picker.PickAgentInstanceProposal(cmd.Context(), id, args[1], index-1)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, fmt.Sprintf("Picked proposal %d (%s)\n", index, result.Pick.State))
	}
	return cmd
}
