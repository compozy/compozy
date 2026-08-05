package cli

import (
	"errors"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/spf13/cobra"
)

func newSessionRewindCommand(deps commandDeps) *cobra.Command {
	var epoch, generation, maxSequence int64
	var idempotencyKey, messageID string
	cmd := &cobra.Command{
		Use:   "rewind <session-id>",
		Short: "Rewind a session before a user message",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if epoch < 0 || generation < 0 || maxSequence < 0 {
				return errors.New("cli: rewind transcript fences cannot be negative")
			}
			selectedMessageID := strings.TrimSpace(messageID)
			if selectedMessageID == "" {
				return errors.New("cli: --message-id is required")
			}
			key := strings.TrimSpace(idempotencyKey)
			if key == "" {
				var err error
				key, err = store.NewID("idem")
				if err != nil {
					return err
				}
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			changedFences := 0
			for _, name := range []string{"expected-epoch", "expected-generation", "expected-max-sequence"} {
				if cmd.Flags().Changed(name) {
					changedFences++
				}
			}
			if changedFences != 0 && changedFences != 3 {
				return errors.New("cli: set all transcript fence flags together or omit all three")
			}
			if changedFences == 0 {
				transcript, err := client.GetSessionTranscript(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				epoch = transcript.Epoch
				generation = transcript.Generation
				maxSequence = transcript.MaxSequence
			}
			result, err := client.RewindSession(cmd.Context(), args[0], SessionRewindRequest{
				MessageID: selectedMessageID, IdempotencyKey: key, ExpectedEpoch: &epoch,
				ExpectedGeneration: &generation, ExpectedMaxSequence: &maxSequence,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionRewindBundle(&result))
		},
	}
	cmd.Flags().Int64Var(&epoch, "expected-epoch", 0, "Transcript epoch returned by the transcript API")
	cmd.Flags().Int64Var(&generation, "expected-generation", 0, "Transcript generation returned by the transcript API")
	cmd.Flags().Int64Var(
		&maxSequence,
		"expected-max-sequence",
		0,
		"Maximum event sequence returned by the transcript API",
	)
	cmd.Flags().StringVar(&messageID, "message-id", "", "Durable user message to rewind before")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Stable retry key (generated when omitted)")
	return cmd
}

func sessionRewindBundle(record *SessionRewindRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Session Rewind", []keyValue{
				{Label: sessionSessionValue, Value: stringOrDash(record.Session.ID)},
				{Label: authoredContextMessageValue, Value: stringOrDash(record.Rewind.TargetMessageID)},
				{Label: "Archived events", Value: strconv.FormatInt(record.Rewind.ArchivedEvents, 10)},
				{Label: "Draft", Value: stringOrDash(record.Rewind.DraftText)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("session_rewind", []string{
				sessionSessionIDKey, "target_message_id", "archived_event_count", "draft_text",
			}, []string{
				record.Session.ID, record.Rewind.TargetMessageID,
				strconv.FormatInt(record.Rewind.ArchivedEvents, 10), record.Rewind.DraftText,
			}), nil
		},
	}
}
