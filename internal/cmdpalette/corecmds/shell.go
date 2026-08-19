package corecmds

import (
	"fmt"

	"github.com/compozy/compozy/internal/cmdpalette"
)

type shellCommandDefinition struct {
	id         cmdpalette.CommandID
	title      string
	section    string
	icon       string
	needsFocus bool
	exempt     bool
	// when overrides the focused-window requirement for commands whose
	// relevance is a different piece of client context.
	when []cmdpalette.Predicate
}

var fixedShellCommands = []shellCommandDefinition{
	{id: "palette.open", title: "Command palette", section: "Shell", icon: "command", exempt: true},
	{id: "session.new", title: "New session", section: "Shell", icon: "square-terminal", exempt: true},
	{id: "scope.global.toggle", title: "Global scope", section: "Shell", icon: "globe", exempt: true},
	{id: "window.nav.back", title: "Back", section: "Shell", icon: "arrow-left", needsFocus: true, exempt: true},
	{id: "sidebar.toggle", title: "Toggle sidebar", section: "Shell", icon: "panel-left"},
	{id: "shell.sessions.toggle", title: "Toggle sessions", section: "Shell", icon: "list"},
	{id: "shortcuts.cheatsheet", title: "Keyboard shortcuts", section: "Shell", icon: "keyboard", exempt: true},
	{id: "window.close", title: "Close window", section: "Window", icon: "x-square", needsFocus: true},
	{id: "window.minimize", title: "Minimize window", section: "Window", icon: "minus-square", needsFocus: true},
	{id: "window.zoom", title: "Zoom window", section: "Window", icon: "maximize-2", needsFocus: true},
	{id: "window.toggle_floating", title: "Toggle floating", section: "Window", icon: "picture-in-picture", needsFocus: true},
	{id: "window.focus.left", title: "Focus left", section: "Window", icon: "arrow-left"},
	{id: "window.focus.right", title: "Focus right", section: "Window", icon: "arrow-right"},
	{id: "window.focus.up", title: "Focus up", section: "Window", icon: "arrow-up"},
	{id: "window.focus.down", title: "Focus down", section: "Window", icon: "arrow-down"},
	{id: "window.focus.last", title: "Focus last window", section: "Window", icon: "history"},
	{id: "window.merge_all", title: "Merge all windows", section: "Window", icon: "combine", when: []cmdpalette.Predicate{{
		Key:      cmdpalette.ContextDesktopWindowCount,
		Operator: cmdpalette.PredicateGreaterThanOrEqual,
		Value:    2,
		Reason:   "needs two windows on this desktop",
	}}},
	{id: "window.tab.new", title: "New tab", section: "Tabs", icon: "plus"},
	{id: "window.tab.detach", title: "Move tab to new window", section: "Tabs", icon: "panels-top-left", when: []cmdpalette.Predicate{{
		Key:    cmdpalette.ContextWindowStacked,
		Value:  true,
		Reason: "needs a tab in a stack",
	}}},
	{id: "window.tab.next", title: "Next tab", section: "Tabs", icon: "chevron-right", needsFocus: true},
	{id: "window.tab.previous", title: "Previous tab", section: "Tabs", icon: "chevron-left", needsFocus: true},
	{id: "window.tab.last", title: "Last tab", section: "Tabs", icon: "history", needsFocus: true},
	{id: "window.tab.reopen", title: "Reopen closed tab", section: "Tabs", icon: "rotate-ccw"},
	{id: "session.cycle.previous", title: "Previous session", section: "Sessions", icon: "chevron-left"},
	{id: "session.cycle.next", title: "Next session", section: "Sessions", icon: "chevron-right"},
	{id: "session.focus.attention", title: "Jump to attention", section: "Sessions", icon: "bell"},
	{id: "desktop.switch.previous", title: "Previous desktop", section: "Desktops", icon: "chevron-left"},
	{id: "desktop.switch.next", title: "Next desktop", section: "Desktops", icon: "chevron-right"},
	{id: "desktop.create", title: "Create desktop", section: "Desktops", icon: "plus"},
	{id: "desktop.overview", title: "Desktops overview", section: "Desktops", icon: "monitor"},
	{id: "workspace.picker", title: "Workspace picker", section: "Workspaces", icon: "folder"},
	{id: "workspace.cycle.previous", title: "Previous workspace", section: "Workspaces", icon: "chevron-left"},
	{id: "workspace.cycle.next", title: "Next workspace", section: "Workspaces", icon: "chevron-right"},
	{id: "layout.arrange.two-up", title: "Arrange left and right", section: "Layout", icon: "columns-2", needsFocus: true},
	{id: "layout.arrange.grid", title: "Arrange in grid", section: "Layout", icon: "layout-grid", needsFocus: true},
	{id: "layout.balance", title: "Balance layout", section: "Layout", icon: "scale", needsFocus: true},
	{id: "layout.undo", title: "Undo layout", section: "Layout", icon: "undo-2"},
	{id: "layout.redo", title: "Redo layout", section: "Layout", icon: "redo-2"},
}

func shellCommands() []cmdpalette.Descriptor {
	definitions := append([]shellCommandDefinition(nil), fixedShellCommands...)
	placements := []struct{ id, title string }{
		{"left", "Tile left half"}, {"right", "Tile right half"},
		{"top", "Tile top half"}, {"bottom", "Tile bottom half"},
		{"top-left", "Tile top left quarter"}, {"top-right", "Tile top right quarter"},
		{"bottom-left", "Tile bottom left quarter"}, {"bottom-right", "Tile bottom right quarter"},
	}
	for _, placement := range placements {
		definitions = append(definitions, shellCommandDefinition{
			id: cmdpalette.CommandID("window.tile." + placement.id), title: placement.title,
			section: "Tiling", icon: "panel-top", needsFocus: true,
		})
	}
	for slot := 1; slot <= 8; slot++ {
		definitions = append(definitions, shellCommandDefinition{
			id:    cmdpalette.CommandID(fmt.Sprintf("window.tab.jump.%d", slot)),
			title: fmt.Sprintf("Go to tab %d", slot), section: "Tabs", icon: "square-stack", needsFocus: true,
		})
	}
	for slot := 1; slot <= 9; slot++ {
		definitions = append(definitions, shellCommandDefinition{
			id:    cmdpalette.CommandID(fmt.Sprintf("desktop.switch.%d", slot)),
			title: fmt.Sprintf("Switch to desktop %d", slot), section: "Desktops", icon: "monitor",
		})
	}
	commands := make([]cmdpalette.Descriptor, 0, len(definitions))
	for _, definition := range definitions {
		command := clientCommand(definition.id, definition.title, definition.section, definition.icon)
		command.AvailabilityExempt = definition.exempt
		switch {
		case definition.when != nil:
			command.When = append([]cmdpalette.Predicate(nil), definition.when...)
		case definition.needsFocus:
			command.When = []cmdpalette.Predicate{{
				Key: cmdpalette.ContextWindowFocused, Value: true, Reason: "requires a focused window",
			}}
		}
		commands = append(commands, command)
	}
	return commands
}
