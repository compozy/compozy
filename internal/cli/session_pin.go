package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func newSessionPinCommand(deps commandDeps) *cobra.Command {
	return newSessionPinChangeCommand(deps, true)
}

func newSessionUnpinCommand(deps commandDeps) *cobra.Command {
	return newSessionPinChangeCommand(deps, false)
}

func newSessionPinChangeCommand(deps commandDeps, pin bool) *cobra.Command {
	command := &cobra.Command{
		Use:   "pin <id>",
		Short: "Pin a session above the session list",
		Long: "Pin a session above the session list. It moves to the top of the Pinned section, above every " +
			"project group and folder, and carries its child sessions with it, nested under it. Its folder, " +
			"manual position and activity order, and those of its children, are kept. Pinning a pinned session " +
			"changes nothing. Reorder pins with `compozy session reorder --pinned`. Subagent sessions and " +
			"always-on agents' main sessions cannot be pinned.",
		Example: `  # Keep a session and its children at the top of the list
  compozy session pin sess-123

  # List what is pinned, in Pinned order
  compozy session list --pinned only --sort pinned`,
	}
	if !pin {
		command.Use = "unpin <id>"
		command.Short = "Unpin a session so it returns to its place in the list"
		command.Long = "Unpin a session. It returns with its child sessions to its project group or folder at the " +
			"position it held. Unpinning a session that is not pinned changes nothing."
		command.Example = `  compozy session unpin sess-123`
	}
	command.Args = exactOneNonBlankArg()
	command.RunE = func(cmd *cobra.Command, args []string) error {
		client, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		record, err := client.SetSessionPinned(cmd.Context(), args[0], pin)
		if err != nil {
			return err
		}
		return writeCommandOutput(cmd, sessionBundle(&record, deps.now))
	}
	return command
}

// SetSessionPinned pins (POST) or unpins (DELETE) a session.
func (c *daemonClient) SetSessionPinned(ctx context.Context, id string, pinned bool) (SessionRecord, error) {
	var response contract.SessionResponse
	path, err := c.sessionScopedPath(ctx, id, "/pin")
	if err != nil {
		return SessionRecord{}, err
	}
	method := http.MethodPost
	if !pinned {
		method = http.MethodDelete
	}
	if err := c.doJSON(ctx, method, path, nil, nil, &response); err != nil {
		return SessionRecord{}, err
	}
	return response.Session, nil
}
