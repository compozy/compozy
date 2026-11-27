package acp

import (
	"testing"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

func TestMatchSpeedConfigAcceptsSpeedCategory(t *testing.T) {
	t.Parallel()

	options := []SessionConfigOption{{
		ID:             "fast-mode",
		Label:          "Fast mode",
		Category:       "speed",
		Kind:           SessionConfigOptionKindSelect,
		CurrentValueID: "off",
		Values:         []SessionConfigOptionValue{{Value: "off", Label: "Off"}, {Value: "on", Label: "On"}},
	}}

	for requested, want := range map[speedpkg.Speed]string{
		speedpkg.SpeedFast:   "on",
		speedpkg.SpeedNormal: "off",
	} {
		t.Run("Should select "+want+" for "+string(requested), func(t *testing.T) {
			t.Parallel()

			match, reason := matchSpeedConfig(requested, options)
			if match == nil {
				t.Fatalf("matchSpeedConfig(%q) found no option, reason %q", requested, reason)
			}
			if match.selection.ValueID != want {
				t.Errorf("matchSpeedConfig(%q) selected %q, want %q", requested, match.selection.ValueID, want)
			}
		})
	}
}
