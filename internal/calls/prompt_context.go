package calls

import (
	"fmt"
	"strings"
)

func callPromptWithRemainingDepth(prompt string, remaining int) string {
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
		"Call context: %s\n\n%s",
		depthContext,
		strings.TrimSpace(prompt),
	)
}

// CallPromptWithRemainingDepth projects the literal remaining-depth context at the daemon boundary.
func CallPromptWithRemainingDepth(prompt string, remaining int) string {
	return callPromptWithRemainingDepth(prompt, remaining)
}
