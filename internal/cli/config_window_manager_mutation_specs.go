package cli

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/windowmanager"
)

type windowManagerMutationSpec struct {
	kind  configSetValueKind
	apply func(*compozyconfig.WindowManagerConfig, []string, any) error
}

var windowManagerMutationSpecs = map[string]windowManagerMutationSpec{
	"new_window_policy": {
		kind: configSetString,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.NewWindowPolicy, "string")
		},
	},
	"small_viewport_policy": {
		kind: configSetString,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.SmallViewportPolicy, "string")
		},
	},
	"focus_policy": {
		kind: configSetString,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.FocusPolicy, "string")
		},
	},
	"focus_wrap": {
		kind: configSetBool,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.FocusWrap, "boolean")
		},
	},
	"focus_follows_pointer": {
		kind: configSetBool,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.FocusFollowsPointer, "boolean")
		},
	},
	"raise_on_focus": {
		kind: configSetBool,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.RaiseOnFocus, "boolean")
		},
	},
	"drag_away_policy": {
		kind: configSetString,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.DragAwayPolicy, "string")
		},
	},
	"group_move_modifier": {
		kind: configSetString,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.GroupMoveModifier, "string")
		},
	},
	"swap_modifier": {
		kind: configSetString,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.SwapModifier, "string")
		},
	},
	"history_limit": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.HistoryLimit, "integer")
		},
	},
	"nav_stack_limit": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.NavStackLimit, "integer")
		},
	},
	"closed_entry_limit": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.ClosedEntryLimit, "integer")
		},
	},
	"desktop_transition": {
		kind: configSetString,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.DesktopTransition, "string")
		},
	},
	"gaps.inner": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Gaps.Inner, "integer")
		},
	},
	"gaps.top": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Gaps.Top, "integer")
		},
	},
	"gaps.right": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Gaps.Right, "integer")
		},
	},
	"gaps.bottom": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Gaps.Bottom, "integer")
		},
	},
	"gaps.left": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Gaps.Left, "integer")
		},
	},
	"snap.edge_band": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Snap.EdgeBand, "integer")
		},
	},
	"snap.corner_reach": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Snap.CornerReach, "integer")
		},
	},
	"snap.exit_slack": {
		kind: configSetInt,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Snap.ExitSlack, "integer")
		},
	},
	"snap.repeat_ratios": {
		kind: configSetFloatSlice,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			ratios, err := requireWindowManagerValue[[]float64](path, value, "float array")
			if err != nil {
				return err
			}
			cfg.Snap.RepeatRatios = append([]float64(nil), ratios...)
			return nil
		},
	},
	"bindings.top_center": {
		kind: configSetString,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Bindings.TopCenter, "string")
		},
	},
	"bindings.bottom_center": {
		kind: configSetString,
		apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
			return assignWindowManagerValue(path, value, &cfg.Bindings.BottomCenter, "string")
		},
	},
}

var windowManagerShortcutMutationSpec = windowManagerMutationSpec{
	kind: configSetStringOrStringSlice,
	apply: func(cfg *compozyconfig.WindowManagerConfig, path []string, value any) error {
		var shortcut windowmanager.ShortcutBinding
		switch typed := value.(type) {
		case string:
			shortcut = windowmanager.ShortcutBinding{typed}
		case []string:
			shortcut = append(windowmanager.ShortcutBinding(nil), typed...)
		case []any:
			shortcut = make(windowmanager.ShortcutBinding, len(typed))
			for index, member := range typed {
				chord, ok := member.(string)
				if !ok {
					return unexpectedWindowManagerConfigValue(path, value, "string or string array")
				}
				shortcut[index] = chord
			}
		default:
			return unexpectedWindowManagerConfigValue(path, value, "string or string array")
		}
		if cfg.Shortcuts == nil {
			cfg.Shortcuts = make(map[string]windowmanager.ShortcutBinding)
		}
		cfg.Shortcuts[path[2]] = shortcut
		return nil
	},
}

func lookupWindowManagerMutationSpec(path []string) (windowManagerMutationSpec, bool) {
	if len(path) < 2 || len(path) > 3 || path[0] != configWindowManagerKey {
		return windowManagerMutationSpec{}, false
	}
	if len(path) == 3 && path[1] == configWindowManagerShortcutsKey {
		return windowManagerShortcutMutationSpec, true
	}
	spec, ok := windowManagerMutationSpecs[strings.Join(path[1:], ".")]
	return spec, ok
}
