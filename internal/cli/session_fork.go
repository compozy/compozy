package cli

import (
	"context"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

// SessionForkRequest is the daemon fork request.
type SessionForkRequest = contract.ForkSessionRequest

type sessionForkFlags struct {
	messageID, name, idempotencyKey string
	epoch, generation, maxSequence  int64
}

func newSessionForkCommand(deps commandDeps) *cobra.Command {
	var flags sessionForkFlags
	cmd := &cobra.Command{
		Use:   "fork <session-id>",
		Short: "Fork a session with the same agent, whole or through one message and its turn",
		Long: "Start a new session with the same agent, runtime, and account as the source. Without " +
			"--message-id the whole conversation through the last settled turn is carried; with it, the " +
			"conversation through that user message and its turn (the agent's reply and tool work). When the " +
			"agent can clone its own session the new session loads that clone on its first prompt. The source " +
			"session is never changed. Re-running with the same --idempotency-key and arguments prints the " +
			"recorded result instead of creating a second session.",
		Example: `  compozy session fork sess_01J9R2K7M4T1
  compozy session fork sess_01J9R2K7M4T1 --message-id msg_01J9R2N3V0QF --name "Migration cleanup (alt)"`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := buildSessionForkRequest(cmd, &flags)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if request.ExpectedEpoch == nil {
				if err := fillDeriveFencesFromTranscript(cmd.Context(), client, args[0],
					&request.ExpectedEpoch, &request.ExpectedGeneration, &request.ExpectedMaxSequence); err != nil {
					return err
				}
			}
			record, err := client.ForkSession(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionDeriveBundle(&record))
		},
	}
	cmd.Flags().StringVar(&flags.messageID, "message-id", "",
		"Durable user message to fork through, including its turn (whole session when omitted)")
	cmd.Flags().StringVar(&flags.name, "name", "", "Name of the new session (defaults to the source title)")
	addDeriveFenceFlags(cmd, &flags.epoch, &flags.generation, &flags.maxSequence)
	cmd.Flags().StringVar(&flags.idempotencyKey, "idempotency-key", "", "Stable retry key (generated when omitted)")
	return cmd
}

func buildSessionForkRequest(cmd *cobra.Command, flags *sessionForkFlags) (SessionForkRequest, error) {
	request := SessionForkRequest{
		MessageID: strings.TrimSpace(flags.messageID), Name: strings.TrimSpace(flags.name),
	}
	if err := applyDeriveFenceFlags(cmd, flags.epoch, flags.generation, flags.maxSequence,
		&request.ExpectedEpoch, &request.ExpectedGeneration, &request.ExpectedMaxSequence); err != nil {
		return SessionForkRequest{}, err
	}
	key, err := deriveIdempotencyKey(flags.idempotencyKey)
	if err != nil {
		return SessionForkRequest{}, err
	}
	request.IdempotencyKey = key
	return request, nil
}

func (c *daemonClient) ForkSession(
	ctx context.Context,
	id string,
	request SessionForkRequest,
) (SessionDeriveRecord, error) {
	var response SessionDeriveRecord
	path, err := c.sessionScopedPath(ctx, id, "/fork")
	if err != nil {
		return SessionDeriveRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return SessionDeriveRecord{}, err
	}
	return response, nil
}
