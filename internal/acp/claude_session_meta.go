package acp

import (
	"maps"
	"runtime"

	"github.com/compozy/compozy/internal/providerexec"
)

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

// claudeBackgroundShellDenyReason is the guidance a refused background shell returns.
const claudeBackgroundShellDenyReason = "CompozyOS cannot wake you when a background shell command exits. " +
	"Run it with compozy__terminal_exec, register compozy__monitor_create on the returned terminal_id, " +
	"then end your turn."

const claudeBackgroundShellDenyOutput = `{"hookSpecificOutput":{"hookEventName":"PreToolUse",` +
	`"permissionDecision":"deny","permissionDecisionReason":"` + claudeBackgroundShellDenyReason + `"}}`

const claudeHookTypeCommand = "command"

// claudeBackgroundShellMatcher covers both shell tools that accept run_in_background.
const claudeBackgroundShellMatcher = "Bash|PowerShell"

// claudeBackgroundShellDenyHook denies run_in_background on a shell tool, whose
// exit would also wake the model outside any prompt, and points at the
// daemon-owned path. It is a settings hook because Claude Code has no switch for
// background shells alone; the input is the PreToolUse JSON on stdin. Windows
// gets a PowerShell hook so it does not depend on Git Bash.
func claudeBackgroundShellDenyHook(goos string) map[string]any {
	if goos == "windows" {
		return map[string]any{
			"type":  claudeHookTypeCommand,
			"shell": "powershell",
			"command": `$hookInput = [Console]::In.ReadToEnd(); ` +
				`if ($hookInput -match '"run_in_background"\s*:\s*true') { ` +
				`Write-Output '` + claudeBackgroundShellDenyOutput + `' }; exit 0`,
		}
	}
	return map[string]any{
		"type": claudeHookTypeCommand,
		"command": `grep -Eq '"run_in_background"[[:space:]]*:[[:space:]]*true' && ` +
			`printf '%s\n' '` + claudeBackgroundShellDenyOutput + `'; exit 0`,
	}
}

// claudeCodeSessionMeta returns the claude-agent-acp `_meta.claudeCode` for
// session/new, session/load, and session/resume, or nil for every other
// provider command. Besides the options, it asks the adapter to forward only
// the four SDK task lifecycle messages that settle native Agent/Task rows.
func claudeCodeSessionMeta(opts StartOpts) any {
	if !providerexec.IsClaudeAgentBridge(opts.Command) {
		return nil
	}
	env := make(map[string]string, len(claudeDaemonOwnedBackgroundEnv))
	maps.Copy(env, claudeDaemonOwnedBackgroundEnv)
	return map[string]any{
		"claudeCode": map[string]any{
			"emitRawSDKMessages": claudeTaskRawSDKFilters(),
			"options": map[string]any{
				"disallowedTools": append([]string(nil), claudeDaemonOwnedBackgroundTools...),
				"env":             env,
				"settings": map[string]any{
					"hooks": map[string]any{
						"PreToolUse": []any{map[string]any{
							"matcher": claudeBackgroundShellMatcher,
							"hooks":   []any{claudeBackgroundShellDenyHook(runtime.GOOS)},
						}},
					},
				},
			},
		},
	}
}
