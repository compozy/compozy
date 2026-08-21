package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func commandListDescriptor() toolspkg.Descriptor {
	return nativeDescriptor(
		toolspkg.ToolIDCommandList,
		"command_list",
		"Command List",
		"List daemon, ACP, and effective skill commands available to one session.",
		sessionIDInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, sessionsCommandsKey, sessionsListKey},
		[]string{"session commands", "slash commands", "available skills"},
	)
}
