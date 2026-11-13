package settings

import (
	"reflect"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// windowManagerWriteBase is what a window-manager write is measured against. A user-scope write
// has no inherited layer and stores the whole section. A project-scope write stores only its
// delta: scalar and gap keys that differ from what the project inherits without its own layer,
// and the keymap, global shortcuts and aliases only when the request changes them.
type windowManagerWriteBase struct {
	// inherited is the configuration the project sees without its own layer; nil for user scope.
	inherited *compozyconfig.Config
	// current is the effective configuration before the write.
	current compozyconfig.Config
}

type windowManagerValueUpdate = struct {
	path  []string
	value any
}

func windowManagerScalarUpdates(settings compozyconfig.WindowManagerConfig) []windowManagerValueUpdate {
	root := func(path ...string) []string {
		return append([]string{windowManagerConfigRoot}, path...)
	}
	return []windowManagerValueUpdate{
		{path: root("new_window_policy"), value: settings.NewWindowPolicy},
		{path: root("small_viewport_policy"), value: settings.SmallViewportPolicy},
		{path: root("focus_policy"), value: settings.FocusPolicy},
		{path: root("focus_wrap"), value: settings.FocusWrap},
		{path: root("focus_follows_pointer"), value: settings.FocusFollowsPointer},
		{path: root("swap_modifier"), value: settings.SwapModifier},
		{path: root("history_limit"), value: settings.HistoryLimit},
		{path: root("nav_stack_limit"), value: settings.NavStackLimit},
		{path: root("closed_entry_limit"), value: settings.ClosedEntryLimit},
		{path: root("desktop_transition"), value: settings.DesktopTransition},
		{path: root("gaps", "inner"), value: settings.Gaps.Inner},
		{path: root("gaps", "top"), value: settings.Gaps.Top},
		{path: root("gaps", "right"), value: settings.Gaps.Right},
		{path: root("gaps", "bottom"), value: settings.Gaps.Bottom},
		{path: root("gaps", "left"), value: settings.Gaps.Left},
	}
}

func applyWindowManagerSettings(
	editor *compozyconfig.OverlayEditor,
	settings compozyconfig.WindowManagerConfig,
	aliases map[string]string,
	base *windowManagerWriteBase,
) error {
	if err := applyWindowManagerScalars(editor, settings, base); err != nil {
		return err
	}
	project := base.inherited != nil
	current := base.current.WindowManager
	if !project || !sameWindowManagerMap(current.Shortcuts, settings.Shortcuts) {
		if err := writeWindowManagerShortcuts(editor, settings); err != nil {
			return err
		}
	}
	if !project || !sameWindowManagerMap(current.GlobalShortcuts, settings.GlobalShortcuts) {
		globalValues := make(map[string]any, len(settings.GlobalShortcuts))
		for commandID, chord := range settings.GlobalShortcuts {
			globalValues[commandID] = chord
		}
		path := []string{windowManagerConfigRoot, "global_shortcuts"}
		if err := editor.SetTable(path, globalValues); err != nil {
			return err
		}
	}
	if project && sameWindowManagerMap(base.current.CmdPalette.Aliases, aliases) {
		return nil
	}
	aliasesPath := []string{cmdPaletteConfigRoot, "aliases"}
	if len(aliases) == 0 {
		return editor.Delete(aliasesPath)
	}
	values := make(map[string]any, len(aliases))
	for commandID, alias := range aliases {
		values[commandID] = alias
	}
	return editor.SetTable(aliasesPath, values)
}

// sameWindowManagerMap treats an absent and an empty map as the same request.
func sameWindowManagerMap[K comparable, V any](current, desired map[K]V) bool {
	if len(current) == 0 && len(desired) == 0 {
		return true
	}
	return reflect.DeepEqual(current, desired)
}

// applyWindowManagerScalars writes every scalar and gap key at user scope. At project scope a
// value equal to the inherited one is removed from the file, so a later change to the inherited
// layer reaches the project; a differing value is written.
func applyWindowManagerScalars(
	editor *compozyconfig.OverlayEditor,
	settings compozyconfig.WindowManagerConfig,
	base *windowManagerWriteBase,
) error {
	updates := windowManagerScalarUpdates(settings)
	if base.inherited == nil {
		return applyValueUpdates(editor, updates)
	}
	inherited := windowManagerScalarUpdates(base.inherited.WindowManager)
	for index, update := range updates {
		if update.value == inherited[index].value {
			if err := editor.DeleteKeyPruningEmptyTable(update.path); err != nil {
				return err
			}
			continue
		}
		if err := editor.SetValue(update.path, update.value); err != nil {
			return err
		}
	}
	return nil
}

func writeWindowManagerShortcuts(
	editor *compozyconfig.OverlayEditor,
	settings compozyconfig.WindowManagerConfig,
) error {
	path := []string{windowManagerConfigRoot, "shortcuts"}
	if len(settings.Shortcuts) == 0 {
		return editor.Delete(path)
	}
	values := make(map[string]any, len(settings.Shortcuts))
	for command, binding := range settings.Shortcuts {
		values[command] = append([]string(nil), binding...)
	}
	return editor.SetTable(path, values)
}
