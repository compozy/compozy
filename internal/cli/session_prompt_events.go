package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/spf13/cobra"
)

type sessionPromptClient interface {
	SteerSessionPrompt(context.Context, string, contract.SteerPromptRequest) (SessionPromptRecord, error)
	SendSessionPrompt(context.Context, string, SessionPromptRequest) (SessionPromptRecord, error)
}

type sessionPromptStreamClient interface {
	StreamPromptSession(context.Context, string, SessionPromptRequest, SSEHandler) error
}

type sessionEventStreamClient interface {
	StreamSessionEvents(context.Context, string, SessionEventQuery, string, SSEHandler) error
}

type sessionPromptCommandFlags struct {
	queue          bool
	interrupt      bool
	steer          bool
	expectedTurnID string
	messageID      string
	idempotencyKey string
	runtime        sessionPromptRuntimeFlags
}

var (
	errPromptIdentityPairRequired = errors.New("cli: --message-id and --idempotency-key must be provided together")
	errPromptIdentityBlank        = errors.New("cli: prompt identity flags must not be blank")
)

func newSessionPromptCommand(deps commandDeps) *cobra.Command {
	flags := &sessionPromptCommandFlags{}
	cmd := newSessionPromptCobraCommand(deps, flags)
	cmd.Flags().BoolVar(&flags.queue, "queue", false, "Queue the prompt if the session is busy")
	cmd.Flags().BoolVar(&flags.interrupt, "interrupt", false, "Interrupt the active turn before sending this prompt")
	cmd.Flags().BoolVar(&flags.steer, "steer", false, "Replace the fenced active turn with steering guidance")
	cmd.Flags().StringVar(
		&flags.expectedTurnID,
		"expected-turn-id",
		"",
		"Active turn id that this steer or interrupt request must match",
	)
	cmd.Flags().StringVar(&flags.messageID, "message-id", "", "Stable authored message id for an explicit retry")
	cmd.Flags().StringVar(
		&flags.idempotencyKey,
		"idempotency-key",
		"",
		"Stable command idempotency key for an explicit retry",
	)
	bindSessionPromptRuntimeFlags(cmd, &flags.runtime)
	return cmd
}

func newSessionPromptCobraCommand(deps commandDeps, flags *sessionPromptCommandFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "prompt <id> <message>",
		Short: "Send a prompt to a session",
		Example: `  # Send a follow-up prompt to an active session
  compozy session prompt sess_1234 "Summarize the current changes."

  # Select the runtime snapshot for this prompt
  compozy session prompt sess_1234 "Review the patch." \
    --provider codex --model gpt-5.6-sol --reasoning-effort high --speed fast

  # Queue input while the session is busy
  compozy session prompt sess_1234 "Run the next check." --queue

  # Replace the fenced active turn with steering guidance
  compozy session prompt sess_1234 "Prefer the smaller patch." --steer --expected-turn-id turn_1234

  # Interrupt the fenced active turn before sending a replacement
  compozy session prompt sess_1234 "Use this direction instead." --interrupt --expected-turn-id turn_1234

  # Manage an already queued input
  compozy session input list sess_1234
  compozy session input cancel sess_1234 queue_entry_1234

  # Retry the same submission with its original identities
  compozy session prompt sess_1234 "Run the next check." \
    --message-id msg_1234 --idempotency-key idem_1234`,
		Args: flags.validateArgs,
		RunE: flags.run(deps),
	}
}

func (flags *sessionPromptCommandFlags) validateArgs(cmd *cobra.Command, args []string) error {
	modeCount := 0
	for _, enabled := range []bool{flags.queue, flags.interrupt, flags.steer} {
		if enabled {
			modeCount++
		}
	}
	if modeCount > 1 {
		return errors.New("cli: choose only one of --queue, --interrupt, or --steer")
	}
	if flags.steer || flags.interrupt {
		if _, err := changedNonEmptyStringFlag(cmd, "expected-turn-id", flags.expectedTurnID); err != nil {
			return err
		}
	} else if cmd.Flags().Changed("expected-turn-id") {
		return errors.New("cli: --expected-turn-id applies only to --steer or --interrupt")
	}
	if len(args) != 2 {
		return fmt.Errorf("cli: session prompt accepts 2 args, received %d", len(args))
	}
	return nil
}

func (flags *sessionPromptCommandFlags) run(deps commandDeps) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		mode, err := resolveOutputFormat(cmd)
		if err != nil {
			return err
		}
		client, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		runtimeSelection, err := flags.runtime.selection(cmd)
		if err != nil {
			return err
		}
		if runtimeSelection != nil && flags.steer {
			return errors.New("cli: runtime selection applies only to submitted prompts")
		}
		if mode == OutputJSONL {
			return flags.streamJSONL(cmd, client, args, runtimeSelection)
		}
		record, err := runSessionPromptAction(cmd, client, args, flags, runtimeSelection)
		if err != nil {
			return err
		}
		return writeCommandOutput(cmd, sessionPromptBundle(record))
	}
}

func (flags *sessionPromptCommandFlags) streamJSONL(
	cmd *cobra.Command,
	client sessionPromptStreamClient,
	args []string,
	runtime *contract.PromptRuntimeSelectionPayload,
) error {
	if flags.queue || flags.interrupt || flags.steer {
		return errors.New("cli: busy-input prompt actions do not support jsonl output")
	}
	messageID, idempotencyKey, err := flags.promptIdentity(cmd)
	if err != nil {
		return err
	}
	return streamPromptEventsJSONL(cmd, client, args[0], SessionPromptRequest{
		Message: args[1], MessageID: messageID, IdempotencyKey: idempotencyKey, Runtime: runtime,
	})
}

func runSessionPromptAction(
	cmd *cobra.Command,
	client sessionPromptClient,
	args []string,
	flags *sessionPromptCommandFlags,
	runtime *contract.PromptRuntimeSelectionPayload,
) (SessionPromptRecord, error) {
	if cmd == nil {
		return SessionPromptRecord{}, errors.New("cli: command is required")
	}
	if client == nil {
		return SessionPromptRecord{}, errors.New("cli: daemon client is required")
	}
	messageID, idempotencyKey, err := flags.promptIdentity(cmd)
	if err != nil {
		return SessionPromptRecord{}, err
	}
	if flags.steer {
		expectedTurnID, err := changedNonEmptyStringFlag(cmd, "expected-turn-id", flags.expectedTurnID)
		if err != nil {
			return SessionPromptRecord{}, err
		}
		return client.SteerSessionPrompt(cmd.Context(), args[0], contract.SteerPromptRequest{
			Text: args[1], MessageID: messageID, IdempotencyKey: idempotencyKey, ExpectedTurnID: expectedTurnID,
		})
	}
	request := SessionPromptRequest{
		Message: args[1], MessageID: messageID, IdempotencyKey: idempotencyKey, Runtime: runtime,
	}
	if flags.queue {
		request.Mode = contract.PromptModeQueue
	}
	if flags.interrupt {
		request.Mode = contract.PromptModeInterrupt
		expectedTurnID, err := changedNonEmptyStringFlag(cmd, "expected-turn-id", flags.expectedTurnID)
		if err != nil {
			return SessionPromptRecord{}, err
		}
		request.ExpectedTurnID = expectedTurnID
	}
	return client.SendSessionPrompt(cmd.Context(), args[0], request)
}

func (flags *sessionPromptCommandFlags) promptIdentity(cmd *cobra.Command) (string, string, error) {
	return promptIdentityFromFlags(cmd, flags.messageID, flags.idempotencyKey)
}

func promptIdentityFromFlags(
	cmd *cobra.Command,
	rawMessageID string,
	rawIdempotencyKey string,
) (string, string, error) {
	if cmd == nil {
		return "", "", errors.New("cli: command is required")
	}
	messageChanged := cmd.Flags().Changed("message-id")
	idempotencyChanged := cmd.Flags().Changed("idempotency-key")
	if messageChanged != idempotencyChanged {
		return "", "", errPromptIdentityPairRequired
	}
	if !messageChanged {
		messageID, err := store.NewID("msg")
		if err != nil {
			return "", "", fmt.Errorf("cli: generate prompt message id: %w", err)
		}
		idempotencyKey, err := store.NewID("idem")
		if err != nil {
			return "", "", fmt.Errorf("cli: generate prompt idempotency key: %w", err)
		}
		return messageID, idempotencyKey, nil
	}
	messageID := strings.TrimSpace(rawMessageID)
	idempotencyKey := strings.TrimSpace(rawIdempotencyKey)
	if messageID == "" || idempotencyKey == "" {
		return "", "", errPromptIdentityBlank
	}
	return messageID, idempotencyKey, nil
}

func newSessionEventsCommand(deps commandDeps) *cobra.Command {
	var (
		eventType     string
		last          int
		afterSequence int64
		sinceRaw      string
		follow        bool
	)

	cmd := &cobra.Command{
		Use:   "events <id>",
		Short: "Read session events",
		Example: `  # Read the latest stored events
  compozy session events sess_1234 --last 20

  # Follow new events until interrupted
  compozy session events sess_1234 --follow`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			since, err := parseSinceFlag(sinceRaw, deps.now)
			if err != nil {
				return err
			}
			if err := validateSessionEventCursorFlags(last, afterSequence); err != nil {
				return err
			}
			query := SessionEventQuery{
				Type:          eventType,
				Last:          last,
				AfterSequence: afterSequence,
				Since:         since,
			}

			if follow {
				return streamSessionEvents(cmd, client, args[0], query)
			}

			events, err := client.SessionEvents(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeSessionEventsOutput(cmd, events)
		},
	}
	cmd.Flags().StringVar(&eventType, extensionTypeKey, "", "Filter by event type")
	cmd.Flags().IntVar(&last, "last", 0, "Show only the most recent N events")
	cmd.Flags().Int64Var(&afterSequence, "after", 0, "Show events after the supplied sequence")
	cmd.Flags().StringVar(&sinceRaw, "since", "", "Show events since an RFC3339 timestamp or relative duration")
	cmd.Flags().BoolVar(&follow, "follow", false, "Stream new events over SSE")
	return cmd
}

func streamPromptEventsJSONL(
	cmd *cobra.Command,
	client sessionPromptStreamClient,
	id string,
	request SessionPromptRequest,
) error {
	return client.StreamPromptSession(cmd.Context(), id, request, func(event SSEEvent) error {
		if len(event.Data) == 0 || strings.TrimSpace(string(event.Data)) == "[DONE]" {
			return nil
		}

		var payload any
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return fmt.Errorf("cli: decode prompt event: %w", err)
		}
		return writeJSONLine(cmd, payload)
	})
}

func writeSessionEventsOutput(cmd *cobra.Command, events []SessionEventRecord) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSONL {
		return writeJSONLines(cmd, events)
	}
	return writeCommandOutput(cmd, sessionEventsBundle(events))
}

func newSessionHistoryCommand(deps commandDeps) *cobra.Command {
	var (
		last          int
		afterSequence int64
	)

	cmd := &cobra.Command{
		Use:   sessionHistoryIDValue,
		Short: "Show session history grouped by turn",
		Example: `  # Show replayable turn history for one session
  compozy session history sess_1234

  # Show turns after a known event sequence
  compozy session history sess_1234 --after 120 --last 10`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := validateSessionEventCursorFlags(last, afterSequence); err != nil {
				return err
			}

			history, err := client.SessionHistory(cmd.Context(), args[0], SessionEventQuery{
				Last:          last,
				AfterSequence: afterSequence,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionHistoryBundle(history))
		},
	}
	cmd.Flags().IntVar(&last, "last", 0, "Show only the most recent N turns")
	cmd.Flags().Int64Var(&afterSequence, "after", 0, "Show turns after the supplied event sequence")
	return cmd
}

func streamSessionEvents(
	cmd *cobra.Command,
	client sessionEventStreamClient,
	id string,
	query SessionEventQuery,
) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)

	err = client.StreamSessionEvents(cmd.Context(), id, query, "", func(event SSEEvent) error {
		var payload SessionEventRecord
		if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &payload); err != nil {
				return fmt.Errorf("cli: decode session stream event: %w", err)
			}
		}
		if payload.Type == "" {
			payload.Type = event.Event
		}

		switch mode {
		case OutputJSON, OutputJSONL:
			if err := encoder.Encode(payload); err != nil {
				return err
			}
		case OutputToon:
			if err := writeRawCommandOutput(cmd, renderToonObject("event", []string{
				sessionSequenceKey,
				extensionTypeKey,
				sessionAgentNameKey,
				sessionTurnIDKey,
				networkTimestampKey,
				memoryContentKey,
			}, []string{
				strconv.FormatInt(payload.Sequence, 10),
				payload.Type,
				payload.AgentName,
				payload.TurnID,
				formatTime(payload.Timestamp),
				compactJSON(payload.Content),
			})); err != nil {
				return err
			}
		default:
			if err := writeRawCommandOutput(cmd, strings.Join([]string{
				stringOrDash(formatTime(payload.Timestamp)),
				stringOrDash(payload.Type),
				stringOrDash(payload.AgentName),
				stringOrDash(compactJSON(payload.Content)),
			}, "  ")); err != nil {
				return err
			}
		}

		if payload.Type == session.EventTypeSessionStopped {
			return errStopSSE
		}
		return nil
	})
	if errors.Is(err, errStopSSE) {
		return nil
	}
	return err
}
