package settings

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"sort"

	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
)

const windowManagerConfigRoot = "window_manager"

func (s *service) buildWindowManagerSection(
	ctx context.Context,
	cfg *compozyconfig.Config,
	workspaceID string,
) (WindowManagerSection, error) {
	section := WindowManagerSection{
		Config:            cloneWindowManagerConfig(cfg.WindowManager),
		Aliases:           cloneAliases(cfg.CmdPalette.Aliases),
		ExtensionDefaults: []WindowManagerExtensionDefault{},
	}
	if s.cmdPalette != nil && workspaceID != "" {
		catalog, err := s.cmdPalette.Catalog(ctx, cmdpalette.WorkspaceID(workspaceID), "")
		if err != nil {
			return WindowManagerSection{}, fmt.Errorf("settings: load command catalog for shortcuts: %w", err)
		}
		section.Commands = make([]WindowManagerShortcutCommand, 0, len(catalog.Commands))
		section.EffectiveShortcuts = make(map[string]windowmanager.ShortcutBinding, len(catalog.Commands))
		section.Aliases = make(map[string]string)
		bindableIDs := make(windowmanager.BindableIDs, len(catalog.Commands))
		for _, command := range catalog.Commands {
			commandID := string(command.ID)
			bindableIDs[commandID] = struct{}{}
			section.Commands = append(section.Commands, WindowManagerShortcutCommand{
				ID:      commandID,
				Title:   command.Title,
				Section: command.Section,
				Source:  command.Source.ID(),
			})
			section.EffectiveShortcuts[commandID] = append(
				windowmanager.ShortcutBinding(nil), command.Bindings...,
			)
			if command.Alias != nil {
				section.Aliases[commandID] = *command.Alias
			}
		}
		_, diagnostics, err := windowmanager.TolerantEffectiveKeymap(
			cfg.WindowManager.Shortcuts,
			bindableIDs,
		)
		if err != nil {
			return WindowManagerSection{}, fmt.Errorf("settings: diagnose stored shortcuts: %w", err)
		}
		section.Diagnostics = diagnostics
		return section, nil
	}

	bindableIDs := windowmanager.DefaultBindableIDs()
	effective, diagnostics, err := windowmanager.TolerantEffectiveKeymap(
		cfg.WindowManager.Shortcuts,
		bindableIDs,
	)
	if err != nil {
		return WindowManagerSection{}, fmt.Errorf("settings: resolve stored shortcuts: %w", err)
	}
	section.EffectiveShortcuts = effective
	section.Diagnostics = diagnostics
	ids := make([]string, 0, len(effective))
	for commandID := range effective {
		ids = append(ids, commandID)
	}
	for commandID := range section.Aliases {
		if _, exists := bindableIDs[commandID]; !exists {
			delete(section.Aliases, commandID)
		}
	}
	sort.Strings(ids)
	section.Commands = make([]WindowManagerShortcutCommand, 0, len(ids))
	for _, commandID := range ids {
		section.Commands = append(section.Commands, WindowManagerShortcutCommand{
			ID: commandID, Title: commandID, Source: string(cmdpalette.SourceKindCore),
		})
	}
	return section, nil
}

func diffWindowManagerSettings(
	current compozyconfig.WindowManagerConfig,
	currentAliases map[string]string,
	desired compozyconfig.WindowManagerConfig,
	desiredAliases map[string]string,
) []string {
	changed := make([]string, 0, 26)
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
	appendChange("window_manager.nav_stack_limit", current.NavStackLimit != desired.NavStackLimit)
	appendChange("window_manager.closed_entry_limit", current.ClosedEntryLimit != desired.ClosedEntryLimit)
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
	appendChange("cmd_palette.aliases", !reflect.DeepEqual(currentAliases, desiredAliases))
	return changed
}

func applyWindowManagerSettings(
	editor *compozyconfig.OverlayEditor,
	settings compozyconfig.WindowManagerConfig,
	aliases map[string]string,
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
		{path: root("nav_stack_limit"), value: settings.NavStackLimit},
		{path: root("closed_entry_limit"), value: settings.ClosedEntryLimit},
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
		if err := editor.Delete(shortcutsPath); err != nil {
			return err
		}
	} else {
		values := make(map[string]any, len(settings.Shortcuts))
		for command, binding := range settings.Shortcuts {
			values[command] = append([]string(nil), binding...)
		}
		if err := editor.SetTable(shortcutsPath, values); err != nil {
			return err
		}
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

func cloneWindowManagerConfig(cfg compozyconfig.WindowManagerConfig) compozyconfig.WindowManagerConfig {
	cfg.Snap.RepeatRatios = append([]float64(nil), cfg.Snap.RepeatRatios...)
	cfg.Shortcuts = windowmanager.CloneShortcutMap(cfg.Shortcuts)
	return cfg
}

func cloneAliases(source map[string]string) map[string]string {
	aliases := make(map[string]string, len(source))
	maps.Copy(aliases, source)
	return aliases
}
