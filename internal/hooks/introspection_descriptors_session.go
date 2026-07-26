package hooks

func sessionHookEventDescriptors() map[HookEvent]EventDescriptor {
	return mergeHookEventDescriptors(
		sessionFamilyHookEventDescriptors(),
		sandboxFamilyHookEventDescriptors(),
		inputFamilyHookEventDescriptors(),
		promptFamilyHookEventDescriptors(),
		eventFamilyHookEventDescriptors(),
	)
}
func sessionFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookSessionPreCreate: {
		Event:         HookSessionPreCreate,
		Family:        HookEventFamilySession,
		SyncEligible:  true,
		PayloadSchema: "SessionPreCreatePayload",
		PatchSchema:   "SessionCreatePatch",
	},
		HookSessionPostCreate: {
			Event:         HookSessionPostCreate,
			Family:        HookEventFamilySession,
			SyncEligible:  true,
			PayloadSchema: "SessionPostCreatePayload",
			PatchSchema:   "SessionPostCreatePatch",
		},
		HookSessionPreResume: {
			Event:         HookSessionPreResume,
			Family:        HookEventFamilySession,
			SyncEligible:  true,
			PayloadSchema: "SessionPreResumePayload",
			PatchSchema:   "SessionPreResumePatch",
		},
		HookSessionPostResume: {
			Event:         HookSessionPostResume,
			Family:        HookEventFamilySession,
			SyncEligible:  true,
			PayloadSchema: "SessionPostResumePayload",
			PatchSchema:   "SessionPostResumePatch",
		},
		HookSessionPreStop: {
			Event:         HookSessionPreStop,
			Family:        HookEventFamilySession,
			SyncEligible:  true,
			PayloadSchema: "SessionPreStopPayload",
			PatchSchema:   "SessionPreStopPatch",
		},
		HookSessionPostStop: {
			Event:         HookSessionPostStop,
			Family:        HookEventFamilySession,
			SyncEligible:  true,
			PayloadSchema: "SessionPostStopPayload",
			PatchSchema:   "SessionPostStopPatch",
		},
		HookSessionMessagePersisted: {
			Event:         HookSessionMessagePersisted,
			Family:        HookEventFamilySession,
			SyncEligible:  false,
			PayloadSchema: "SessionMessagePersistedPayload",
			PatchSchema:   introspectionAuthoredContextObservationPatchValue,
		},

		HookSessionHealthUpdateAfter: {
			Event:         HookSessionHealthUpdateAfter,
			Family:        HookEventFamilySession,
			SyncEligible:  false,
			PayloadSchema: "SessionHealthUpdateAfterPayload",
			PatchSchema:   introspectionAuthoredContextObservationPatchValue,
		}}
}
func sandboxFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookSandboxPrepare: {
		Event:         HookSandboxPrepare,
		Family:        HookEventFamilySandbox,
		SyncEligible:  true,
		PayloadSchema: "SandboxPreparePayload",
		PatchSchema:   "SandboxPreparePatch",
	},
		HookSandboxReady: {
			Event:         HookSandboxReady,
			Family:        HookEventFamilySandbox,
			SyncEligible:  false,
			PayloadSchema: "SandboxReadyPayload",
			PatchSchema:   "SandboxReadyPatch",
		},
		HookSandboxSyncBefore: {
			Event:         HookSandboxSyncBefore,
			Family:        HookEventFamilySandbox,
			SyncEligible:  true,
			PayloadSchema: "SandboxSyncBeforePayload",
			PatchSchema:   "SandboxSyncBeforePatch",
		},
		HookSandboxSyncAfter: {
			Event:         HookSandboxSyncAfter,
			Family:        HookEventFamilySandbox,
			SyncEligible:  false,
			PayloadSchema: "SandboxSyncAfterPayload",
			PatchSchema:   "SandboxSyncAfterPatch",
		},
		HookSandboxStop: {
			Event:         HookSandboxStop,
			Family:        HookEventFamilySandbox,
			SyncEligible:  true,
			PayloadSchema: "SandboxStopPayload",
			PatchSchema:   "SandboxStopPatch",
		}}
}
func inputFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookInputPreSubmit: {
		Event:         HookInputPreSubmit,
		Family:        HookEventFamilyInput,
		SyncEligible:  true,
		PayloadSchema: "InputPreSubmitPayload",
		PatchSchema:   "InputPreSubmitPatch",
	}}
}
func promptFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookPromptPostAssemble: {
		Event:         HookPromptPostAssemble,
		Family:        HookEventFamilyPrompt,
		SyncEligible:  true,
		PayloadSchema: "PromptPayload",
		PatchSchema:   "PromptPatch",
	}}
}
func eventFamilyHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{HookEventPreRecord: {
		Event:         HookEventPreRecord,
		Family:        HookEventFamilyEvent,
		SyncEligible:  false,
		PayloadSchema: "EventPreRecordPayload",
		PatchSchema:   "EventPreRecordPatch",
	},
		HookEventPostRecord: {
			Event:         HookEventPostRecord,
			Family:        HookEventFamilyEvent,
			SyncEligible:  false,
			PayloadSchema: "EventPostRecordPayload",
			PatchSchema:   "EventPostRecordPatch",
		}}
}
