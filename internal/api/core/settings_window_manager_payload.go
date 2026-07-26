package core

import (
	"errors"
	"maps"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	settingspkg "github.com/compozy/agh/internal/settings"
)

func settingsWindowManagerSectionResponse(
	envelope settingspkg.SectionEnvelope,
) (any, error) {
	if envelope.WindowManager == nil {
		return nil, errors.New("settings window-manager section is required")
	}
	return contract.SettingsWindowManagerResponse{
		SettingsGlobalSectionResponseMetaPayload: settingsGlobalSectionMetaPayload(envelope),
		Config: settingsWindowManagerConfigPayload(
			envelope.WindowManager.Config,
		),
	}, nil
}

func settingsWindowManagerConfigPayload(
	cfg aghconfig.WindowManagerConfig,
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
		Shortcuts: requiredWindowManagerShortcutsPayload(cfg.Shortcuts),
	}
}

func requiredWindowManagerShortcutsPayload(src map[string]string) map[string]string {
	shortcuts := make(map[string]string, len(src))
	maps.Copy(shortcuts, src)
	return shortcuts
}

func windowManagerConfigFromPayload(
	payload contract.SettingsWindowManagerConfigPayload,
) (aghconfig.WindowManagerConfig, error) {
	value := aghconfig.WindowManagerConfig{
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
		DesktopTransition:   strings.TrimSpace(string(payload.DesktopTransition)),
		Gaps: aghconfig.WindowManagerGapsConfig{
			Inner:  payload.Gaps.Inner,
			Top:    payload.Gaps.Top,
			Right:  payload.Gaps.Right,
			Bottom: payload.Gaps.Bottom,
			Left:   payload.Gaps.Left,
		},
		Snap: aghconfig.WindowManagerSnapConfig{
			EdgeBand:     payload.Snap.EdgeBand,
			CornerReach:  payload.Snap.CornerReach,
			ExitSlack:    payload.Snap.ExitSlack,
			RepeatRatios: append([]float64(nil), payload.Snap.RepeatRatios...),
		},
		Bindings: aghconfig.WindowManagerBindingConfig{
			TopCenter:    strings.TrimSpace(string(payload.Bindings.TopCenter)),
			BottomCenter: strings.TrimSpace(string(payload.Bindings.BottomCenter)),
		},
		Shortcuts: cloneStringMap(payload.Shortcuts),
	}
	if err := value.Validate(); err != nil {
		return aghconfig.WindowManagerConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}
