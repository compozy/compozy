package shellwait

import (
	"encoding/json"
	"fmt"
)

const (
	redirectLead = "CompozyOS refused this shell wait: it keeps the turn busy and Compozy cannot supervise it. "
	redirectTail = " Then keep working on steps that do not depend on it, or end your turn if nothing else remains: " +
		"Compozy wakes this session when the monitor fires."
	terminalMonitorInput = `{"kind":"terminal","terminal_id":"<returned id>","until":"exit",` +
		`"description":"<what you are waiting for>"}`
)

// DenyReason names the exact daemon-owned replacement for a detected wait.
func DenyReason(finding Finding) string {
	switch finding.Kind {
	case KindSleep:
		return redirectLead + fmt.Sprintf(
			`Call compozy__monitor_create {"kind":"timer","delay_seconds":%d,"description":"<why you are waiting>"}, `+
				"and run the next step after the wake. Delays under %d seconds are allowed.",
			finding.DelaySeconds, MinDelaySeconds,
		) + redirectTail
	case KindPoll:
		return redirectLead + terminalRedirect(pollInvocation(finding)) + redirectTail
	default:
		return redirectLead + terminalRedirect(finding.Argv) + redirectTail
	}
}

func pollInvocation(finding Finding) []string {
	if finding.Shell == shellPowerShell {
		return []string{shellPowerShell, "-Command", finding.Source}
	}
	return []string{shellPOSIX, "-c", finding.Source}
}

func terminalRedirect(argv []string) string {
	if len(argv) == 0 {
		return "Run the command with compozy__terminal_exec."
	}
	input, err := json.Marshal(map[string]any{"command": argv[0], "args": append([]string{}, argv[1:]...)})
	if err != nil {
		return "Run the command with compozy__terminal_exec."
	}
	return "Run it with compozy__terminal_exec " + string(input) +
		"; if the result reports still_running, call compozy__monitor_create " + terminalMonitorInput + "."
}

// DetectToolInput reads a provider-native shell tool input. It accepts the
// command string of a Bash tool, the argv array Codex reports, and the
// command-plus-args shape of an ACP terminal/create request.
func DetectToolInput(raw json.RawMessage) (Finding, bool) {
	var input struct {
		Command json.RawMessage `json:"command"`
		Args    []string        `json:"args"`
	}
	if err := json.Unmarshal(raw, &input); err != nil || len(input.Command) == 0 {
		return Finding{}, false
	}
	var argv []string
	if err := json.Unmarshal(input.Command, &argv); err == nil {
		return detectCall(argv, 0)
	}
	var command string
	if err := json.Unmarshal(input.Command, &command); err != nil {
		return Finding{}, false
	}
	if len(input.Args) == 0 {
		return Detect(command)
	}
	return detectCall(append([]string{command}, input.Args...), 0)
}
