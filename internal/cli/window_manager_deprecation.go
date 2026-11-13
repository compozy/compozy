package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

// Remove the v0.7 CLI warning translations in v0.8.0.
func warnWindowManagerDeprecations(cmd *cobra.Command, deprecations []windowmanager.Deprecation) {
	surfaces := map[string]string{
		"window.toggle_floating": "window float", "window.zoom": "window zoom",
		"window.resize": "window resize", "layout.frame_resize": "layout frame-resize",
		"window.stack.group": "window group", "window.stack.reorder": "window.stack.reorder",
		"window.stack.set_active":          "window.stack.set_active",
		"window.open.window.floating":      "window open --floating",
		"window.open.window.floating_rect": "window open --rect",
		"window.open.restore_window_id":    "window open --restore",
		"window.close.minimize":            "window close --minimize",
		"window.move.placement=floating":   "window move --placement floating",
		"window.move.floating_rect":        "window move --rect",
		"layout.arrange.arrangement=stack": "layout arrange --arrangement stack",
		"layout.arrange.frame":             "layout arrange --frame", "layout.arrange.group_id": "layout arrange --group",
		"layout.arrange.keep_frames": "layout arrange --keep-frames", "layout.balance.group_id": "layout balance --group",
	}
	seen := map[string]bool{}
	for _, deprecation := range deprecations {
		if seen[deprecation.Surface] {
			continue
		}
		seen[deprecation.Surface] = true
		if deprecation.Surface == "window.toggle_floating" {
			cmd.PrintErrln("warning: `window float` is deprecated and does nothing; floating windows were removed. It will be deleted in v0.8.0.")
			continue
		}
		surface := surfaces[deprecation.Surface]
		if surface == "" {
			surface = deprecation.Surface
		}
		replacement := strings.NewReplacer(
			"window.tab.reorder", "window reorder", "window.tab.activate", "window activate",
			"window.move", "window move", "window.open", "window open", "window.close", "window close",
			"layout.resize", "layout resize", "layout.arrange", "layout arrange", "layout.balance.tab_id", "layout balance --tab",
		).Replace(deprecation.Replacement)
		cmd.PrintErrln(fmt.Sprintf("warning: `%s` is deprecated; %s Use %s. It will be deleted in %s.",
			surface, deprecation.Message, replacement, deprecation.RemovalRelease))
	}
}
