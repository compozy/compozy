package cli

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
)

func settingsWindowManagerPayloadFromConfig(
	cfg compozyconfig.WindowManagerConfig,
) contract.SettingsWindowManagerConfigPayload {
	return contract.SettingsWindowManagerConfigPayload{
		NewWindowPolicy:     contract.SettingsWindowNewPolicy(cfg.NewWindowPolicy),
		SmallViewportPolicy: contract.SettingsWindowSmallViewportPolicy(cfg.SmallViewportPolicy),
		FocusPolicy:         contract.SettingsWindowFocusPolicy(cfg.FocusPolicy),
		FocusWrap:           cfg.FocusWrap,
		FocusFollowsPointer: cfg.FocusFollowsPointer,
		SwapModifier:        contract.SettingsWindowDragModifier(cfg.SwapModifier),
		HistoryLimit:        cfg.HistoryLimit,
		NavStackLimit:       cfg.NavStackLimit,
		ClosedEntryLimit:    cfg.ClosedEntryLimit,
		DesktopTransition:   contract.SettingsWindowDesktopTransition(cfg.DesktopTransition),
		Gaps: contract.SettingsWindowManagerGapsPayload{
			Inner: cfg.Gaps.Inner, Top: cfg.Gaps.Top, Right: cfg.Gaps.Right,
			Bottom: cfg.Gaps.Bottom, Left: cfg.Gaps.Left,
		},
		// Retained wire placeholders for v0.6 readers; removed in v0.8.0.
		DragAwayPolicy:    contract.SettingsWindowDragAwayPolicyWindow,
		GroupMoveModifier: contract.SettingsWindowDragModifierNone,
		Snap:              contract.SettingsWindowManagerSnapPayload{RepeatRatios: []float64{}},
		Bindings: contract.SettingsWindowManagerBindingPayload{
			TopCenter: contract.SettingsWindowBindingActionNone, BottomCenter: contract.SettingsWindowBindingActionNone,
		},
		Shortcuts:       windowmanager.CloneShortcutMap(cfg.Shortcuts),
		GlobalShortcuts: windowmanager.CloneGlobalShortcutMap(cfg.GlobalShortcuts),
	}
}

type configWindowManagerDiscovery struct {
	Config    contract.SettingsWindowManagerConfigPayload `json:"config"`
	Defaults  map[string]windowmanager.ShortcutBinding    `json:"defaults"`
	Effective map[string]windowmanager.ShortcutBinding    `json:"effective"`
}

func windowManagerConfigDiscovery(
	cfg compozyconfig.WindowManagerConfig,
) (configWindowManagerDiscovery, error) {
	effective, err := windowmanager.EffectiveStoredKeymap(cfg.Shortcuts)
	if err != nil {
		return configWindowManagerDiscovery{}, fmt.Errorf("cli: resolve effective window-manager keymap: %w", err)
	}
	return configWindowManagerDiscovery{
		Config:    settingsWindowManagerPayloadFromConfig(cfg),
		Defaults:  windowmanager.DefaultKeymap(),
		Effective: effective,
	}, nil
}
