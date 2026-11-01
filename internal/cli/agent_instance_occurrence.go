package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type agentInstanceOccurrenceClient interface {
	AgentInstanceOccurrence(context.Context, string, string) (contract.AgentInstanceOccurrenceResponse, error)
}

var _ agentInstanceOccurrenceClient = (*daemonClient)(nil)

func (c *daemonClient) AgentInstanceOccurrence(ctx context.Context, id, occurrence string) (contract.AgentInstanceOccurrenceResponse, error) {
	var result contract.AgentInstanceOccurrenceResponse
	err := c.doJSON(ctx, "GET", "/api/agent-instances/"+url.PathEscape(id)+"/occurrences/"+url.PathEscape(occurrence), nil, nil, &result)
	return result, err
}

func waitInstanceOccurrence(cmd *cobra.Command, client agentInstanceOccurrenceClient, id, occurrence string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		result, err := client.AgentInstanceOccurrence(cmd.Context(), id, occurrence)
		if err != nil {
			return err
		}
		if result.State == "finished" {
			return writeInstanceOutput(cmd, result, fmt.Sprintf("Check finished: %s (%s)\n", result.Result, result.Reason))
		}
		select {
		case <-cmd.Context().Done():
			return cmd.Context().Err()
		case <-ticker.C:
		}
	}
}

func newAgentInstanceOccurrenceCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{Use: "occurrence <occurrence-id>", Short: "Read an instance check occurrence", Args: exactOneNonBlankArg(), RunE: func(cmd *cobra.Command, args []string) error {
		base, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		instances, ok := base.(agentInstancesClient)
		if !ok {
			return errors.New("cli: agent instances unavailable")
		}
		reader, ok := base.(agentInstanceOccurrenceClient)
		if !ok {
			return errors.New("cli: instance occurrences unavailable")
		}
		query := url.Values{"all_workspaces": {"true"}, "limit": {"100"}}
		for {
			page, err := instances.ListAgentInstances(cmd.Context(), query)
			if err != nil {
				return err
			}
			for _, instance := range page.Instances {
				result, err := reader.AgentInstanceOccurrence(cmd.Context(), instance.ID, args[0])
				if err == nil {
					return writeInstanceOutput(cmd, result, fmt.Sprintf("%s\t%s\t%s\n", result.State, result.Result, result.Reason))
				}
				var apiErr *daemonAPIError
				if !errors.As(err, &apiErr) || apiErr.statusCode != 404 {
					return err
				}
			}
			if page.NextCursor == "" {
				return errors.New("cli: occurrence not found")
			}
			query.Set("cursor", page.NextCursor)
		}
	}}
}
