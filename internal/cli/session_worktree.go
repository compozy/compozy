package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type sessionWorktreeClient interface {
	ChangeSessionWorktree(context.Context, string, string, string) (SessionRecord, error)
}

func newSessionWorktreeCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: "worktree", Short: "Change an idle session's execution location in place"}
	for _, bind := range []bool{true, false} {
		var expected string
		use, short, count := "unbind <session>", "Use the workspace root for the same conversation", 1
		if bind {
			use, short, count = "bind <session> <worktree-ref>", "Use a ready worktree for the same conversation", 2
		}
		child := &cobra.Command{
			Use:   use,
			Short: short,
			Args:  cobra.ExactArgs(count),
			RunE: func(cmd *cobra.Command, args []string) error {
				client, err := clientFromDeps(deps)
				if err != nil {
					return err
				}
				bindingClient, ok := client.(sessionWorktreeClient)
				if !ok {
					return errors.New("cli: session worktree client is unavailable")
				}
				if !cmd.Flags().Changed("expected-worktree") {
					current, err := client.GetSession(cmd.Context(), args[0])
					if err != nil {
						return err
					}
					expected = current.WorktreeID
				}
				ref := ""
				if bind {
					ref = strings.TrimSpace(args[1])
					if ref == "" {
						return errors.New("cli: worktree ref is required")
					}
				}
				info, err := bindingClient.ChangeSessionWorktree(cmd.Context(), args[0], ref, expected)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, sessionBundle(&info, deps.now))
			},
		}
		child.Flags().
			StringVar(&expected, "expected-worktree", "", "Expected current worktree id (empty for workspace root)")
		cmd.AddCommand(child)
	}
	return cmd
}

func (c *daemonClient) ChangeSessionWorktree(ctx context.Context, id, ref, expected string) (SessionRecord, error) {
	path, err := c.sessionScopedPath(ctx, id, "/worktree")
	if err != nil {
		return SessionRecord{}, err
	}
	var response contract.SessionResponse
	if ref == "" {
		err = c.doJSON(ctx, http.MethodDelete, path, url.Values{"expected_worktree_id": {expected}}, nil, &response)
	} else {
		err = c.doJSON(
			ctx,
			http.MethodPut,
			path,
			nil,
			contract.BindSessionWorktreeRequest{Worktree: ref, ExpectedWorktreeID: &expected},
			&response,
		)
	}
	return response.Session, err
}

var _ sessionWorktreeClient = (*daemonClient)(nil)
