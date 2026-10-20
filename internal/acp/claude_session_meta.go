package acp

import "github.com/compozy/compozy/internal/providerexec"

// claudeDaemonOwnedBackgroundTools are Claude Code tools whose completion wakes
// the model through a provider-internal turn that no ACP prompt owns. CompozyOS
// drops that turn, so the model would promise a report that never arrives;
// compozy__monitor_create replaces them with a daemon-owned wake.
var claudeDaemonOwnedBackgroundTools = []string{"Monitor", "ScheduleWakeup", "CronCreate", "CronDelete", "CronList"}

// claudeDaemonOwnedBackgroundEnv disables Bash run_in_background and background
// Agent launches, which also finish through provider-internal turns.
var claudeDaemonOwnedBackgroundEnv = map[string]string{
	"CLAUDE_CODE_DISABLE_BACKGROUND_TASKS": "1",
	"CLAUDE_CODE_DISABLE_CRON":             "1",
}

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
			},
		},
	}
}
