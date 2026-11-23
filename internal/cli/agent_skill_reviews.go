package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	root.AddCommand(review, newAgentSkillInstallConflictCommand(deps))
	for _, action := range []string{agentListKey, configShowKey, loopApproveKey, "reject"} {
		review.AddCommand(newAgentSkillReviewAction(deps, action))
	}
	return root
}

func newAgentSkillReviewAction(deps commandDeps, action string) *cobra.Command {
	var instance, state, kind, decidedVia, cursor, digest string
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
		cmd.Flags().StringVar(&state, appTokenStateFlag, "", "pending, approved, rejected or reverted")
		cmd.Flags().StringVar(&kind, cliKindKey, "", "new or edit")
		cmd.Flags().StringVar(&decidedVia, "decided-via", "", "operator or auto_approve")
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
				cliKindKey:        {kind},
				"decided_via":     {decidedVia},
				"cursor":          {cursor},
				"limit":           {strconv.Itoa(limit)},
			}
			result, err := client.ListAgentSkillReviews(cmd.Context(), query)
			if err != nil {
				return err
			}
			var output strings.Builder
			output.WriteString(agentSkillReviewTable(result.Reviews))
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

func agentSkillReviewTable(reviews []contract.AgentSkillReviewPayload) string {
	rows := make([][]string, 0, len(reviews))
	for _, review := range reviews {
		decidedAt := ""
		if review.DecidedAt != nil {
			decidedAt = review.DecidedAt.UTC().Format(time.RFC3339)
		}
		rows = append(rows, []string{
			review.ID, review.Name, review.Kind, review.State, review.DecidedVia, decidedAt,
		})
	}
	headers := []string{"ID", cliNameHeader, cliKindHeader, cliStateHeader, "DECIDED VIA", "DECIDED AT"}
	return renderHumanTable("", headers, rows) + "\n"
}

func agentSkillReviewText(review contract.AgentSkillReviewPayload) string {
	var text strings.Builder
	kind := review.Kind
	if kind == "" {
		kind = "new"
	}
	fmt.Fprintf(&text, "%s (%s) · %s\nDigest: %s", review.Name, kind, review.State, review.Digest)
	if review.BaseDigest != "" {
		fmt.Fprintf(&text, "   Base: %s", review.BaseDigest)
	}
	text.WriteString("\n")
	if lesson := review.Lesson; lesson != nil {
		fmt.Fprintf(&text, "Lesson (%s confidence)\n  Cue:      %s\n  Lesson:   %s\n  Evidence: %s\n",
			lesson.Confidence, lesson.Cue, lesson.Lesson, lesson.Evidence)
	} else {
		text.WriteString("No lesson recorded\n")
	}
	if len(review.EvidenceSessionIDs) > 0 {
		fmt.Fprintf(&text, "  Sessions: %s\n", strings.Join(review.EvidenceSessionIDs, ", "))
	}
	if kind == skillReviewKindEdit {
		agentSkillEditText(&text, review)
		return text.String()
	}
	fmt.Fprintf(
		&text,
		"Runs automatically: %d hooks, %d MCP servers\nSupporting scripts: %d\n",
		review.RunsAutomatically.Hooks,
		review.RunsAutomatically.MCPServers,
		review.RunsAutomatically.Scripts,
	)
	for _, file := range review.Files {
		fmt.Fprintf(&text, "--- %s\n%s\n", file.Path, file.Content)
	}
	return text.String()
}
