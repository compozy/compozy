package hooks

const (
	HookWindowManagerTabSplit       HookEvent = "window_manager.tab.split"
	HookWindowManagerTabUnsplit     HookEvent = "window_manager.tab.unsplit"
	HookWindowManagerTabActivated   HookEvent = "window_manager.tab.activated"
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
		{event: HookWindowManagerTabSplit, family: HookEventFamilyWindowManager, syncEligible: false},
		{event: HookWindowManagerTabUnsplit, family: HookEventFamilyWindowManager, syncEligible: false},
		{event: HookWindowManagerTabActivated, family: HookEventFamilyWindowManager, syncEligible: false},
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

// RetiredWindowManagerEventWarning is removed with the retired events in v0.8.0.
func RetiredWindowManagerEventWarning(event HookEvent) string {
	switch event {
	case HookWindowManagerStackGrouped, HookWindowManagerStackUngrouped:
		return string(event) + " is retired and never fires; use window_manager.tab.split or window_manager.tab.unsplit for split transitions. It will be deleted in v0.8.0."
	case HookWindowManagerStackActivated:
		return "window_manager.stack.activated is deprecated; use window_manager.tab.activated. It will be deleted in v0.8.0."
	default:
		return ""
	}
}
