package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type sessionPullRequestClient interface {
	RefreshSessionPullRequest(context.Context, string) (contract.SessionPullRequestResponse, error)
}

func (c *daemonClient) RefreshSessionPullRequest(
	ctx context.Context,
	id string,
) (contract.SessionPullRequestResponse, error) {
	var response contract.SessionPullRequestResponse
	target, err := requirePathValue("session_id", id)
	if err != nil {
		return response, err
	}
	err = c.doJSON(
		ctx,
		http.MethodPost,
		"/api/sessions/"+url.PathEscape(target)+"/pull-request/refresh",
		nil,
		nil,
		&response,
	)
	return response, err
}

func newSessionPRCommand(deps commandDeps) *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{
		Use:     "pr <id>",
		Short:   "Show the pull request linked to a session branch",
		Args:    exactOneNonBlankArg(),
		Example: "  compozy session pr sess_1234\n  compozy session pr sess_1234 --refresh -o json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			var result contract.SessionPullRequestResponse
			if refresh {
				refresher, ok := client.(sessionPullRequestClient)
				if !ok {
					return errors.New("cli: pull request refresh is unavailable")
				}
				result, err = refresher.RefreshSessionPullRequest(cmd.Context(), args[0])
			} else {
				var record SessionRecord
				record, err = client.GetSession(cmd.Context(), args[0])
				result = contract.SessionPullRequestResponse{
					SessionID:         record.ID,
					Branch:            record.Branch,
					PullRequest:       record.PullRequest,
					PullRequestLookup: record.PullRequestLookup,
				}
			}
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				outputBundle{
					jsonValue: result,
					human:     func() (string, error) { return sessionPRText(result), nil },
					toon:      func() (string, error) { return sessionPRText(result), nil },
				},
			)
		},
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh the linked pull request from its forge provider")
	return cmd
}

func sessionPRText(result contract.SessionPullRequestResponse) string {
	if result.Branch == "" {
		return "No branch linked to this session.\n"
	}
	lookup := "pending"
	if result.PullRequestLookup != nil {
		lookup = string(result.PullRequestLookup.State)
		if result.PullRequestLookup.Stale {
			lookup += " (stale)"
		}
		if result.PullRequestLookup.Cause != "" {
			lookup += ": " + result.PullRequestLookup.Cause
		}
	}
	if result.PullRequest == nil {
		return fmt.Sprintf("Branch: %s\nPull request: none\nLookup: %s\n", result.Branch, lookup)
	}
	pr := result.PullRequest
	return fmt.Sprintf(
		"Branch: %s\nPR #%d: %s\nState: %s\nChecks: %s\nReady to merge: %t\nLookup: %s\n",
		result.Branch,
		pr.Number,
		pr.URL,
		pr.State,
		pr.Checks.State,
		pr.ReadyToMerge,
		lookup,
	)
}
