package hooks

const introspectionWindowManagerObservationPatchValue = "WindowManagerObservationPatch"

func windowManagerHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{
		HookWindowManagerTabActivated: windowManagerDescriptor(
			HookWindowManagerTabActivated,
			"WindowManagerTabActivatedPayload",
		),
		HookWindowManagerTabUnsplit: windowManagerDescriptor(
			HookWindowManagerTabUnsplit,
			"WindowManagerTabUnsplitPayload",
		),
		HookWindowManagerTabSplit: windowManagerDescriptor(
			HookWindowManagerTabSplit,
			"WindowManagerTabSplitPayload",
		),
		HookWindowManagerLayoutApplied: windowManagerDescriptor(
			HookWindowManagerLayoutApplied,
			"WindowManagerLayoutAppliedPayload",
		),
		HookWindowManagerDesktopCreated: windowManagerDescriptor(
			HookWindowManagerDesktopCreated,
			"WindowManagerDesktopCreatedPayload",
		),
		HookWindowManagerDesktopDeleted: windowManagerDescriptor(
			HookWindowManagerDesktopDeleted,
			"WindowManagerDesktopDeletedPayload",
		),
		HookWindowManagerWindowMoved: windowManagerDescriptor(
			HookWindowManagerWindowMoved,
			"WindowManagerWindowMovedPayload",
		),
		HookWindowManagerWindowOpened: windowManagerDescriptor(
			HookWindowManagerWindowOpened,
			"WindowManagerWindowOpenedPayload",
		),
		HookWindowManagerWindowClosed: windowManagerDescriptor(
			HookWindowManagerWindowClosed,
			"WindowManagerWindowClosedPayload",
		),
		HookWindowManagerStackGrouped: windowManagerDescriptor(
			HookWindowManagerStackGrouped,
			"WindowManagerStackGroupedPayload",
		),
		HookWindowManagerStackUngrouped: windowManagerDescriptor(
			HookWindowManagerStackUngrouped,
			"WindowManagerStackUngroupedPayload",
		),
		HookWindowManagerStackActivated: windowManagerDescriptor(
			HookWindowManagerStackActivated,
			"WindowManagerStackActivatedPayload",
		),
	}
}

func windowManagerDescriptor(event HookEvent, payload string) EventDescriptor {
	return EventDescriptor{
		Event: event, Family: HookEventFamilyWindowManager, SyncEligible: false,
		PayloadSchema: payload, PatchSchema: introspectionWindowManagerObservationPatchValue,
	}
}
