package cli

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func sessionWaitBundle(outcome SessionWaitRecord) outputBundle {
	state := outcome.State
	if state == "" {
		state = outcome.Outcome
	}
	return outputBundle{
		jsonValue: outcome,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, outcome)
		},
		human: func() (string, error) {
			return renderHumanSection("Session Wait", []keyValue{
				{Label: sessionStateValue, Value: stringOrDash(state)},
				{Label: "Waited", Value: (time.Duration(outcome.WaitedMS) * time.Millisecond).String()},
				{Label: sessionSessionValue, Value: stringOrDash(outcome.SessionID)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("session_wait", []string{
				sessionSessionIDKey, cliOutcomeKey, sessionStateKey, "waited_ms", "revision", "resume_id",
			}, []string{
				outcome.SessionID,
				outcome.Outcome,
				outcome.State,
				strconv.FormatInt(outcome.WaitedMS, 10),
				strconv.FormatInt(outcome.Revision, 10),
				outcome.ResumeID,
			}), nil
		},
	}
}
