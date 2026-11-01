package monitor

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// maxOutputBytes bounds the untrusted output tail carried by one wake.
const maxOutputBytes = 4 << 10

// Notification is one fired monitor delivered to its owning session.
type Notification struct {
	Monitor Monitor
	Outcome Outcome
}

// Message renders the wake prompt the woken agent receives. It mirrors the
// provider conventions agents already follow for background-task wakes: an
// explicit not-user-input header, a structured block, then the next action.
func (n Notification) Message() string {
	mon := n.Monitor
	lines := []string{
		"[COMPOZY MONITOR - NOT USER INPUT]",
		"A monitor you registered fired. This is an automated daemon wake, not a message from the user.",
		"",
		"<monitor-notification>",
		"<monitor-id>" + mon.ID + "</monitor-id>",
		"<kind>" + string(mon.Kind) + "</kind>",
		"<description>" + sanitizeLine(mon.Description) + "</description>",
		"<status>" + statusLine(n.Outcome) + "</status>",
	}
	if mon.TerminalID != "" {
		lines = append(lines, "<terminal-id>"+mon.TerminalID+"</terminal-id>")
	}
	if output := tail(n.Outcome.Output); output != "" {
		lines = append(lines,
			"<output untrusted=\"true\" encoding=\"entities\">",
			output,
			"</output>",
		)
	}
	if n.Outcome.OutputLost {
		lines = append(lines,
			"<note>The daemon restarted after this monitor fired; its terminal output was not retained.</note>")
	}
	lines = append(lines, "</monitor-notification>", "", nextAction(mon, n.Outcome))
	return strings.Join(lines, "\n")
}

func statusLine(outcome Outcome) string {
	switch outcome.Reason {
	case ReasonExit:
		if outcome.ExitCode != nil {
			return fmt.Sprintf("exited with code %d", *outcome.ExitCode)
		}
		return "exited"
	case ReasonMatch:
		return "output matched the pattern"
	case ReasonTimeout:
		return "timed out before the condition was met; the process may still be running"
	case ReasonStalled:
		return "output stream ended before the pattern matched"
	case ReasonTimerElapsed:
		return "timer elapsed"
	case ReasonTerminalFailed:
		return "terminal is no longer observable"
	default:
		return sanitizeLine(outcome.Reason)
	}
}

func nextAction(mon Monitor, outcome Outcome) string {
	switch {
	case mon.Kind == KindTimer:
		return "Resume the work you scheduled this wake for. Treat the description as your own earlier note."
	case outcome.Reason == ReasonTimeout && mon.TerminalID != "":
		return "Inspect the terminal with compozy__terminal_read, then decide whether to register another " +
			"monitor with compozy__monitor_create or stop the process with compozy__terminal_close."
	case mon.TerminalID != "":
		return "Continue the task that registered this monitor. Read more output with compozy__terminal_read " +
			"if the tail above is not enough, and report the outcome to the user."
	default:
		return "Continue the task that registered this monitor."
	}
}

// escapeOutput entity-encodes markup so untrusted output can never close the
// <output> block or forge daemon-authored notification fields.
var escapeOutput = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// tail keeps the last maxOutputBytes of valid UTF-8 output, entity-encoded.
func tail(output string) string {
	output = strings.TrimSpace(strings.ToValidUTF8(output, "�"))
	if len(output) <= maxOutputBytes {
		return escapeOutput.Replace(output)
	}
	cut := len(output) - maxOutputBytes
	for cut < len(output) && !utf8.RuneStart(output[cut]) {
		cut++
	}
	return "[... earlier output truncated ...]\n" + escapeOutput.Replace(output[cut:])
}

func sanitizeLine(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.ReplaceAll(value, "<", "‹")
	return strings.ReplaceAll(value, ">", "›")
}
