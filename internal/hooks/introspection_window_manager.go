package hooks

const introspectionWindowManagerObservationPatchValue = "WindowManagerObservationPatch"

func windowManagerHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{
		HookWindowManagerLayoutApplied: {
			Event:         HookWindowManagerLayoutApplied,
			Family:        HookEventFamilyWindowManager,
			SyncEligible:  false,
			PayloadSchema: "WindowManagerLayoutAppliedPayload",
			PatchSchema:   introspectionWindowManagerObservationPatchValue,
		},
		HookWindowManagerDesktopCreated: {
			Event:         HookWindowManagerDesktopCreated,
			Family:        HookEventFamilyWindowManager,
			SyncEligible:  false,
			PayloadSchema: "WindowManagerDesktopCreatedPayload",
			PatchSchema:   introspectionWindowManagerObservationPatchValue,
		},
		HookWindowManagerDesktopDeleted: {
			Event:         HookWindowManagerDesktopDeleted,
			Family:        HookEventFamilyWindowManager,
			SyncEligible:  false,
			PayloadSchema: "WindowManagerDesktopDeletedPayload",
			PatchSchema:   introspectionWindowManagerObservationPatchValue,
		},
		HookWindowManagerWindowMoved: {
			Event:         HookWindowManagerWindowMoved,
			Family:        HookEventFamilyWindowManager,
			SyncEligible:  false,
			PayloadSchema: "WindowManagerWindowMovedPayload",
			PatchSchema:   introspectionWindowManagerObservationPatchValue,
		},
	}
}
