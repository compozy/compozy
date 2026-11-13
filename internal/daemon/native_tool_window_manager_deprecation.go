package daemon

import (
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

// Native tool names and flattened fields stay accepted until v0.8.0.
func windowManagerToolDeprecations(
	toolID toolspkg.ToolID,
	entries []windowmanager.Deprecation,
) []windowmanager.Deprecation {
	if toolID == toolspkg.ToolIDLayoutPreview {
		return entries
	}
	commands := map[string]string{
		"window.toggle_floating": "compozy__window_float", "window.zoom": "compozy__window_zoom",
		"window.resize": "compozy__window_resize", "window.stack.group": "compozy__window_group",
		"window.stack.reorder": "window.stack.reorder", "window.stack.set_active": "window.stack.set_active",
		"layout.frame_resize": "compozy__layout_frame_resize",
	}
	replace := strings.NewReplacer(
		"window.tab.reorder",
		"compozy__window_reorder",
		"window.tab.activate",
		"compozy__window_activate",
		"window.move",
		"compozy__window_move",
		"window.open",
		"compozy__window_open",
		"window.close",
		"compozy__window_close",
		"layout.resize",
		"compozy__layout_resize",
		"layout.arrange",
		"compozy__layout_arrange",
		"layout.balance.tab_id",
		"compozy__layout_balance with tab_id",
	)
	for i := range entries {
		entry := &entries[i]
		if surface, ok := commands[entry.Surface]; ok {
			entry.Surface = surface
		} else {
			field := entry.Surface
			for _, prefix := range []string{"window.open.window.", "window.open.", "window.close.", "window.move.", "layout.arrange.", "layout.balance."} {
				if suffix, ok := strings.CutPrefix(field, prefix); ok {
					field = suffix
					break
				}
			}
			entry.Surface = toolID.String() + "." + field
		}
		entry.Replacement = replace.Replace(entry.Replacement)
	}
	return entries
}
