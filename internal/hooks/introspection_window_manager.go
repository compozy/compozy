package hooks

const introspectionWindowManagerObservationPatchValue = "WindowManagerObservationPatch"

func windowManagerHookEventDescriptors() map[HookEvent]EventDescriptor {
	return map[HookEvent]EventDescriptor{
		HookWindowManagerTabActivated: {Event: HookWindowManagerTabActivated, Family: HookEventFamilyWindowManager, SyncEligible: false, PayloadSchema: "WindowManagerTabActivatedPayload", PatchSchema: introspectionWindowManagerObservationPatchValue},
		HookWindowManagerTabUnsplit:   {Event: HookWindowManagerTabUnsplit, Family: HookEventFamilyWindowManager, SyncEligible: false, PayloadSchema: "WindowManagerTabUnsplitPayload", PatchSchema: introspectionWindowManagerObservationPatchValue},
		HookWindowManagerTabSplit:     {Event: HookWindowManagerTabSplit, Family: HookEventFamilyWindowManager, SyncEligible: false, PayloadSchema: "WindowManagerTabSplitPayload", PatchSchema: introspectionWindowManagerObservationPatchValue},
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
		HookWindowManagerWindowOpened: {
			Event:         HookWindowManagerWindowOpened,
			Family:        HookEventFamilyWindowManager,
			SyncEligible:  false,
			PayloadSchema: "WindowManagerWindowOpenedPayload",
			PatchSchema:   introspectionWindowManagerObservationPatchValue,
		},
		HookWindowManagerWindowClosed: {
			Event:         HookWindowManagerWindowClosed,
			Family:        HookEventFamilyWindowManager,
			SyncEligible:  false,
			PayloadSchema: "WindowManagerWindowClosedPayload",
			PatchSchema:   introspectionWindowManagerObservationPatchValue,
		},
		HookWindowManagerStackGrouped: {
			Event:         HookWindowManagerStackGrouped,
			Family:        HookEventFamilyWindowManager,
			SyncEligible:  false,
			PayloadSchema: "WindowManagerStackGroupedPayload",
			PatchSchema:   introspectionWindowManagerObservationPatchValue,
		},
		HookWindowManagerStackUngrouped: {
			Event:         HookWindowManagerStackUngrouped,
			Family:        HookEventFamilyWindowManager,
			SyncEligible:  false,
			PayloadSchema: "WindowManagerStackUngroupedPayload",
			PatchSchema:   introspectionWindowManagerObservationPatchValue,
		},
		HookWindowManagerStackActivated: {
			Event:         HookWindowManagerStackActivated,
			Family:        HookEventFamilyWindowManager,
			SyncEligible:  false,
			PayloadSchema: "WindowManagerStackActivatedPayload",
			PatchSchema:   introspectionWindowManagerObservationPatchValue,
		},
	}
}
