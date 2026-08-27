package calls

import (
	"fmt"
	"strings"
)

// CallPromptWithRemainingDepth projects the bound-call duty and remaining depth into a child prompt.
func CallPromptWithRemainingDepth(callID, prompt string, remaining int) string {
	if remaining < 0 {
		remaining = 0
	}
	depthContext := "You cannot delegate further."
	if remaining == 1 {
		depthContext = "You may delegate 1 more level."
	} else if remaining > 1 {
		depthContext = fmt.Sprintf("You may delegate %d more levels.", remaining)
	}
	return fmt.Sprintf(
		"Bound call: %s\n"+
			"Duty: compozy__call_return is your terminal act. agent_call, mailbox messages, and ordinary prose do not "+
			"settle this call; use agent_call only for further delegation; a truly empty omitted return settles "+
			"completed-without-result.\n"+
			"Delegation: %s\n\nAssignment:\n%s",
		strings.TrimSpace(callID),
		depthContext,
		strings.TrimSpace(prompt),
	)
}
