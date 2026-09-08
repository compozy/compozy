package hooks

const terminalObservationPatchSchema = "TerminalObservationPatch"

func terminalHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{
		HookTerminalOpened:         terminalDescriptor(HookTerminalOpened, "TerminalOpenedPayload"),
		HookTerminalClosed:         terminalDescriptor(HookTerminalClosed, "TerminalClosedPayload"),
		HookTerminalCommandStarted: terminalDescriptor(HookTerminalCommandStarted, "TerminalCommandStartedPayload"),
		HookTerminalCommandFinished: terminalDescriptor(
			HookTerminalCommandFinished,
			"TerminalCommandFinishedPayload",
		),
		HookTerminalInputRequested: terminalDescriptor(HookTerminalInputRequested, "TerminalInputRequestedPayload"),
		HookTerminalInputProvided:  terminalDescriptor(HookTerminalInputProvided, "TerminalInputProvidedPayload"),
		HookTerminalRecordingStarted: terminalDescriptor(
			HookTerminalRecordingStarted,
			"TerminalRecordingStartedPayload",
		),
		HookTerminalRecordingStopped: terminalDescriptor(
			HookTerminalRecordingStopped,
			"TerminalRecordingStoppedPayload",
		),
		HookTerminalSubscriberEvicted: terminalDescriptor(
			HookTerminalSubscriberEvicted,
			"TerminalSubscriberEvictedPayload",
		),
		HookTerminalLimitRejected: terminalDescriptor(HookTerminalLimitRejected, "TerminalLimitRejectedPayload"),
	}
}

func terminalDescriptor(event HookEvent, payload string) EventDescriptor {
	return EventDescriptor{
		Event: event, Family: HookEventFamilyTerminal, SyncEligible: false,
		PayloadSchema: payload, PatchSchema: terminalObservationPatchSchema,
	}
}
