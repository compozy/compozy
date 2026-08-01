package cli

import (
	"fmt"
	"maps"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func applyWindowManagerConfigValue(
	cfg *compozyconfig.WindowManagerConfig,
	path []string,
	value any,
) error {
	if cfg == nil || len(path) < 2 || path[0] != configWindowManagerKey {
		return fmt.Errorf("cli: invalid window-manager config path %q", strings.Join(path, "."))
	}
	spec, ok := lookupWindowManagerMutationSpec(path)
	if !ok {
		return fmt.Errorf("cli: unsupported window-manager config path %q", strings.Join(path, "."))
	}
	if err := spec.apply(cfg, path, value); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("cli: validate window-manager config: %w", err)
	}
	return nil
}

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
		Shortcuts: maps.Clone(cfg.Shortcuts),
	}
}
