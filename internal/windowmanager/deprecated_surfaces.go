package windowmanager

import (
	"context"
	"errors"
)

const deprecationMoveToTab = `window.move with placement "tab"`

type DeprecationObserver func(context.Context, CommandRequest, []Deprecation)

func WithDeprecationObserver(observer DeprecationObserver) Option {
	return func(options *managerOptions) error {
		if observer == nil {
			return errors.New("window manager deprecation observer is required")
		}
		options.deprecationObserver = observer
		return nil
	}
}

// SurfaceDeprecation describes the v0.7 compatibility shim removed in v0.8.0.
func SurfaceDeprecation(surface string) Deprecation {
	type guidance struct{ replacement, message string }
	entries := map[string]guidance{
		"window.toggle_floating":                 {deprecationMoveToTab, "Floating windows were removed. Open or move a window to a tab instead."},
		"window.zoom":                            {deprecationMoveToTab, "Zoom was removed. Move the window to its own tab instead."},
		"window.resize":                          {"layout.resize", "Window frames were removed. Resize the seam between split members instead."},
		"layout.frame_resize":                    {"layout.resize", "Layout frames were removed. Resize the seam between split members instead."},
		"window.stack.group":                     {deprecationMoveToTab, "Stacks were removed. Each window becomes a tab next to the target instead."},
		"window.stack.reorder":                   {"window.tab.reorder", "Use window.tab.reorder to reorder the window's tab."},
		"window.stack.set_active":                {"window.tab.activate", "Use window.tab.activate to activate the window's tab."},
		"window.close.minimize":                  {"window.close", "Minimize was removed. The window remains open; close it when finished."},
		"window.open.window.floating":            {`window.open without floating`, "Floating windows were removed. The window opens as a tab instead."},
		"window.open.window.floating_rect":       {"window.open", "Floating rectangles were removed. The rectangle is ignored."},
		"window.open.restore_window_id":          {"window.tab.activate", "Minimize was removed. The existing window's tab is activated instead."},
		"window.move.placement=floating":         {deprecationMoveToTab, "Floating windows were removed. The window moves to a tab instead."},
		"window.move.floating_rect":              {deprecationMoveToTab, "Floating rectangles were removed. The rectangle is ignored."},
		"layout.arrange.arrangement=stack":       {deprecationMoveToTab, "Stacks were removed. Each window becomes an adjacent tab instead."},
		"layout.arrange.frame":                   {string(CommandLayoutArrange), "Layout frames were removed. The frame is ignored."},
		"layout.arrange.group_id":                {string(CommandLayoutArrange), "Layout groups were removed. The group ID is ignored."},
		"layout.arrange.keep_frames":             {string(CommandLayoutArrange), "Layout frames were removed. keep_frames is ignored."},
		"layout.balance.group_id":                {"layout.balance.tab_id", "Use tab_id to balance every split in the tab."},
		"compozy__window_list.include_minimized": {"compozy__window_list", "Minimize was removed. Every open window is listed."},
	}
	if RetiredShortcut(surface) {
		if _, exists := entries[surface]; !exists {
			entries[surface] = guidance{"remove the retired shortcut binding", "This shortcut is retired and does nothing."}
		}
		if surface == "window.tab.detach" {
			entries[surface] = guidance{"window.unsplit", "Stacks were removed. Use window.unsplit to move a split member to its own tab."}
		}
	}
	entry := entries[surface]
	return Deprecation{Code: "window_manager_surface_deprecated", Surface: surface,
		Replacement: entry.replacement, RemovalRelease: "v0.8.0", Message: entry.message}
}
