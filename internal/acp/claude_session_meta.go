package acp

import "github.com/compozy/compozy/internal/providerexec"

// claudeDaemonOwnedBackgroundTools are Claude Code tools whose completion wakes
// the model through a provider-internal turn that no ACP prompt owns. CompozyOS
// drops that turn, so the model would promise a report that never arrives;
// compozy__monitor_create replaces them with a daemon-owned wake.
var claudeDaemonOwnedBackgroundTools = []string{"Monitor", "ScheduleWakeup", "CronCreate", "CronDelete", "CronList"}

// claudeDaemonOwnedBackgroundEnv disables the remaining out-of-prompt wake
// sources: scheduled prompts and MCP tool calls moved to background tasks.
// Background Agent subagents stay enabled: claude-agent-acp holds the prompt
// open until they settle, so their results reach the owning turn.
var claudeDaemonOwnedBackgroundEnv = map[string]string{
	"CLAUDE_CODE_DISABLE_CRON":                "1",
	"CLAUDE_CODE_DISABLE_MCP_TASK_BACKGROUND": "1",
}

// claudeBackgroundBashDenyHook denies Bash run_in_background, whose exit would
// also wake the model outside any prompt, and points at the daemon-owned path.
// It is a settings hook because Claude Code has no switch for background shells
// alone; the input is the PreToolUse JSON on stdin.
const claudeBackgroundBashDenyHook = `grep -Eq '"run_in_background"[[:space:]]*:[[:space:]]*true' && ` +
	`printf '%s\n' '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny",` +
	`"permissionDecisionReason":"CompozyOS cannot wake you when a background Bash command exits. ` +
	`Run it with compozy__terminal_exec, register compozy__monitor_create on the returned terminal_id, ` +
	`then end your turn."}}'; exit 0`

// claudeCodeSessionMeta returns the claude-agent-acp `_meta.claudeCode.options`
// for session/new and session/load, or nil for every other provider command.
func claudeCodeSessionMeta(opts StartOpts) any {
	if !providerexec.IsClaudeAgentBridge(opts.Command) {
		return nil
	}
	env := make(map[string]string, len(claudeDaemonOwnedBackgroundEnv))
	for key, value := range claudeDaemonOwnedBackgroundEnv {
		env[key] = value
	}
	return map[string]any{
		"claudeCode": map[string]any{
			"options": map[string]any{
				"disallowedTools": append([]string(nil), claudeDaemonOwnedBackgroundTools...),
				"env":             env,
				"settings": map[string]any{
					"hooks": map[string]any{
						"PreToolUse": []any{map[string]any{
							"matcher": "Bash",
							"hooks": []any{map[string]any{
								"type": "command", "command": claudeBackgroundBashDenyHook,
							}},
						}},
					},
				},
			},
		},
	}
}
