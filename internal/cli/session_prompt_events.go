package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/spf13/cobra"
)

func newSessionPromptCommand(deps commandDeps) *cobra.Command {
	var (
		queue       bool
		interrupt   bool
		steer       bool
		cancelEntry string
	)

	cmd := &cobra.Command{
		Use:   "prompt <id> <message>",
		Short: "Send a prompt to a session",
		Example: `  # Send a follow-up prompt to an active session
  agh session prompt sess_1234 "Summarize the current changes."

  # Queue input while the session is busy
  agh session prompt sess_1234 "Run the next check." --queue

  # Steer the current turn after the next tool result
  agh session prompt sess_1234 "Prefer the smaller patch." --steer

  # Cancel queued input by entry id
  agh session prompt sess_1234 --cancel queue_entry_1234`,
		Args: func(cmd *cobra.Command, args []string) error {
			cancelChanged := cmd.Flags().Changed("cancel")
			modeCount := 0
			for _, enabled := range []bool{queue, interrupt, steer, cancelChanged} {
				if enabled {
					modeCount++
				}
			}
			if modeCount > 1 {
				return errors.New("cli: choose only one of --queue, --interrupt, --steer, or --cancel")
			}
			if cancelChanged {
				if strings.TrimSpace(cancelEntry) == "" {
					return errors.New("cli: --cancel requires a queue entry id")
				}
				if len(args) != 1 {
					return fmt.Errorf("cli: session prompt --cancel accepts 1 arg, received %d", len(args))
				}
				return nil
			}
			if len(args) != 2 {
				return fmt.Errorf("cli: session prompt accepts 2 args, received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := resolveOutputFormat(cmd)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if mode == OutputJSONL {
				if queue || interrupt || steer || cmd.Flags().Changed("cancel") {
					return errors.New("cli: busy-input prompt actions do not support jsonl output")
				}
				return streamPromptEventsJSONL(cmd, client, args[0], args[1])
			}

			record, err := runSessionPromptAction(cmd, client, args, queue, interrupt, steer, cancelEntry)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionPromptBundle(record))
		},
	}
	cmd.Flags().BoolVar(&queue, "queue", false, "Queue the prompt if the session is busy")
	cmd.Flags().BoolVar(&interrupt, "interrupt", false, "Interrupt the active turn before sending this prompt")
	cmd.Flags().BoolVar(&steer, "steer", false, "Stage steering guidance for the active turn")
	cmd.Flags().StringVar(&cancelEntry, "cancel", "", "Cancel a queued prompt entry by id")
	return cmd
}

func runSessionPromptAction(
	cmd *cobra.Command,
	client DaemonClient,
	args []string,
	queue bool,
	interrupt bool,
	steer bool,
	cancelEntry string,
) (SessionPromptRecord, error) {
	if cmd == nil {
		return SessionPromptRecord{}, errors.New("cli: command is required")
	}
	if client == nil {
		return SessionPromptRecord{}, errors.New("cli: daemon client is required")
	}
	if cmd.Flags().Changed("cancel") {
		return client.CancelQueuedSessionPrompt(cmd.Context(), args[0], strings.TrimSpace(cancelEntry))
	}
	if steer {
		return client.SteerSessionPrompt(cmd.Context(), args[0], args[1])
	}
	request := SessionPromptRequest{Message: args[1]}
	if queue {
		request.Mode = contract.PromptModeQueue
	}
	if interrupt {
		request.Mode = contract.PromptModeInterrupt
	}
	return client.SendSessionPrompt(cmd.Context(), args[0], request)
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
  agh session events sess_1234 --last 20

  # Follow new events until interrupted
  agh session events sess_1234 --follow`,
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

func streamPromptEventsJSONL(cmd *cobra.Command, client DaemonClient, id string, message string) error {
	return client.StreamPromptSession(cmd.Context(), id, message, func(event SSEEvent) error {
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
  agh session history sess_1234

  # Show turns after a known event sequence
  agh session history sess_1234 --after 120 --last 10`,
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

func streamSessionEvents(cmd *cobra.Command, client DaemonClient, id string, query SessionEventQuery) error {
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
