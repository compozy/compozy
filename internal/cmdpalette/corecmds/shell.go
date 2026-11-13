package corecmds

import (
	"fmt"

	"github.com/compozy/compozy/internal/cmdpalette"
)

const requiresFocusedWindowReason = "requires a focused window"

type shellCommandDefinition struct {
	id                 cmdpalette.CommandID
	title              string
	section            string
	icon               string
	needsFocus         bool
	availabilityExempt bool
	// when overrides the focused-window requirement for commands whose
	// relevance is a different piece of client context.
	when []cmdpalette.Predicate
}

// arrangeWhen gates layout presets on a focused window with at least one
// visible peer on its desktop: with a single window they have nothing to lay
// out, so they read disabled with a reason instead of doing nothing.
var arrangeWhen = []cmdpalette.Predicate{
	{Key: cmdpalette.ContextWindowFocused, Value: true, Reason: requiresFocusedWindowReason},
	{
		Key:      cmdpalette.ContextDesktopWindowCount,
		Operator: cmdpalette.PredicateGreaterThanOrEqual,
		Value:    2,
		Reason:   "needs two windows on this desktop",
	},
}

var splitWindowWhen = []cmdpalette.Predicate{
	{Key: cmdpalette.ContextWindowFocused, Value: true, Reason: requiresFocusedWindowReason},
	{Key: cmdpalette.ContextWindowStacked, Value: true, Reason: "needs a window in a split"},
}

var fixedShellCommands = []shellCommandDefinition{
	{
		id: "processes.left_running", title: "Processes left running",
		section: coreSectionSessions, icon: "cpu", availabilityExempt: true,
	},
	{
		id:                 "palette.open",
		title:              "Command palette",
		section:            coreSectionShell,
		icon:               coreIconCommand,
		availabilityExempt: true,
	},
	{
		id: "palette.summon.global", title: "Command palette · global",
		section: coreSectionShell, icon: coreIconCommand, availabilityExempt: true,
	},
	{
		id:                 "session.new",
		title:              "New session",
		section:            coreSectionShell,
		icon:               coreIconTerminal,
		availabilityExempt: true,
	},
	{
		id:                 "scope.global.toggle",
		title:              "All projects",
		section:            coreSectionShell,
		icon:               coreIconGlobe,
		availabilityExempt: true,
	},
	{
		id:                 "window.nav.back",
		title:              "Back",
		section:            coreSectionShell,
		icon:               coreIconArrowLeft,
		needsFocus:         true,
		availabilityExempt: true,
	},
	{id: "sidebar.toggle", title: "Toggle sidebar", section: coreSectionShell, icon: "panel-left"},
	{id: "shell.sessions.toggle", title: "Toggle sessions", section: coreSectionShell, icon: "list"},
	{
		id:                 "shortcuts.cheatsheet",
		title:              "Keyboard shortcuts",
		section:            coreSectionShell,
		icon:               "keyboard",
		availabilityExempt: true,
	},
	{id: "window.close", title: "Close window", section: coreSectionWindow, icon: "x-square", needsFocus: true},
	{id: "window.focus.left", title: "Focus left", section: coreSectionWindow, icon: coreIconArrowLeft},
	{id: "window.focus.right", title: "Focus right", section: coreSectionWindow, icon: "arrow-right"},
	{id: "window.focus.up", title: "Focus up", section: coreSectionWindow, icon: "arrow-up"},
	{id: "window.focus.down", title: "Focus down", section: coreSectionWindow, icon: "arrow-down"},
	{id: "window.focus.last", title: "Focus last window", section: coreSectionWindow, icon: "history"},
	{id: "window.tab.new", title: "New tab", section: coreSectionTabs, icon: coreIconPlus},
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
	{id: "desktop.overview", title: "All desktops", section: coreSectionDesktops, icon: coreIconMonitor},
	{id: "workspace.picker", title: "Workspace picker", section: coreSectionWorkspaces, icon: "folder"},
	{
		id:      "workspace.cycle.previous",
		title:   "Previous workspace",
		section: coreSectionWorkspaces,
		icon:    coreIconChevronLeft,
	},
	{id: "workspace.cycle.next", title: "Next workspace", section: coreSectionWorkspaces, icon: coreIconChevronRight},
	{
		id:      "layout.arrange.two-up",
		title:   "Arrange left and right",
		section: coreSectionLayout,
		icon:    "columns-2",
		when:    arrangeWhen,
	},
	{
		id:      "layout.arrange.main-stack",
		title:   "Main and stack",
		section: coreSectionLayout,
		icon:    "layout-panel-left",
		when:    arrangeWhen,
	},
	{
		id:      "layout.arrange.columns",
		title:   "Columns",
		section: coreSectionLayout,
		icon:    "columns-3",
		when:    arrangeWhen,
	},
	{
		id:      "layout.arrange.grid",
		title:   "Grid",
		section: coreSectionLayout,
		icon:    "layout-grid",
		when:    arrangeWhen,
	},
	{id: "layout.balance", title: "Balance sizes", section: coreSectionLayout, icon: "scale", when: arrangeWhen},
	{id: "layout.undo", title: "Undo layout", section: coreSectionLayout, icon: "undo-2"},
	{id: "layout.redo", title: "Redo layout", section: coreSectionLayout, icon: "redo-2"},
}

func shellCommands() []cmdpalette.Descriptor {
	definitions := append([]shellCommandDefinition(nil), fixedShellCommands...)
	definitions = append(
		definitions,
		shellCommandDefinition{
			id:      "window.unsplit",
			title:   "Move to new tab",
			section: coreSectionWindow,
			icon:    "panels-top-left",
			when:    splitWindowWhen,
		},
		shellCommandDefinition{
			id:         "window.tab.move_left",
			title:      "Move tab left",
			section:    coreSectionTabs,
			icon:       coreIconArrowLeft,
			needsFocus: true,
		},
		shellCommandDefinition{
			id:         "window.tab.move_right",
			title:      "Move tab right",
			section:    coreSectionTabs,
			icon:       "arrow-right",
			needsFocus: true,
		},
	)
	placements := []struct{ id, title string }{
		{"left", "Move to tab's left edge"}, {"right", "Move to tab's right edge"},
		{"top", "Move to tab's top edge"}, {"bottom", "Move to tab's bottom edge"},
	}
	for _, placement := range placements {
		definitions = append(definitions, shellCommandDefinition{
			id: cmdpalette.CommandID("window.tile." + placement.id), title: placement.title,
			section: coreSectionTiling, icon: coreIconPanelTop, when: splitWindowWhen,
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
	for slot := 1; slot <= 9; slot++ {
		definitions = append(definitions, shellCommandDefinition{
			id:    cmdpalette.CommandID(fmt.Sprintf("window.move_to_desktop.%d", slot)),
			title: fmt.Sprintf("Move window to desktop %d", slot), section: coreSectionDesktops,
			icon: coreIconMonitor, needsFocus: true,
		})
	}
	commands := make([]cmdpalette.Descriptor, 0, len(definitions))
	for _, definition := range definitions {
		command := clientCommand(definition.id, definition.title, definition.section, definition.icon)
		if definition.id == "palette.summon.global" {
			command.Action.Op = "palette.open"
		}
		command.AvailabilityExempt = definition.availabilityExempt
		switch {
		case definition.when != nil:
			command.When = append([]cmdpalette.Predicate(nil), definition.when...)
		case definition.needsFocus:
			command.When = []cmdpalette.Predicate{{
				Key: cmdpalette.ContextWindowFocused, Value: true, Reason: requiresFocusedWindowReason,
			}}
		}
		commands = append(commands, command)
	}
	return commands
}
