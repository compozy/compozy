package windowmanager

var commandIDs = [...]CommandID{
	CommandDesktopCreate,
	CommandDesktopUpdate,
	CommandDesktopReorder,
	CommandDesktopSwitch,
	CommandDesktopDelete,
	CommandWindowOpen,
	CommandWindowNavigate,
	CommandWindowClose,
	CommandWindowFocus,
	CommandWindowMove,
	CommandWindowResize,
	CommandWindowSwap,
	CommandWindowToggleFloating,
	CommandWindowZoom,
	CommandWindowStackGroup,
	CommandWindowStackReorder,
	CommandWindowStackSetActive,
	CommandWindowPin,
	CommandWindowReopen,
	CommandLayoutArrange,
	CommandLayoutResize,
	CommandLayoutFrameResize,
	CommandLayoutBalance,
	CommandLayoutUndo,
	CommandLayoutRedo,
	CommandLayoutReplace,
}

// CommandIDs returns every daemon semantic window-manager action ID.
func CommandIDs() []CommandID {
	return append([]CommandID(nil), commandIDs[:]...)
}
