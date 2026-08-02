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

func windowManagerHookEventDefinitions() []hookEventDefinition {
	return []hookEventDefinition{
		{event: HookWindowManagerLayoutApplied,
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		{event: HookWindowManagerDesktopCreated,
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		{event: HookWindowManagerDesktopDeleted,
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		{event: HookWindowManagerWindowMoved,
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		{event: HookWindowManagerWindowOpened,
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		{event: HookWindowManagerWindowClosed,
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		{event: HookWindowManagerStackGrouped,
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		{event: HookWindowManagerStackUngrouped,
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
		{event: HookWindowManagerStackActivated,
			family:       HookEventFamilyWindowManager,
			syncEligible: false,
		},
	}
}
