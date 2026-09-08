package hooks

const (
	HookTerminalOpened            HookEvent = "terminal.opened"
	HookTerminalClosed            HookEvent = "terminal.closed"
	HookTerminalCommandStarted    HookEvent = "terminal.command_started"
	HookTerminalCommandFinished   HookEvent = "terminal.command_finished"
	HookTerminalInputRequested    HookEvent = "terminal.input_requested"
	HookTerminalInputProvided     HookEvent = "terminal.input_provided"
	HookTerminalRecordingStarted  HookEvent = "terminal.recording_started"
	HookTerminalRecordingStopped  HookEvent = "terminal.recording_stopped"
	HookTerminalSubscriberEvicted HookEvent = "terminal.subscriber_evicted"
	HookTerminalLimitRejected     HookEvent = "terminal.limit_rejected"
)

func terminalHookEventDefinitions() []hookEventDefinition {
	return []hookEventDefinition{
		{event: HookTerminalOpened, family: HookEventFamilyTerminal, syncEligible: false},
		{event: HookTerminalClosed, family: HookEventFamilyTerminal, syncEligible: false},
		{event: HookTerminalCommandStarted, family: HookEventFamilyTerminal, syncEligible: false},
		{event: HookTerminalCommandFinished, family: HookEventFamilyTerminal, syncEligible: false},
		{event: HookTerminalInputRequested, family: HookEventFamilyTerminal, syncEligible: false},
		{event: HookTerminalInputProvided, family: HookEventFamilyTerminal, syncEligible: false},
		{event: HookTerminalRecordingStarted, family: HookEventFamilyTerminal, syncEligible: false},
		{event: HookTerminalRecordingStopped, family: HookEventFamilyTerminal, syncEligible: false},
		{event: HookTerminalSubscriberEvicted, family: HookEventFamilyTerminal, syncEligible: false},
		{event: HookTerminalLimitRejected, family: HookEventFamilyTerminal, syncEligible: false},
	}
}
