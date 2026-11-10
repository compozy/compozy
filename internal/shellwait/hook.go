package shellwait

import (
	"encoding/json"
	"io"
	"strings"
)

// HookCommandName is the hidden CLI verb a provider PreToolUse hook runs.
const HookCommandName = "shell-wait-hook"

const subagentWaitDenyReason = "CompozyOS monitors wake the parent session, not a Claude provider-native subagent. " +
	"Do not register a monitor or end your turn expecting a monitor to resume you. " +
	"Return the pending work and any terminal_id to your parent so it can register the monitor and resume you, " +
	"or use a separate CompozyOS child session for work that needs its own wake."

const powerShellToolName = "PowerShell"

type hookInput struct {
	AgentID   string `json:"agent_id"`
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

// RunHook emits Claude deny decisions for shell waits and subagent monitors; malformed input fails open.
func RunHook(stdin io.Reader, stdout io.Writer) {
	var input hookInput
	if err := json.NewDecoder(stdin).Decode(&input); err != nil {
		return
	}
	reason := ""
	if input.ToolName == "compozy__monitor_create" || strings.HasSuffix(input.ToolName, "__compozy__monitor_create") {
		if strings.TrimSpace(input.AgentID) == "" {
			return
		}
		reason = subagentWaitDenyReason
	} else {
		finding, found := detectForTool(input)
		if !found {
			return
		}
		reason = DenyReason(finding)
		if strings.TrimSpace(input.AgentID) != "" {
			reason = subagentWaitDenyReason
		}
	}
	decision := map[string]hookDecision{"hookSpecificOutput": {
		HookEventName:            "PreToolUse",
		PermissionDecision:       "deny",
		PermissionDecisionReason: reason,
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
