package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
)

func assignWindowManagerValue[T any](path []string, value any, target *T, want string) error {
	typed, err := requireWindowManagerValue[T](path, value, want)
	if err != nil {
		return err
	}
	*target = typed
	return nil
}

func requireWindowManagerValue[T any](path []string, value any, want string) (T, error) {
	typed, ok := value.(T)
	if ok {
		return typed, nil
	}
	var zero T
	return zero, unexpectedWindowManagerConfigValue(path, value, want)
}

func unexpectedWindowManagerConfigValue(path []string, value any, want string) error {
	return fmt.Errorf(
		"cli: config set %q expects a %s payload, got %T",
		strings.Join(path, "."),
		want,
		value,
	)
}

func settingsWindowManagerPayloadFromConfig(
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
			Inner: cfg.Gaps.Inner, Top: cfg.Gaps.Top, Right: cfg.Gaps.Right,
			Bottom: cfg.Gaps.Bottom, Left: cfg.Gaps.Left,
		},
		Snap: contract.SettingsWindowManagerSnapPayload{
			EdgeBand: cfg.Snap.EdgeBand, CornerReach: cfg.Snap.CornerReach,
			ExitSlack: cfg.Snap.ExitSlack, RepeatRatios: append([]float64(nil), cfg.Snap.RepeatRatios...),
		},
		Bindings: contract.SettingsWindowManagerBindingPayload{
			TopCenter:    contract.SettingsWindowBindingAction(cfg.Bindings.TopCenter),
			BottomCenter: contract.SettingsWindowBindingAction(cfg.Bindings.BottomCenter),
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
