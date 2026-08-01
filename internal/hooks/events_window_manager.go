package hooks

const (
	HookWindowManagerLayoutApplied  HookEvent = "window_manager.layout.applied"
	HookWindowManagerDesktopCreated HookEvent = "window_manager.desktop.created"
	HookWindowManagerDesktopDeleted HookEvent = "window_manager.desktop.deleted"
	HookWindowManagerWindowMoved    HookEvent = "window_manager.window.moved"
	HookWindowManagerWindowOpened   HookEvent = "window_manager.window.opened"
	HookWindowManagerWindowClosed   HookEvent = "window_manager.window.closed"
	HookWindowManagerStackGrouped   HookEvent = "window_manager.stack.grouped"
	HookWindowManagerStackUngrouped HookEvent = "window_manager.stack.ungrouped"
	HookWindowManagerStackActivated HookEvent = "window_manager.stack.activated"
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
		HookWindowManagerWindowOpened: {
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		HookWindowManagerWindowClosed: {
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		HookWindowManagerStackGrouped: {
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		HookWindowManagerStackUngrouped: {
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		HookWindowManagerStackActivated: {
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
		HookWindowManagerWindowOpened,
		HookWindowManagerWindowClosed,
		HookWindowManagerStackGrouped,
		HookWindowManagerStackUngrouped,
		HookWindowManagerStackActivated,
	}
}
