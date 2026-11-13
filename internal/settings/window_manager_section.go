package settings

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"

	"github.com/compozy/compozy/internal/cmdpalette"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
)

const windowManagerConfigRoot = "window_manager"

func (s *service) buildWindowManagerSection(
	ctx context.Context,
	cfg *compozyconfig.Config,
	workspaceID string,
	clientID string,
	clientProfile cmdpalette.ProfileLens,
) (WindowManagerSection, error) {
	section := WindowManagerSection{
		Config:            cloneWindowManagerConfig(cfg.WindowManager),
		Aliases:           cloneAliases(cfg.CmdPalette.Aliases),
		ExtensionDefaults: []WindowManagerExtensionDefault{},
	}
	if s.cmdPalette != nil && workspaceID != "" {
		return s.buildCatalogWindowManagerSection(ctx, cfg, workspaceID, clientID, clientProfile, section)
	}
	return buildDefaultWindowManagerSection(cfg, section)
}

func (s *service) buildCatalogWindowManagerSection(
	ctx context.Context,
	cfg *compozyconfig.Config,
	workspaceID string,
	clientID string,
	clientProfile cmdpalette.ProfileLens,
	section WindowManagerSection,
) (WindowManagerSection, error) {
	if clientProfile.ID == "" {
		clientProfile = cmdpalette.ScopedProfileLens(cmdpalette.DefaultProfileLensID, "default")
	}
	request := cmdpalette.CatalogRequest{
		ProfileLens: clientProfile,
		WorkspaceID: cmdpalette.WorkspaceID(workspaceID),
		ClientID:    cmdpalette.ClientID(clientID),
	}
	catalog, err := s.cmdPalette.Catalog(ctx, request)
	if err != nil && clientID != "" && errors.Is(err, cmdpalette.ErrNoAttachedShell) {
		// A read can name a client that is not attached to this workspace yet
		// (registration racing a workspace switch, or a daemon restart). The
		// section is still answerable without that client's context; only its
		// global-shortcut statuses are missing until the next read.
		slog.Default().DebugContext(
			ctx, "settings: window-manager read names an unattached client; reading without it",
			"workspace_id", workspaceID, "client_id", clientID,
		)
		request.ClientID = ""
		catalog, err = s.cmdPalette.Catalog(ctx, request)
	}
	if err != nil {
		return WindowManagerSection{}, fmt.Errorf("settings: load command catalog for shortcuts: %w", err)
	}
	section.Commands = make([]WindowManagerShortcutCommand, 0, len(catalog.Commands))
	section.EffectiveShortcuts = make(map[string]windowmanager.ShortcutBinding, len(catalog.Commands))
	section.Aliases = make(map[string]string)
	catalogIDs := make([]string, 0, len(catalog.Commands))
	for _, command := range catalog.Commands {
		catalogIDs = append(catalogIDs, string(command.ID))
	}
	bindableIDs := catalogBindableIDs(catalogIDs)
	for _, command := range catalog.Commands {
		commandID := string(command.ID)
		section.Commands = append(section.Commands, WindowManagerShortcutCommand{
			ID: commandID, Title: command.Title, Section: command.Section, Source: command.Source.ID(),
		})
		section.EffectiveShortcuts[commandID] = append(
			windowmanager.ShortcutBinding(nil), command.Bindings...,
		)
		if command.Alias != nil {
			section.Aliases[commandID] = *command.Alias
		}
		if command.GlobalShortcut != nil {
			section.GlobalShortcuts = append(section.GlobalShortcuts, WindowManagerGlobalShortcut{
				CommandID: commandID, IntendedChord: command.GlobalShortcut.IntendedChord,
				ActiveChord: command.GlobalShortcut.ActiveChord, Status: command.GlobalShortcut.Status,
				Reason: command.GlobalShortcut.Reason, SettingsURL: command.GlobalShortcut.SettingsURL,
			})
		}
	}
	_, diagnostics, err := windowmanager.TolerantEffectiveKeymap(cfg.WindowManager.Shortcuts, bindableIDs)
	if err != nil {
		return WindowManagerSection{}, fmt.Errorf("settings: diagnose stored shortcuts: %w", err)
	}
	section.ExtensionDefaults, err = s.buildWindowManagerExtensionDefaults(
		ctx, request.ProfileLens, workspaceID, cfg.WindowManager.Shortcuts, bindableIDs,
	)
	if err != nil {
		return WindowManagerSection{}, err
	}
	section.Diagnostics = diagnostics
	return section, nil
}

func buildDefaultWindowManagerSection(
	cfg *compozyconfig.Config,
	section WindowManagerSection,
) (WindowManagerSection, error) {
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
	maps.DeleteFunc(section.Aliases, func(commandID, _ string) bool {
		_, exists := bindableIDs[commandID]
		return !exists
	})
	slices.Sort(ids)
	section.Commands = make([]WindowManagerShortcutCommand, 0, len(ids))
	for _, commandID := range ids {
		section.Commands = append(section.Commands, WindowManagerShortcutCommand{
			ID: commandID, Title: commandID, Source: string(cmdpalette.SourceKindCore),
		})
	}
	globalIDs := slices.Sorted(maps.Keys(cfg.WindowManager.GlobalShortcuts))
	for _, commandID := range globalIDs {
		section.GlobalShortcuts = append(section.GlobalShortcuts, WindowManagerGlobalShortcut{
			CommandID: commandID, IntendedChord: cfg.WindowManager.GlobalShortcuts[commandID],
		})
	}
	return section, nil
}

func (s *service) buildWindowManagerExtensionDefaults(
	ctx context.Context,
	profileLens cmdpalette.ProfileLens,
	workspaceID string,
	overrides map[string]windowmanager.ShortcutBinding,
	bindableIDs windowmanager.BindableIDs,
) ([]WindowManagerExtensionDefault, error) {
	provider, ok := s.cmdPalette.(cmdpalette.ExtensionDefaultCatalog)
	if !ok {
		return []WindowManagerExtensionDefault{}, nil
	}
	defaults, err := provider.ExtensionDefaults(
		ctx,
		profileLens,
		cmdpalette.WorkspaceID(workspaceID),
	)
	if err != nil {
		return nil, fmt.Errorf("settings: load extension shortcut defaults: %w", err)
	}
	claims := make([]windowmanager.ExtensionDefaultShortcut, 0, len(defaults))
	for _, item := range defaults {
		claims = append(claims, windowmanager.ExtensionDefaultShortcut{
			CommandID: string(item.CommandID), Chord: item.Chord, Source: item.Source, Active: item.Active,
		})
	}
	_, statuses, _, err := windowmanager.TolerantEffectiveKeymapWithExtensionDefaults(
		overrides, bindableIDs, claims,
	)
	if err != nil {
		return nil, fmt.Errorf("settings: resolve extension shortcut defaults: %w", err)
	}
	result := make([]WindowManagerExtensionDefault, 0, len(statuses))
	for _, status := range statuses {
		result = append(result, WindowManagerExtensionDefault{
			CommandID: status.CommandID, Binding: status.Binding,
			Dormant: status.Dormant, ConflictWith: status.ConflictWith,
		})
	}
	return result, nil
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
	appendChange(
		"window_manager.shortcuts",
		!reflect.DeepEqual(current.Shortcuts, desired.Shortcuts),
	)
	appendChange(
		"window_manager.global_shortcuts",
		!reflect.DeepEqual(current.GlobalShortcuts, desired.GlobalShortcuts),
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

	globalShortcutsPath := root("global_shortcuts")
	globalValues := make(map[string]any, len(settings.GlobalShortcuts))
	for commandID, chord := range settings.GlobalShortcuts {
		globalValues[commandID] = chord
	}
	if err := editor.SetTable(globalShortcutsPath, globalValues); err != nil {
		return err
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
	cfg.GlobalShortcuts = windowmanager.CloneGlobalShortcutMap(cfg.GlobalShortcuts)
	return cfg
}

func cloneAliases(source map[string]string) map[string]string {
	aliases := make(map[string]string, len(source))
	maps.Copy(aliases, source)
	return aliases
}
