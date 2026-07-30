package daemon

import (
	"fmt"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func taskClaimNativeInstruction(runID string, capabilities []string) string {
	instruction := fmt.Sprintf(
		"Call the hosted native tool `%s` with `run_id` set to %q",
		toolspkg.ToolIDTaskRunClaimNext,
		strings.TrimSpace(runID),
	)
	if required := uniqueNonEmptyStrings(capabilities); len(required) > 0 {
		instruction += fmt.Sprintf(" and `required_capabilities` set to %q", required)
	}
	return instruction + " before doing any work. Do not use the CLI for session-bound lease operations."
}

func taskLeaseNativeInstruction() string {
	return fmt.Sprintf(
		"Maintain and settle the lease from this same session with `%s`, `%s`, `%s`, or `%s`.",
		toolspkg.ToolIDTaskRunHeartbeat,
		toolspkg.ToolIDTaskRunComplete,
		toolspkg.ToolIDTaskRunFail,
		toolspkg.ToolIDTaskRunRelease,
	)
}
