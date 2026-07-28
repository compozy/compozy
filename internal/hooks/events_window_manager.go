package hooks

const (
	HookWindowManagerLayoutApplied  HookEvent = "window_manager.layout.applied"
	HookWindowManagerDesktopCreated HookEvent = "window_manager.desktop.created"
	HookWindowManagerDesktopDeleted HookEvent = "window_manager.desktop.deleted"
	HookWindowManagerWindowMoved    HookEvent = "window_manager.window.moved"
)

func windowManagerHookEventSpecs() map[HookEvent]hookEventSpec {
	return map[HookEvent]hookEventSpec{
		HookWindowManagerLayoutApplied: {
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		HookWindowManagerDesktopCreated: {
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		HookWindowManagerDesktopDeleted: {
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		HookWindowManagerWindowMoved: {
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
	}
}

func windowManagerHookEvents() []HookEvent {
	return []HookEvent{
		HookWindowManagerLayoutApplied,
		HookWindowManagerDesktopCreated,
		HookWindowManagerDesktopDeleted,
		HookWindowManagerWindowMoved,
	}
}
