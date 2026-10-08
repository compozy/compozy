package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/spf13/cobra"
)

type sessionCompactClient interface {
	CompactSession(context.Context, string) (contract.SessionCompactResponse, error)
}

type sessionCompactOutcome struct {
	SessionID string `json:"session_id"`
	PromptID  string `json:"prompt_id"`
	Command   string `json:"command"`
	Outcome   string `json:"outcome"`
}

// CompactSession requests the agent's advertised native compaction command.
func (c *daemonClient) CompactSession(ctx context.Context, id string) (contract.SessionCompactResponse, error) {
	var response contract.SessionCompactResponse
	path, err := c.sessionScopedPath(ctx, id, "/compact")
	if err != nil {
		return response, err
	}
	err = c.doJSON(ctx, http.MethodPost, path, nil, struct{}{}, &response)
	return response, err
}

func newSessionCompactCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "compact <id>",
		Short: "Request native agent compaction and wait for its turn (experimental)",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := resolveOutputFormat(cmd)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			compactClient, ok := client.(sessionCompactClient)
			if !ok {
				return errors.New("cli: daemon client does not support session compaction")
			}
			accepted, err := compactClient.CompactSession(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if mode == OutputHuman {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Compaction requested: /%s (prompt %s)\n", accepted.Command, accepted.PromptID); err != nil {
					return err
				}
			}
			outcome, err := waitSessionCompaction(cmd.Context(), client, accepted)
			if err != nil {
				return err
			}
			value := sessionCompactOutcome{SessionID: accepted.SessionID, PromptID: accepted.PromptID, Command: accepted.Command, Outcome: outcome}
			if mode == OutputJSONL {
				return writeJSONLine(cmd, value)
			}
			return writeCommandOutput(cmd, outputBundle{
				jsonValue: value,
				human:     func() (string, error) { return "Compaction " + outcome, nil },
				toon: func() (string, error) {
					return renderToonObject("session_compact", []string{"session_id", "prompt_id", "command", "outcome"}, []string{value.SessionID, value.PromptID, value.Command, value.Outcome}), nil
				},
			})
		},
	}
}

func waitSessionCompaction(ctx context.Context, client sessionEventStreamClient, accepted contract.SessionCompactResponse) (string, error) {
	if accepted.PromptID == "" {
		return "", errors.New("cli: compaction request did not return prompt_id")
	}
	terminal := errors.New("compaction turn ended")
	outcome := ""
	err := client.StreamSessionEvents(ctx, accepted.SessionID, SessionEventQuery{Forward: true, TurnID: accepted.PromptID}, "", func(frame SSEEvent) error {
		var event SessionEventRecord
		if len(frame.Data) == 0 {
			return nil
		}
		if err := json.Unmarshal(frame.Data, &event); err != nil {
			return fmt.Errorf("cli: decode compaction turn event: %w", err)
		}
		if event.TurnID != accepted.PromptID {
			return nil
		}
		switch event.Type {
		case acp.EventTypeCompaction:
			payload, err := transcript.UnmarshalAgentEvent(string(event.Content))
			if err != nil {
				return fmt.Errorf("cli: decode compaction snapshot: %w", err)
			}
			if payload.Compaction != nil && outcome == "" {
				switch payload.Compaction.Status {
				case "completed", "failed", "cancelled": //nolint:misspell // ACP wire spelling.
					outcome = payload.Compaction.Status
				}
			}
		case acp.EventTypeDone:
			if outcome == "" {
				outcome = "turn_completed"
			}
			return terminal
		case acp.EventTypeError:
			if outcome == "" {
				outcome = "turn_failed"
			}
			return terminal
		}
		return nil
	})
	if errors.Is(err, terminal) {
		return outcome, nil
	}
	if err != nil {
		return "", err
	}
	return "", errors.New("cli: compaction event stream ended before the turn completed")
}
