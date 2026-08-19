package corecmds

import (
	"context"
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
		provider, err := New()
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		commands, err := provider.ProvideCommands(context.Background(), "ws-test")
		if err != nil {
			t.Fatalf("ProvideCommands() error = %v", err)
		}
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

	t.Run("Should navigate to every settings route [UT-012]", func(t *testing.T) {
		t.Parallel()
		provider, err := New()
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		commands, err := provider.ProvideCommands(context.Background(), "ws-test")
		if err != nil {
			t.Fatalf("ProvideCommands() error = %v", err)
		}
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
			"settings.network=/settings/network",
			"settings.observability=/settings/observability",
			"settings.providers=/settings/providers",
			"settings.roles=/settings/roles",
			"settings.skills=/settings/skills",
		}
		sort.Strings(actual)
		if !slices.Equal(actual, expected) {
			t.Fatalf("settings destinations = %#v, want %#v", actual, expected)
		}
	})
}
