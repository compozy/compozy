package settings

import (
	"maps"
	"reflect"

	aghconfig "github.com/compozy/agh/internal/config"
)

const windowManagerConfigRoot = "window_manager"

func buildWindowManagerSection(cfg *aghconfig.Config) WindowManagerSection {
	return WindowManagerSection{Config: cloneWindowManagerConfig(cfg.WindowManager)}
}

func diffWindowManagerSettings(
	current aghconfig.WindowManagerConfig,
	desired aghconfig.WindowManagerConfig,
) []string {
	changed := make([]string, 0, 23)
	appendChange := func(path string, differs bool) {
		if differs {
			changed = append(changed, path)
		}
	}

	appendChange("window_manager.new_window_policy", current.NewWindowPolicy != desired.NewWindowPolicy)
	appendChange(
		"window_manager.small_viewport_policy",
		current.SmallViewportPolicy != desired.SmallViewportPolicy,
	)
	appendChange("window_manager.focus_policy", current.FocusPolicy != desired.FocusPolicy)
	appendChange("window_manager.focus_wrap", current.FocusWrap != desired.FocusWrap)
	appendChange(
		"window_manager.focus_follows_pointer",
		current.FocusFollowsPointer != desired.FocusFollowsPointer,
	)
	appendChange("window_manager.raise_on_focus", current.RaiseOnFocus != desired.RaiseOnFocus)
	appendChange("window_manager.drag_away_policy", current.DragAwayPolicy != desired.DragAwayPolicy)
	appendChange(
		"window_manager.group_move_modifier",
		current.GroupMoveModifier != desired.GroupMoveModifier,
	)
	appendChange("window_manager.swap_modifier", current.SwapModifier != desired.SwapModifier)
	appendChange("window_manager.history_limit", current.HistoryLimit != desired.HistoryLimit)
	appendChange(
		"window_manager.desktop_transition",
		current.DesktopTransition != desired.DesktopTransition,
	)
	appendChange("window_manager.gaps.inner", current.Gaps.Inner != desired.Gaps.Inner)
	appendChange("window_manager.gaps.top", current.Gaps.Top != desired.Gaps.Top)
	appendChange("window_manager.gaps.right", current.Gaps.Right != desired.Gaps.Right)
	appendChange("window_manager.gaps.bottom", current.Gaps.Bottom != desired.Gaps.Bottom)
	appendChange("window_manager.gaps.left", current.Gaps.Left != desired.Gaps.Left)
	appendChange("window_manager.snap.edge_band", current.Snap.EdgeBand != desired.Snap.EdgeBand)
	appendChange("window_manager.snap.corner_reach", current.Snap.CornerReach != desired.Snap.CornerReach)
	appendChange("window_manager.snap.exit_slack", current.Snap.ExitSlack != desired.Snap.ExitSlack)
	appendChange(
		"window_manager.snap.repeat_ratios",
		!reflect.DeepEqual(current.Snap.RepeatRatios, desired.Snap.RepeatRatios),
	)
	appendChange(
		"window_manager.bindings.top_center",
		current.Bindings.TopCenter != desired.Bindings.TopCenter,
	)
	appendChange(
		"window_manager.bindings.bottom_center",
		current.Bindings.BottomCenter != desired.Bindings.BottomCenter,
	)
	appendChange(
		"window_manager.shortcuts",
		!reflect.DeepEqual(current.Shortcuts, desired.Shortcuts),
	)
	return changed
}

func applyWindowManagerSettings(
	editor *aghconfig.OverlayEditor,
	settings aghconfig.WindowManagerConfig,
) error {
	root := func(path ...string) []string {
		return append([]string{windowManagerConfigRoot}, path...)
	}
	updates := []struct {
		path  []string
		value any
	}{
		{path: root("new_window_policy"), value: settings.NewWindowPolicy},
		{path: root("small_viewport_policy"), value: settings.SmallViewportPolicy},
		{path: root("focus_policy"), value: settings.FocusPolicy},
		{path: root("focus_wrap"), value: settings.FocusWrap},
		{path: root("focus_follows_pointer"), value: settings.FocusFollowsPointer},
		{path: root("raise_on_focus"), value: settings.RaiseOnFocus},
		{path: root("drag_away_policy"), value: settings.DragAwayPolicy},
		{path: root("group_move_modifier"), value: settings.GroupMoveModifier},
		{path: root("swap_modifier"), value: settings.SwapModifier},
		{path: root("history_limit"), value: settings.HistoryLimit},
		{path: root("desktop_transition"), value: settings.DesktopTransition},
		{path: root("gaps", "inner"), value: settings.Gaps.Inner},
		{path: root("gaps", "top"), value: settings.Gaps.Top},
		{path: root("gaps", "right"), value: settings.Gaps.Right},
		{path: root("gaps", "bottom"), value: settings.Gaps.Bottom},
		{path: root("gaps", "left"), value: settings.Gaps.Left},
		{path: root("snap", "edge_band"), value: settings.Snap.EdgeBand},
		{path: root("snap", "corner_reach"), value: settings.Snap.CornerReach},
		{path: root("snap", "exit_slack"), value: settings.Snap.ExitSlack},
		{path: root("snap", "repeat_ratios"), value: append([]float64(nil), settings.Snap.RepeatRatios...)},
		{path: root("bindings", "top_center"), value: settings.Bindings.TopCenter},
		{path: root("bindings", "bottom_center"), value: settings.Bindings.BottomCenter},
	}
	if err := applyValueUpdates(editor, updates); err != nil {
		return err
	}

	shortcutsPath := root("shortcuts")
	if len(settings.Shortcuts) == 0 {
		return editor.Delete(shortcutsPath)
	}
	values := make(map[string]any, len(settings.Shortcuts))
	for command, chord := range settings.Shortcuts {
		values[command] = chord
	}
	return editor.SetTable(shortcutsPath, values)
}

func cloneWindowManagerConfig(cfg aghconfig.WindowManagerConfig) aghconfig.WindowManagerConfig {
	cfg.Snap.RepeatRatios = append([]float64(nil), cfg.Snap.RepeatRatios...)
	cfg.Shortcuts = maps.Clone(cfg.Shortcuts)
	return cfg
}
