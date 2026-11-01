package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type agentReadModelsClient interface {
	AgentActivity(context.Context, string, int) (agentinstance.Activity, error)
	InstanceAway(context.Context, string) (contract.AgentInstanceAwayResponse, error)
	InstanceSeen(context.Context, string) (contract.AgentInstanceSeenResponse, error)
	InstanceCheckIns(context.Context, string, url.Values) (contract.AgentInstanceCheckInsResponse, error)
}

var _ agentReadModelsClient = (*daemonClient)(nil)

func (c *daemonClient) AgentActivity(ctx context.Context, name string, days int) (agentinstance.Activity, error) {
	var result agentinstance.Activity
	err := c.doJSON(ctx, http.MethodGet, "/api/agents/"+url.PathEscape(name)+"/activity", url.Values{"days": {strconv.Itoa(days)}}, nil, &result)
	return result, err
}
func (c *daemonClient) InstanceAway(ctx context.Context, id string) (contract.AgentInstanceAwayResponse, error) {
	var result contract.AgentInstanceAwayResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/agent-instances/"+url.PathEscape(id)+"/away", nil, nil, &result)
	return result, err
}
func (c *daemonClient) InstanceSeen(ctx context.Context, id string) (contract.AgentInstanceSeenResponse, error) {
	var result contract.AgentInstanceSeenResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/agent-instances/"+url.PathEscape(id)+"/away", nil, nil, &result)
	return result, err
}
func (c *daemonClient) InstanceCheckIns(ctx context.Context, id string, query url.Values) (contract.AgentInstanceCheckInsResponse, error) {
	var result contract.AgentInstanceCheckInsResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/agent-instances/"+url.PathEscape(id)+"/check-ins", query, nil, &result)
	return result, err
}

func newAgentActivityCommand(deps commandDeps) *cobra.Command {
	var days int
	cmd := &cobra.Command{Use: "activity <agent>", Short: "Show agent activity", Args: exactOneNonBlankArg()}
	cmd.Flags().IntVar(&days, "days", 30, "Number of days (1-365)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if days < 1 || days > 365 {
			return errors.New("cli: --days must be from 1 to 365")
		}
		base, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		client, ok := base.(agentReadModelsClient)
		if !ok {
			return errors.New("cli: agent activity unavailable")
		}
		result, err := client.AgentActivity(cmd.Context(), args[0], days)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, fmt.Sprintf("Projects: %d\nReport days: %d\n", len(result.CostByProject), len(result.ReportsByDay)))
	}
	return cmd
}

func writeInstanceRead(cmd *cobra.Command, base any, action, id string, query url.Values) error {
	client, ok := base.(agentReadModelsClient)
	if !ok {
		return errors.New("cli: instance reads unavailable")
	}
	switch action {
	case "away":
		result, err := client.InstanceAway(cmd.Context(), id)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, fmt.Sprintf("Quiet check-ins: %d\nReports: %d\n", result.QuietCheckIns, len(result.Reports)))
	case "mark-seen":
		result, err := client.InstanceSeen(cmd.Context(), id)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, "Instance marked as seen.\n")
	default:
		result, err := client.InstanceCheckIns(cmd.Context(), id, query)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, fmt.Sprintf("Check-ins: %d\nNext cursor: %s\n", len(result.CheckIns), result.NextCursor))
	}
}
