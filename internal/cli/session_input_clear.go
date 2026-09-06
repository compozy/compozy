package cli

import (
	"context"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func (c *daemonClient) ClearSessionInputs(
	ctx context.Context,
	sessionID string,
) (contract.SessionInputClearResponse, error) {
	var response contract.SessionInputClearResponse
	path, err := c.sessionScopedPath(ctx, sessionID, "/prompt/queue")
	if err != nil {
		return response, err
	}
	if err := c.doJSON(ctx, http.MethodDelete, path, nil, nil, &response); err != nil {
		return response, err
	}
	return response, nil
}

func newSessionInputClearCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use: "clear <session-id>", Short: "Clear parked input with per-entry transcript traces",
		Args: exactOneNonBlankArg(), Example: "  compozy session input clear sess_1234",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.ClearSessionInputs(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			bundle := sessionInputListBundle(contract.SessionInputListResponse{Inputs: result.Inputs})
			bundle.jsonValue = result
			bundle.jsonl = func(cmd *cobra.Command) error { return writeJSONLine(cmd, result) }
			bundle.human = func() (string, error) {
				return fmt.Sprintf("Cleared %d queued entries (traced in the transcript).", result.ClearedCount), nil
			}
			return writeCommandOutput(cmd, bundle)
		},
	}
}
