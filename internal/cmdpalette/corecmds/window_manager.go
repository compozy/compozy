package corecmds

import (
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/windowmanager"
)

var windowManagerTitles = map[windowmanager.CommandID]string{
	windowmanager.CommandDesktopCreate:        "Create desktop",
	windowmanager.CommandDesktopUpdate:        "Update desktop",
	windowmanager.CommandDesktopReorder:       "Reorder desktop",
	windowmanager.CommandDesktopSwitch:        "Switch desktop",
	windowmanager.CommandDesktopDelete:        "Delete desktop",
	windowmanager.CommandWindowOpen:           "Open window",
	windowmanager.CommandWindowNavigate:       "Navigate window",
	windowmanager.CommandWindowClose:          "Close window",
	windowmanager.CommandWindowFocus:          "Focus window",
	windowmanager.CommandWindowMove:           "Move window",
	windowmanager.CommandWindowResize:         "Resize window",
	windowmanager.CommandWindowSwap:           "Swap windows",
	windowmanager.CommandWindowToggleFloating: "Toggle floating window",
	windowmanager.CommandWindowZoom:           "Zoom window",
	windowmanager.CommandWindowStackGroup:     "Group windows",
	windowmanager.CommandWindowStackReorder:   "Reorder window stack",
	windowmanager.CommandWindowStackSetActive: "Activate stacked window",
	windowmanager.CommandWindowPin:            "Pin window",
	windowmanager.CommandWindowReopen:         "Reopen window",
	windowmanager.CommandLayoutArrange:        "Arrange layout",
	windowmanager.CommandLayoutResize:         "Resize layout",
	windowmanager.CommandLayoutFrameResize:    "Resize layout frame",
	windowmanager.CommandLayoutBalance:        "Balance layout",
	windowmanager.CommandLayoutUndo:           "Undo layout",
	windowmanager.CommandLayoutRedo:           "Redo layout",
	windowmanager.CommandLayoutReplace:        "Replace layout",
}

func windowManagerCommands() []cmdpalette.Descriptor {
	shellIDs := make(map[cmdpalette.CommandID]struct{})
	for _, command := range shellCommands() {
		shellIDs[command.ID] = struct{}{}
	}
	commands := make([]cmdpalette.Descriptor, 0, len(windowmanager.CommandIDs()))
	for _, windowManagerID := range windowmanager.CommandIDs() {
		id := cmdpalette.CommandID(windowManagerID)
		if _, exists := shellIDs[id]; exists {
			continue
		}
		section := "Window"
		icon := "panel-top"
		switch {
		case strings.HasPrefix(string(id), "desktop."):
			section, icon = "Desktops", "monitor"
		case strings.HasPrefix(string(id), "layout."):
			section, icon = "Layout", "layout-grid"
		}
		command := clientCommand(id, windowManagerTitles[windowManagerID], section, icon)
		command.When = clientContextRequirement(id)
		commands = append(commands, command)
	}
	return commands
}

func clientContextRequirement(id cmdpalette.CommandID) []cmdpalette.Predicate {
	if strings.HasPrefix(string(id), "window.") || strings.HasPrefix(string(id), "layout.") {
		return []cmdpalette.Predicate{{
			Key: cmdpalette.ContextShellDesktop, Value: true, Reason: "requires an attached shell",
		}}
	}
	return nil
}
