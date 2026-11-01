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

func (c *daemonClient) AgentInstanceOccurrence(
	ctx context.Context,
	id, occurrence string,
) (contract.AgentInstanceOccurrenceResponse, error) {
	var result contract.AgentInstanceOccurrenceResponse
	err := c.doJSON(
		ctx,
		"GET",
		"/api/agent-instances/"+url.PathEscape(id)+"/occurrences/"+url.PathEscape(occurrence),
		nil,
		nil,
		&result,
	)
	return result, err
}

func waitInstanceOccurrence(
	cmd *cobra.Command,
	client agentInstanceOccurrenceClient,
	id, occurrence string,
	interval time.Duration,
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		result, err := client.AgentInstanceOccurrence(cmd.Context(), id, occurrence)
		if err != nil {
			return err
		}
		if result.State == "finished" {
			return writeCommandOutput(cmd, outputBundle{jsonValue: result,
				human: func() (string, error) { return instanceOccurrenceText(cmd.Context(), client, id, result) },
				json:  func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, result) },
				jsonl: func(cmd *cobra.Command) error { return writeJSONLineWithoutWorkspaceResolution(cmd, result) },
			})
		}
		select {
		case <-cmd.Context().Done():
			return cmd.Context().Err()
		case <-ticker.C:
		}
	}
}

func newAgentInstanceOccurrenceCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "occurrence <occurrence-id>",
		Short: "Read an instance check occurrence",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			query := url.Values{"all_workspaces": {"true"}, listLimitField: {"100"}}
			for {
				page, err := instances.ListAgentInstances(cmd.Context(), query)
				if err != nil {
					return err
				}
				for _, instance := range page.Instances {
					result, err := reader.AgentInstanceOccurrence(cmd.Context(), instance.ID, args[0])
					if err == nil {
						return writeInstanceOutput(
							cmd,
							result,
							fmt.Sprintf("%s\t%s\t%s\n", result.State, result.Result, result.Reason),
						)
					}
					apiErr, ok := errors.AsType[*daemonAPIError](err)
					if !ok || apiErr.statusCode != 404 {
						return err
					}
				}
				if page.NextCursor == "" {
					return errors.New("cli: occurrence not found")
				}
				query.Set("cursor", page.NextCursor)
			}
		},
	}
}

func instanceOccurrenceText(
	ctx context.Context,
	client agentInstanceOccurrenceClient,
	id string,
	result contract.AgentInstanceOccurrenceResponse,
) (string, error) {
	if result.Result != "reported" || result.ReportID == "" {
		return fmt.Sprintf("Check finished: %s (%s)\n", result.Result, result.Reason), nil
	}
	reports, ok := client.(agentInstancesClient)
	if !ok {
		return "", errors.New("cli: instance reports unavailable")
	}
	query := url.Values{listLimitField: {"100"}}
	for {
		page, err := reports.AgentInstanceReports(ctx, id, query)
		if err != nil {
			return "", err
		}
		for _, report := range page.Reports {
			if report.ID != result.ReportID {
				continue
			}
			noun := "proposals"
			if len(report.Proposals) == 1 {
				noun = "proposal"
			}
			return fmt.Sprintf("Check finished: %d %s %s\n", len(report.Proposals), noun, report.State), nil
		}
		if page.NextCursor == "" {
			return "", errors.New("cli: completed check report not found")
		}
		query.Set("cursor", page.NextCursor)
	}
}
