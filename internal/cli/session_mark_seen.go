package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newSessionMarkSeenCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "mark-seen <id>",
		Short: "Mark a session's Needs you as seen",
		Long: "Mark a session's Needs you as seen. A turn that ended early, a return from Later, or a stopped " +
			"session's failure leaves Needs you until it happens again. A session that still waits on a " +
			"permission answer, a question, a sign-in, or a stop is refused; answer or resolve it instead. " +
			"A session that does not need you is returned unchanged.",
		Example: `  # Clear the Needs you of a session you already looked at
  compozy session mark-seen sess-123`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.MarkSessionSeen(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&record, deps.now))
		},
	}
}

// MarkSessionSeen clears a session's Needs you until its next occurrence.
func (c *daemonClient) MarkSessionSeen(ctx context.Context, id string) (SessionRecord, error) {
	var response contract.SessionResponse
	path, err := c.sessionScopedPath(ctx, id, "/mark-seen")
	if err != nil {
		return SessionRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}
