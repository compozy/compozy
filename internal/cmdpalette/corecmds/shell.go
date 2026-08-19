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
	{id: "palette.open", title: "Command palette", section: coreSectionShell, icon: "command", exempt: true},
	{id: "session.new", title: "New session", section: coreSectionShell, icon: "square-terminal", exempt: true},
	{id: "scope.global.toggle", title: "Global scope", section: coreSectionShell, icon: "globe", exempt: true},
	{
		id:         "window.nav.back",
		title:      "Back",
		section:    coreSectionShell,
		icon:       "arrow-left",
		needsFocus: true,
		exempt:     true,
	},
	{id: "sidebar.toggle", title: "Toggle sidebar", section: coreSectionShell, icon: "panel-left"},
	{id: "shell.sessions.toggle", title: "Toggle sessions", section: coreSectionShell, icon: "list"},
	{
		id:      "shortcuts.cheatsheet",
		title:   "Keyboard shortcuts",
		section: coreSectionShell,
		icon:    "keyboard",
		exempt:  true,
	},
	{id: "window.close", title: "Close window", section: coreSectionWindow, icon: "x-square", needsFocus: true},
	{
		id:         "window.minimize",
		title:      "Minimize window",
		section:    coreSectionWindow,
		icon:       "minus-square",
		needsFocus: true,
	},
	{id: "window.zoom", title: "Zoom window", section: coreSectionWindow, icon: "maximize-2", needsFocus: true},
	{
		id:         "window.toggle_floating",
		title:      "Toggle floating",
		section:    coreSectionWindow,
		icon:       "picture-in-picture",
		needsFocus: true,
	},
	{id: "window.focus.left", title: "Focus left", section: coreSectionWindow, icon: "arrow-left"},
	{id: "window.focus.right", title: "Focus right", section: coreSectionWindow, icon: "arrow-right"},
	{id: "window.focus.up", title: "Focus up", section: coreSectionWindow, icon: "arrow-up"},
	{id: "window.focus.down", title: "Focus down", section: coreSectionWindow, icon: "arrow-down"},
	{id: "window.focus.last", title: "Focus last window", section: coreSectionWindow, icon: "history"},
	{
		id:      "window.merge_all",
		title:   "Merge all windows",
		section: coreSectionWindow,
		icon:    "combine",
		when: []cmdpalette.Predicate{{
			Key:      cmdpalette.ContextDesktopWindowCount,
			Operator: cmdpalette.PredicateGreaterThanOrEqual,
			Value:    2,
			Reason:   "needs two windows on this desktop",
		}},
	},
	{id: "window.tab.new", title: "New tab", section: coreSectionTabs, icon: coreIconPlus},
	{
		id:      "window.tab.detach",
		title:   "Move tab to new window",
		section: coreSectionTabs,
		icon:    "panels-top-left",
		when: []cmdpalette.Predicate{{
			Key:    cmdpalette.ContextWindowStacked,
			Value:  true,
			Reason: "needs a tab in a stack",
		}},
	},
	{id: "window.tab.next", title: "Next tab", section: coreSectionTabs, icon: coreIconChevronRight, needsFocus: true},
	{
		id:         "window.tab.previous",
		title:      "Previous tab",
		section:    coreSectionTabs,
		icon:       coreIconChevronLeft,
		needsFocus: true,
	},
	{id: "window.tab.last", title: "Last tab", section: coreSectionTabs, icon: "history", needsFocus: true},
	{id: "window.tab.reopen", title: "Reopen closed tab", section: coreSectionTabs, icon: "rotate-ccw"},
	{id: "session.cycle.previous", title: "Previous session", section: coreSectionSessions, icon: coreIconChevronLeft},
	{id: "session.cycle.next", title: "Next session", section: coreSectionSessions, icon: coreIconChevronRight},
	{id: "session.focus.attention", title: "Jump to attention", section: coreSectionSessions, icon: "bell"},
	{id: "desktop.switch.previous", title: "Previous desktop", section: coreSectionDesktops, icon: coreIconChevronLeft},
	{id: "desktop.switch.next", title: "Next desktop", section: coreSectionDesktops, icon: coreIconChevronRight},
	{id: "desktop.create", title: "Create desktop", section: coreSectionDesktops, icon: coreIconPlus},
	{id: "desktop.overview", title: "Desktops overview", section: coreSectionDesktops, icon: coreIconMonitor},
	{id: "workspace.picker", title: "Workspace picker", section: coreSectionWorkspaces, icon: "folder"},
	{
		id:      "workspace.cycle.previous",
		title:   "Previous workspace",
		section: coreSectionWorkspaces,
		icon:    coreIconChevronLeft,
	},
	{id: "workspace.cycle.next", title: "Next workspace", section: coreSectionWorkspaces, icon: coreIconChevronRight},
	{
		id:         "layout.arrange.two-up",
		title:      "Arrange left and right",
		section:    coreSectionLayout,
		icon:       "columns-2",
		needsFocus: true,
	},
	{
		id:         "layout.arrange.grid",
		title:      "Arrange in grid",
		section:    coreSectionLayout,
		icon:       "layout-grid",
		needsFocus: true,
	},
	{id: "layout.balance", title: "Balance layout", section: coreSectionLayout, icon: "scale", needsFocus: true},
	{id: "layout.undo", title: "Undo layout", section: coreSectionLayout, icon: "undo-2"},
	{id: "layout.redo", title: "Redo layout", section: coreSectionLayout, icon: "redo-2"},
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
			section: "Tiling", icon: coreIconPanelTop, needsFocus: true,
		})
	}
	for slot := 1; slot <= 8; slot++ {
		definitions = append(definitions, shellCommandDefinition{
			id:    cmdpalette.CommandID(fmt.Sprintf("window.tab.jump.%d", slot)),
			title: fmt.Sprintf("Go to tab %d", slot), section: coreSectionTabs, icon: "square-stack", needsFocus: true,
		})
	}
	for slot := 1; slot <= 9; slot++ {
		definitions = append(definitions, shellCommandDefinition{
			id:    cmdpalette.CommandID(fmt.Sprintf("desktop.switch.%d", slot)),
			title: fmt.Sprintf("Switch to desktop %d", slot), section: coreSectionDesktops, icon: coreIconMonitor,
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
