package cli

import "github.com/spf13/cobra"

func sessionPromptCancelBundle(result SessionPromptCancelRecord) outputBundle {
	return outputBundle{
		jsonValue: result,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, result)
		},
		human: func() (string, error) {
			values := []keyValue{{Label: sessionSessionValue, Value: stringOrDash(result.SessionID)}}
			if result.TurnID != "" {
				values = append([]keyValue{{Label: "Canceled", Value: "turn " + result.TurnID}}, values...)
			} else {
				values = append([]keyValue{{Label: taskOutcomeValue, Value: "nothing in flight"}}, values...)
			}
			return renderHumanSection("Prompt Cancel", values), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"session_prompt_cancel",
				[]string{sessionSessionIDKey, cliOutcomeKey, sessionTurnIDKey},
				[]string{result.SessionID, result.Outcome, result.TurnID},
			), nil
		},
	}
}
