package core

import (
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/windowmanager"
)

func settingsWindowManagerSectionResponse(
	envelope settingspkg.SectionEnvelope,
) (any, error) {
	if envelope.WindowManager == nil {
		return nil, errors.New("settings window-manager section is required")
	}
	effective := envelope.WindowManager.EffectiveShortcuts
	diagnostics := envelope.WindowManager.Diagnostics
	if effective == nil {
		resolved, storedDiagnostics, err := windowmanager.TolerantEffectiveKeymap(
			envelope.WindowManager.Config.Shortcuts,
			windowmanager.DefaultBindableIDs(),
		)
		if err != nil {
			return nil, fmt.Errorf("settings window-manager shortcuts are invalid: %w", err)
		}
		effective = resolved
		diagnostics = storedDiagnostics
	}
	return contract.SettingsWindowManagerResponse{
		SettingsGlobalWorkspaceSectionResponseMetaPayload: settingsGlobalWorkspaceSectionMetaPayload(envelope),
		Config: settingsWindowManagerConfigPayload(
			envelope.WindowManager.Config,
		),
		Defaults:           requiredWindowManagerShortcutsPayload(windowmanager.DefaultKeymap()),
		EffectiveShortcuts: requiredWindowManagerShortcutsPayload(effective),
		Aliases:            cloneSettingsAliases(envelope.WindowManager.Aliases),
		Commands:           settingsWindowManagerCommands(envelope.WindowManager.Commands),
		ExtensionDefaults:  settingsWindowManagerDefaults(envelope.WindowManager.ExtensionDefaults),
		Diagnostics:        settingsWindowManagerDiagnostics(diagnostics),
		GlobalShortcuts:    settingsGlobalShortcuts(envelope.WindowManager.GlobalShortcuts),
	}, nil
}

func settingsGlobalShortcuts(
	items []settingspkg.WindowManagerGlobalShortcut,
) []contract.SettingsGlobalShortcutPayload {
	result := make([]contract.SettingsGlobalShortcutPayload, 0, len(items))
	for _, item := range items {
		result = append(result, contract.SettingsGlobalShortcutPayload{
			CommandID:     item.CommandID,
			IntendedChord: item.IntendedChord,
			ActiveChord:   item.ActiveChord,
			Status:        contract.SettingsGlobalShortcutStatus(item.Status),
			Reason:        item.Reason,
			SettingsURL:   item.SettingsURL,
		})
	}
	return result
}

func settingsWindowManagerCommands(
	commands []settingspkg.WindowManagerShortcutCommand,
) []contract.SettingsWindowManagerCommandPayload {
	result := make([]contract.SettingsWindowManagerCommandPayload, 0, len(commands))
	for _, command := range commands {
		result = append(result, contract.SettingsWindowManagerCommandPayload{
			ID: command.ID, Title: command.Title, Section: command.Section, Source: command.Source,
		})
	}
	return result
}

func settingsWindowManagerDefaults(
	defaults []settingspkg.WindowManagerExtensionDefault,
) []contract.SettingsWindowManagerDefaultPayload {
	result := make([]contract.SettingsWindowManagerDefaultPayload, 0, len(defaults))
	for _, item := range defaults {
		result = append(result, contract.SettingsWindowManagerDefaultPayload{
			CommandID:    item.CommandID,
			Binding:      append(windowmanager.ShortcutBinding(nil), item.Binding...),
			Dormant:      item.Dormant,
			ConflictWith: item.ConflictWith,
		})
	}
	return result
}

func settingsWindowManagerDiagnostics(
	diagnostics []windowmanager.ShortcutDiagnostic,
) []contract.SettingsWindowManagerDiagnosticPayload {
	result := make([]contract.SettingsWindowManagerDiagnosticPayload, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, contract.SettingsWindowManagerDiagnosticPayload{
			CommandID: diagnostic.CommandID, Message: diagnostic.Message,
		})
	}
	return result
}

func cloneSettingsAliases(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	maps.Copy(result, source)
	return result
}

func settingsWindowManagerConfigPayload(
	cfg compozyconfig.WindowManagerConfig,
) contract.SettingsWindowManagerConfigPayload {
	return contract.SettingsWindowManagerConfigPayload{
		NewWindowPolicy:     contract.SettingsWindowNewPolicy(cfg.NewWindowPolicy),
		SmallViewportPolicy: contract.SettingsWindowSmallViewportPolicy(cfg.SmallViewportPolicy),
		FocusPolicy:         contract.SettingsWindowFocusPolicy(cfg.FocusPolicy),
		FocusWrap:           cfg.FocusWrap,
		FocusFollowsPointer: cfg.FocusFollowsPointer,
		RaiseOnFocus:        cfg.RaiseOnFocus,
		DragAwayPolicy:      contract.SettingsWindowDragAwayPolicy(cfg.DragAwayPolicy),
		GroupMoveModifier:   contract.SettingsWindowDragModifier(cfg.GroupMoveModifier),
		SwapModifier:        contract.SettingsWindowDragModifier(cfg.SwapModifier),
		HistoryLimit:        cfg.HistoryLimit,
		NavStackLimit:       cfg.NavStackLimit,
		ClosedEntryLimit:    cfg.ClosedEntryLimit,
		DesktopTransition:   contract.SettingsWindowDesktopTransition(cfg.DesktopTransition),
		Gaps: contract.SettingsWindowManagerGapsPayload{
			Inner:  cfg.Gaps.Inner,
			Top:    cfg.Gaps.Top,
			Right:  cfg.Gaps.Right,
			Bottom: cfg.Gaps.Bottom,
			Left:   cfg.Gaps.Left,
		},
		Snap: contract.SettingsWindowManagerSnapPayload{
			EdgeBand:     cfg.Snap.EdgeBand,
			CornerReach:  cfg.Snap.CornerReach,
			ExitSlack:    cfg.Snap.ExitSlack,
			RepeatRatios: append([]float64(nil), cfg.Snap.RepeatRatios...),
		},
		Bindings: contract.SettingsWindowManagerBindingPayload{
			TopCenter:    contract.SettingsWindowBindingAction(cfg.Bindings.TopCenter),
			BottomCenter: contract.SettingsWindowBindingAction(cfg.Bindings.BottomCenter),
		},
		Shortcuts:       requiredWindowManagerShortcutsPayload(cfg.Shortcuts),
		GlobalShortcuts: windowmanager.CloneGlobalShortcutMap(cfg.GlobalShortcuts),
	}
}

func requiredWindowManagerShortcutsPayload(
	src map[string]windowmanager.ShortcutBinding,
) map[string]windowmanager.ShortcutBinding {
	shortcuts := make(map[string]windowmanager.ShortcutBinding, len(src))
	for action, binding := range src {
		cloned := make(windowmanager.ShortcutBinding, len(binding))
		copy(cloned, binding)
		shortcuts[action] = cloned
	}
	return shortcuts
}

func windowManagerConfigFromPayload(
	payload contract.SettingsWindowManagerConfigPayload,
) (compozyconfig.WindowManagerConfig, error) {
	value := compozyconfig.WindowManagerConfig{
		NewWindowPolicy:     strings.TrimSpace(string(payload.NewWindowPolicy)),
		SmallViewportPolicy: strings.TrimSpace(string(payload.SmallViewportPolicy)),
		FocusPolicy:         strings.TrimSpace(string(payload.FocusPolicy)),
		FocusWrap:           payload.FocusWrap,
		FocusFollowsPointer: payload.FocusFollowsPointer,
		RaiseOnFocus:        payload.RaiseOnFocus,
		DragAwayPolicy:      strings.TrimSpace(string(payload.DragAwayPolicy)),
		GroupMoveModifier:   strings.TrimSpace(string(payload.GroupMoveModifier)),
		SwapModifier:        strings.TrimSpace(string(payload.SwapModifier)),
		HistoryLimit:        payload.HistoryLimit,
		NavStackLimit:       payload.NavStackLimit,
		ClosedEntryLimit:    payload.ClosedEntryLimit,
		DesktopTransition:   strings.TrimSpace(string(payload.DesktopTransition)),
		Gaps: compozyconfig.WindowManagerGapsConfig{
			Inner:  payload.Gaps.Inner,
			Top:    payload.Gaps.Top,
			Right:  payload.Gaps.Right,
			Bottom: payload.Gaps.Bottom,
			Left:   payload.Gaps.Left,
		},
		Snap: compozyconfig.WindowManagerSnapConfig{
			EdgeBand:     payload.Snap.EdgeBand,
			CornerReach:  payload.Snap.CornerReach,
			ExitSlack:    payload.Snap.ExitSlack,
			RepeatRatios: append([]float64(nil), payload.Snap.RepeatRatios...),
		},
		Bindings: compozyconfig.WindowManagerBindingConfig{
			TopCenter:    strings.TrimSpace(string(payload.Bindings.TopCenter)),
			BottomCenter: strings.TrimSpace(string(payload.Bindings.BottomCenter)),
		},
		Shortcuts:       windowmanager.CloneShortcutMap(payload.Shortcuts),
		GlobalShortcuts: windowmanager.CloneGlobalShortcutMap(payload.GlobalShortcuts),
	}
	if err := value.Validate(); err != nil {
		return compozyconfig.WindowManagerConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}
