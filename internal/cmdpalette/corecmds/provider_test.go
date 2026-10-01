package corecmds

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/windowmanager"
)

// Suite: core command absorption.
// Invariant: every daemon window action and every settings destination has exactly one canonical descriptor.
func TestProviderAbsorption(t *testing.T) {
	t.Parallel()

	t.Run("Should include every window-manager action exactly once [UT-002]", func(t *testing.T) {
		t.Parallel()
		commands := mustCommands(t)
		counts := make(map[cmdpalette.CommandID]int, len(commands))
		for _, command := range commands {
			counts[command.ID]++
		}
		for _, windowManagerID := range windowmanager.CommandIDs() {
			id := cmdpalette.CommandID(windowManagerID)
			if counts[id] != 1 {
				t.Errorf("command %q count = %d, want 1", id, counts[id])
			}
		}
	})

	t.Run("Should expose Terminal as its own navigable app", func(t *testing.T) {
		t.Parallel()
		commands := mustCommands(t)
		byID := make(map[cmdpalette.CommandID]cmdpalette.Descriptor, len(commands))
		for _, command := range commands {
			byID[command.ID] = command
		}
		terminal := byID["app.open.terminal"]
		if terminal.Title != "Open Terminal" || terminal.Action.Kind != cmdpalette.ActionKindNavigate ||
			terminal.Action.App != "terminal" || !slices.Contains(terminal.Keywords, "terminal") {
			t.Fatalf("app.open.terminal = %#v, want canonical Terminal navigation", terminal)
		}
		session := byID["app.open.session"]
		if slices.Contains(session.Keywords, "terminal") {
			t.Fatalf("app.open.session keywords = %#v, want no terminal alias", session.Keywords)
		}
	})

	t.Run("Should give every shell-only palette row a bindable id [UT-002]", func(t *testing.T) {
		t.Parallel()
		commands := mustCommands(t)
		byID := make(map[cmdpalette.CommandID]cmdpalette.Descriptor, len(commands))
		for _, command := range commands {
			byID[command.ID] = command
		}
		// ADR-001: rows the shipped palette rendered without an action id and
		// could never bind. They are client operations, so each carries the
		// verbatim reason its context predicate reports when unavailable.
		expected := map[cmdpalette.CommandID]string{
			"shell.sessions.toggle": "",
			"window.merge_all":      "needs two windows on this desktop",
			"window.tab.detach":     "needs a tab in a stack",
		}
		for id, reason := range expected {
			command, ok := byID[id]
			if !ok {
				t.Errorf("command %q missing from the core catalog", id)
				continue
			}
			if command.Action.Kind != cmdpalette.ActionKindClientOp || command.Action.Op != string(id) {
				t.Errorf("command %q action = %#v, want client_op %q", id, command.Action, id)
			}
			if reason == "" {
				if len(command.When) != 0 {
					t.Errorf("command %q when = %#v, want none", id, command.When)
				}
				continue
			}
			if len(command.When) != 1 || command.When[0].Reason != reason {
				t.Errorf("command %q when = %#v, want single predicate reason %q", id, command.When, reason)
			}
		}
	})

	t.Run("Should expose the arrange presets under the Window menu copy", func(t *testing.T) {
		t.Parallel()
		commands := mustCommands(t)
		byID := make(map[cmdpalette.CommandID]cmdpalette.Descriptor, len(commands))
		for _, command := range commands {
			byID[command.ID] = command
		}
		expected := map[cmdpalette.CommandID]struct{ title, icon string }{
			"layout.arrange.main-stack": {"Main and stack", "layout-panel-left"},
			"layout.arrange.columns":    {"Columns", "columns-3"},
			"layout.arrange.grid":       {"Grid", "layout-grid"},
			"layout.balance":            {"Balance sizes", "scale"},
		}
		for id, want := range expected {
			command, ok := byID[id]
			if !ok {
				t.Errorf("command %q missing from the core catalog", id)
				continue
			}
			if command.Title != want.title || command.Icon != want.icon {
				t.Errorf("command %q = %q/%q, want %q/%q", id, command.Title, command.Icon, want.title, want.icon)
			}
			if command.Action.Kind != cmdpalette.ActionKindClientOp || command.Action.Op != string(id) {
				t.Errorf("command %q action = %#v, want client_op %q", id, command.Action, id)
			}
			// A preset has nothing to lay out without a peer: it reads disabled
			// with the reason instead of running as a no-op.
			if len(command.When) != 2 || command.When[0].Key != cmdpalette.ContextWindowFocused ||
				command.When[1].Key != cmdpalette.ContextDesktopWindowCount ||
				command.When[1].Operator != cmdpalette.PredicateGreaterThanOrEqual ||
				command.When[1].Value != 2 || command.When[1].Reason != "needs two windows on this desktop" {
				t.Errorf("command %q when = %#v, want focused window + two windows on this desktop", id, command.When)
			}
		}
	})

	t.Run("Should give every desktop slot a focus-gated move-window command", func(t *testing.T) {
		t.Parallel()
		commands := mustCommands(t)
		byID := make(map[cmdpalette.CommandID]cmdpalette.Descriptor, len(commands))
		for _, command := range commands {
			byID[command.ID] = command
		}
		keymap := windowmanager.DefaultKeymap()
		for slot := 1; slot <= 9; slot++ {
			id := cmdpalette.CommandID(fmt.Sprintf("window.move_to_desktop.%d", slot))
			command, ok := byID[id]
			if !ok {
				t.Errorf("command %q missing from the core catalog", id)
				continue
			}
			if command.Title != fmt.Sprintf("Move window to desktop %d", slot) {
				t.Errorf("command %q title = %q", id, command.Title)
			}
			if command.Action.Kind != cmdpalette.ActionKindClientOp || command.Action.Op != string(id) {
				t.Errorf("command %q action = %#v, want client_op %q", id, command.Action, id)
			}
			if len(command.When) != 1 || command.When[0].Key != cmdpalette.ContextWindowFocused {
				t.Errorf("command %q when = %#v, want the focused-window requirement", id, command.When)
			}
			if binding, bindable := keymap[string(id)]; !bindable || len(binding) != 0 {
				t.Errorf("DefaultKeymap()[%q] = %v (bindable %t), want unbound", id, binding, bindable)
			}
		}
	})

	t.Run("Should navigate to every settings route [UT-012]", func(t *testing.T) {
		t.Parallel()
		commands := mustCommands(t)
		actual := make([]string, 0)
		for _, command := range commands {
			if command.Section != "Settings" {
				continue
			}
			pathname, ok := command.Action.Args["pathname"].(string)
			if !ok {
				t.Fatalf("settings command %q pathname = %#v, want string", command.ID, command.Action.Args["pathname"])
			}
			actual = append(actual, fmt.Sprintf("%s=%s", command.ID, pathname))
		}
		expected := []string{
			"settings.appearance=/settings/appearance",
			"settings.attention=/settings/attention",
			"settings.automation=/settings/automation",
			"settings.extensions=/settings/extensions",
			"settings.gateway=/settings/gateway",
			"settings.general=/settings/general",
			"settings.hooks=/settings/hooks",
			"settings.layouts=/settings/layouts",
			"settings.memory=/settings/memory",
			"settings.observability=/settings/observability",
			"settings.palette=/settings/palette",
			"settings.profiles=/settings/profiles",
			"settings.providers=/settings/providers",
			"settings.roles=/settings/roles",
			"settings.skills=/settings/skills",
		}
		sort.Strings(actual)
		if !slices.Equal(actual, expected) {
			t.Fatalf("settings destinations = %#v, want %#v", actual, expected)
		}
	})

	t.Run("Should expose every built-in palette view exactly once", func(t *testing.T) {
		t.Parallel()
		commands := mustCommands(t)
		want := []string{
			"agents", "extensions", "jobs", "knowledge", "loops", "marketplace",
			"profiles", "sessions", "tasks", "triggers", "vault", "worktrees",
		}
		actual := make([]string, 0, len(want))
		seen := make(map[string]int, len(want))
		for _, command := range commands {
			if command.Action.Kind != cmdpalette.ActionKindView {
				continue
			}
			actual = append(actual, command.Action.View)
			seen[command.Action.View]++
		}
		sort.Strings(actual)
		if !slices.Equal(actual, want) {
			t.Fatalf("view ids = %#v, want %#v", actual, want)
		}
		for _, viewID := range want {
			if seen[viewID] != 1 {
				t.Errorf("view %q count = %d, want 1", viewID, seen[viewID])
			}
		}
	})

	t.Run("Should expose stable profile actions with canonical handoffs [UT-095]", func(t *testing.T) {
		t.Parallel()
		commands := mustCommands(t)
		byID := make(map[cmdpalette.CommandID]cmdpalette.Descriptor, len(commands))
		for _, command := range commands {
			byID[command.ID] = command
		}
		expectedFlows := map[cmdpalette.CommandID]string{
			"profile.create":    "create",
			"profile.update":    "update",
			"profile.rename":    "rename",
			"profile.archive":   "archive",
			"profile.unarchive": "unarchive",
			"profile.delete":    "delete",
		}
		for _, id := range []cmdpalette.CommandID{
			"profile.use", "profile.create", "profile.update", "profile.rename",
			"profile.archive", "profile.unarchive", "profile.delete",
		} {
			command, found := byID[id]
			if !found {
				t.Errorf("profile command %q is missing", id)
				continue
			}
			if id == "profile.use" {
				if command.Action.Kind != cmdpalette.ActionKindClientOp || command.Action.Op != string(id) {
					t.Errorf("profile.use action = %#v, want canonical client operation", command.Action)
				}
				continue
			}
			if command.Action.Kind != cmdpalette.ActionKindNavigate ||
				command.Action.Args["pathname"] != "/settings/profiles" ||
				command.Action.Args["flow"] != expectedFlows[id] {
				t.Errorf("profile command %q action = %#v, want profiles flow handoff", id, command.Action)
			}
		}
		expectedArguments := map[cmdpalette.CommandID][]string{
			"profile.use":       {"profile"},
			"profile.create":    {"name"},
			"profile.update":    {"profile"},
			"profile.rename":    {"profile", "new_name"},
			"profile.archive":   {"profile"},
			"profile.unarchive": {"profile"},
			"profile.delete":    {"profile"},
		}
		for id, names := range expectedArguments {
			command := byID[id]
			if len(command.Arguments) != len(names) {
				t.Errorf("profile command %q arguments = %#v, want %v", id, command.Arguments, names)
				continue
			}
			for index, name := range names {
				argument := command.Arguments[index]
				if argument.Name != name || !argument.Required || argument.Type != cmdpalette.ArgumentTypeText {
					t.Errorf("profile command %q argument %d = %#v, want required text %q", id, index, argument, name)
				}
			}
		}
		deleteCommand := byID["profile.delete"]
		if !deleteCommand.Destructive || deleteCommand.Confirmation == nil ||
			deleteCommand.Confirmation.Title != "Delete profile?" ||
			deleteCommand.Confirmation.Confirm != "Delete" {
			t.Errorf("profile.delete = %#v, want canonical destructive confirmation", deleteCommand)
		}
	})
}

func mustCommands(t *testing.T) []cmdpalette.Descriptor {
	t.Helper()
	provider, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	commands, err := provider.ProvideCommands(t.Context(), cmdpalette.CatalogRequest{
		ProfileLens: cmdpalette.ScopedProfileLens(cmdpalette.DefaultProfileLensID, "default"),
		WorkspaceID: "ws-test",
	})
	if err != nil {
		t.Fatalf("ProvideCommands() error = %v", err)
	}
	return commands
}
