package cli

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type sessionWorkClient interface {
	SessionWorkLinks(context.Context, string, string, *contract.SessionWorkLinkRequest) (contract.SessionLinkedWorkResponse, error)
}

func (c *daemonClient) SessionWorkLinks(ctx context.Context, id, method string, request *contract.SessionWorkLinkRequest) (contract.SessionLinkedWorkResponse, error) {
	var response contract.SessionLinkedWorkResponse
	path, err := c.sessionScopedPath(ctx, id, "/links")
	if err != nil {
		return response, err
	}
	var body any
	if request != nil {
		body = request
	}
	err = c.doJSON(ctx, method, path, nil, body, &response)
	return response, err
}

func newSessionWorkLinkCommand(deps commandDeps, unlink bool) *cobra.Command {
	request := contract.SessionWorkLinkRequest{}
	use, short, method := "link <id> [branch]", "List or link work to a session", http.MethodPost
	if unlink {
		use, short, method = "unlink <id> <branch>", "Unlink work without deleting it", http.MethodDelete
	}
	cmd := &cobra.Command{Use: use, Short: short, Args: cobra.RangeArgs(1, 2), RunE: func(cmd *cobra.Command, args []string) error {
		client, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		links, ok := client.(sessionWorkClient)
		if !ok {
			return errors.New("session linked work client unavailable")
		}
		action := method
		var body *contract.SessionWorkLinkRequest
		if len(args) == 1 && request.PRNumber == 0 {
			if unlink {
				return errors.New("branch is required")
			}
			action = http.MethodGet
		} else {
			if len(args) > 1 {
				request.Branch = args[1]
			}
			body = &request
		}
		response, err := links.SessionWorkLinks(cmd.Context(), args[0], action, body)
		if err != nil {
			return err
		}
		return writeCommandOutput(cmd, outputBundle{jsonValue: response, human: func() (string, error) {
			var b strings.Builder
			for _, item := range response.Items {
				b.WriteString(item.Branch + "\n")
			}
			return b.String(), nil
		}})
	}}
	cmd.Flags().StringVar(&request.Repository, "repository", "", "Repository identity (defaults to this project)")
	cmd.Flags().IntVar(&request.PRNumber, "pr-number", 0, "Pull request number")
	return cmd
}
