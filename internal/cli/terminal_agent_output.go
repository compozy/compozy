package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func terminalExecBundle(result terminalpkg.ExecResult) outputBundle {
	return outputBundle{
		jsonValue: result,
		human: func() (string, error) {
			if result.StillRunning && result.TerminalID != nil {
				return fmt.Sprintf("Still running in %s.", *result.TerminalID), nil
			}
			return result.Output, nil
		},
	}
}

func terminalSignalBundle(id, signal string) outputBundle {
	return outputBundle{
		jsonValue: map[string]any{networkDeliveredKey: true},
		human: func() (string, error) {
			return fmt.Sprintf("SIG%s delivered to %s.", signal, id), nil
		},
	}
}

func terminalInputRequestsBundle(requests TerminalInputRequests) outputBundle {
	value := map[string]any{"pending": requests.Pending, "resolved": requests.Resolved}
	return outputBundle{
		jsonValue: value,
		json:      terminalHTTPJSON(value),
		human: func() (string, error) {
			if len(requests.Pending) == 0 && len(requests.Resolved) == 0 {
				return "No pending input requests.", nil
			}
			lines := []string{"REQUEST\tTERMINAL\tPROFILE\tSTATE\tREDACTED"}
			for _, request := range requests.Pending {
				lines = append(lines, fmt.Sprintf(
					"%s\t%s\t%s\t%s\t%t",
					request.ID, request.TerminalID, request.ProfileName, "pending", request.Redacted,
				))
			}
			for _, request := range requests.Resolved {
				lines = append(lines, fmt.Sprintf(
					"%s\t%s\t%s\t%s\t%t",
					request.ID, request.TerminalID, request.ProfileName, request.Outcome, request.Redacted,
				))
			}
			return strings.Join(lines, "\n"), nil
		},
	}
}

func terminalAnsweredInputBundle(requestID string, delivered int, redacted bool) outputBundle {
	return outputBundle{
		jsonValue: map[string]any{
			terminalRequestIDKey: requestID, "outcome": terminalOutcomeAnswered,
			"delivered_bytes": delivered, "redacted": redacted,
		},
		human: func() (string, error) {
			label := ""
			if redacted {
				label = "redacted, "
			}
			return fmt.Sprintf("Delivered (%s%d bytes).", label, delivered), nil
		},
	}
}

func terminalRejectedInputBundle(requestID string) outputBundle {
	return outputBundle{
		jsonValue: map[string]any{terminalRequestIDKey: requestID, "outcome": terminalOutcomeRejected},
		human: func() (string, error) {
			return "Rejected — the agent was notified.", nil
		},
	}
}

func terminalJournalBundle(page terminalpkg.Page) outputBundle {
	entries := make([]contract.TerminalCommandRowPayload, 0, len(page.Entries))
	for _, row := range page.Entries {
		entries = append(entries, contract.TerminalCommandRowPayloadFromDomain(row, row.ProfileName))
	}
	value := map[string]any{"entries": entries, terminalNextKey: nullableTerminalCursor(page.Next)}
	return outputBundle{
		jsonValue: value,
		json:      terminalHTTPJSON(value),
		human: func() (string, error) {
			if len(page.Entries) == 0 {
				return "No terminal commands matched.", nil
			}
			lines := []string{"STARTED\tPROFILE\tACTOR\tEXIT\tDETECTED\tCOMMAND"}
			for _, entry := range page.Entries {
				exit := entry.ExitCause
				if entry.ExitCode != nil {
					exit = fmt.Sprintf("%s:%d", exit, *entry.ExitCode)
				}
				lines = append(lines, fmt.Sprintf(
					"%s\t%s\t%s\t%s\t%s\t%s",
					entry.StartedAt.Format("2006-01-02T15:04:05Z07:00"), entry.ProfileName,
					entry.Actor.Kind, exit, entry.DetectedBy, entry.Command,
				))
			}
			return strings.Join(lines, "\n"), nil
		},
	}
}

func terminalRecordingBundle(recording terminalpkg.RecordingRef, action string) outputBundle {
	return outputBundle{
		jsonValue: map[string]any{"recording": recording},
		human: func() (string, error) {
			if action == terminalRecordingStartAction {
				return fmt.Sprintf("Recording %s as %s.", recording.TerminalID, recording.ID), nil
			}
			return fmt.Sprintf("%s saved (%d bytes).", recording.ID, recording.Bytes), nil
		},
	}
}

func terminalQuoteBundle(id string, from, to int, content string) outputBundle {
	quote := terminalQuote(id, from, to, content)
	return outputBundle{
		jsonValue: map[string]any{"terminal_id": id, "from": from, "to": to, "quote": quote, "untrusted": true},
		human: func() (string, error) {
			return quote, nil
		},
	}
}

func terminalQuote(id string, from, to int, content string) string {
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	quoted := make([]string, 0, len(lines)+2)
	quoted = append(quoted, fmt.Sprintf(
		`<terminal_context terminal=%q lines="%d-%d">`,
		escapeTerminalContext(id),
		from,
		to,
	))
	for index, line := range lines {
		quoted = append(quoted, fmt.Sprintf("%d | %s", from+index, escapeTerminalContext(line)))
	}
	quoted = append(quoted, "</terminal_context>")
	return strings.Join(quoted, "\n")
}

var terminalContextEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&apos;",
)

func escapeTerminalContext(value string) string {
	return terminalContextEscaper.Replace(value)
}

func nullableTerminalCursor(cursor string) any {
	if cursor == "" {
		return nil
	}
	return cursor
}
