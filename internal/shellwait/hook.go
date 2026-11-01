package shellwait

import (
	"encoding/json"
	"io"
)

// HookCommandName is the hidden CLI verb a provider PreToolUse hook runs.
const HookCommandName = "shell-wait-hook"

const powerShellToolName = "PowerShell"

type hookInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookDecision struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

// RunHook answers one PreToolUse hook call: it writes a deny decision for a
// shell wait and nothing otherwise. Every failure allows the command, because a
// broken guard must never stop a session's shell.
func RunHook(stdin io.Reader, stdout io.Writer) {
	var input hookInput
	if err := json.NewDecoder(stdin).Decode(&input); err != nil {
		return
	}
	finding, found := detectForTool(input)
	if !found {
		return
	}
	decision := map[string]hookDecision{"hookSpecificOutput": {
		HookEventName:            "PreToolUse",
		PermissionDecision:       "deny",
		PermissionDecisionReason: DenyReason(finding),
	}}
	if err := json.NewEncoder(stdout).Encode(decision); err != nil {
		return
	}
}

func detectForTool(input hookInput) (Finding, bool) {
	if input.ToolName == powerShellToolName {
		return DetectPowerShell(input.ToolInput.Command)
	}
	return Detect(input.ToolInput.Command)
}
