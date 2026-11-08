package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type agentSkillReviewsClient interface {
	ListAgentSkillReviews(context.Context, url.Values) (contract.AgentSkillReviewsResponse, error)
	GetAgentSkillReview(context.Context, string) (contract.AgentSkillReviewResponse, error)
	DecideAgentSkillReview(context.Context, string, string, string) (contract.AgentSkillReviewResponse, error)
}

func (c *daemonClient) ListAgentSkillReviews(
	ctx context.Context,
	query url.Values,
) (contract.AgentSkillReviewsResponse, error) {
	var result contract.AgentSkillReviewsResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/agent-skill-reviews", query, nil, &result)
	return result, err
}
func (c *daemonClient) GetAgentSkillReview(ctx context.Context, id string) (contract.AgentSkillReviewResponse, error) {
	var result contract.AgentSkillReviewResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/agent-skill-reviews/"+url.PathEscape(id), nil, nil, &result)
	return result, err
}

func (c *daemonClient) DecideAgentSkillReview(
	ctx context.Context,
	id, action, digest string,
) (contract.AgentSkillReviewResponse, error) {
	var result contract.AgentSkillReviewResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/agent-skill-reviews/"+url.PathEscape(id)+"/"+action, nil,
		contract.AgentSkillReviewDecisionRequest{Digest: digest}, &result)
	return result, err
}

func newAgentSkillCommand(deps commandDeps) *cobra.Command {
	root := &cobra.Command{Use: "skill", Short: "Manage proposed agent skills (experimental)"}
	review := &cobra.Command{Use: "review", Short: "Review proposed skill contents before activation"}
	root.AddCommand(review)
	for _, action := range []string{agentListKey, configShowKey, loopApproveKey, "reject"} {
		review.AddCommand(newAgentSkillReviewAction(deps, action))
	}
	return root
}

func newAgentSkillReviewAction(deps commandDeps, action string) *cobra.Command {
	var instance, state, cursor, digest string
	var limit int
	cmd := &cobra.Command{
		Use:   action + " <review-id>",
		Short: "Manage skill reviews: " + action,
		Args:  exactOneNonBlankArg(),
	}
	if action == agentListKey {
		cmd.Use, cmd.Args = agentListKey, cobra.NoArgs
		addWorkspaceFlag(cmd)
		cmd.Flags().StringVar(&instance, "instance", "", "Filter by instance ID")
		cmd.Flags().StringVar(&state, appTokenStateFlag, "", "pending, approved or rejected")
		cmd.Flags().StringVar(&cursor, "cursor", "", "Continue from next_cursor")
		cmd.Flags().IntVar(&limit, "limit", 50, "Page size (1-100)")
	}
	if action == loopApproveKey || action == "reject" {
		cmd.Flags().StringVar(&digest, "expected-digest", "", "Require the digest shown during review")
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		base, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		client, ok := base.(agentSkillReviewsClient)
		if !ok {
			return errors.New("cli: skill reviews unavailable")
		}
		if action == agentListKey {
			scope, err := agentQueryFromCommand(cmd, deps, base)
			if err != nil {
				return err
			}
			query := url.Values{
				"workspace":       {scope.Workspace},
				"instance_id":     {instance},
				appTokenStateFlag: {state},
				"cursor":          {cursor},
				"limit":           {strconv.Itoa(limit)},
			}
			result, err := client.ListAgentSkillReviews(cmd.Context(), query)
			if err != nil {
				return err
			}
			var output strings.Builder
			for _, row := range result.Reviews {
				fmt.Fprintf(&output, "%s\t%s\t%s\t%s\n", row.ID, row.Agent, row.Name, row.State)
			}
			if result.NextCursor != "" {
				fmt.Fprintf(&output, "Next cursor: %s\n", result.NextCursor)
			}
			return writeInstanceOutput(cmd, result, output.String())
		}
		result, err := client.GetAgentSkillReview(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		if action == configShowKey {
			return writeInstanceOutput(cmd, result, agentSkillReviewText(result.Review))
		}
		if digest == "" {
			digest = result.Review.Digest
		}
		result, err = client.DecideAgentSkillReview(cmd.Context(), args[0], action, digest)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, fmt.Sprintf("%s: %s\n", result.Review.Name, result.Review.State))
	}
	return cmd
}

func agentSkillReviewText(review contract.AgentSkillReviewPayload) string {
	var text strings.Builder
	fmt.Fprintf(
		&text,
		"%s (%s)\nDigest: %s\nRuns automatically: %d hooks, %d MCP servers\nSupporting scripts: %d\n",
		review.Name,
		review.State,
		review.Digest,
		review.RunsAutomatically.Hooks,
		review.RunsAutomatically.MCPServers,
		review.RunsAutomatically.Scripts,
	)
	for _, file := range review.Files {
		fmt.Fprintf(&text, "\n--- %s ---\n%s\n", file.Path, file.Content)
	}
	return text.String()
}
