package config

import "github.com/compozy/compozy/internal/windowmanager"

// WindowNewPolicyFloating is translated only at public boundaries until v0.8.0.
const WindowNewPolicyFloating = "floating"

// Retired window-manager keys are accepted with any value until v0.8.0.
// WithWindowManagerWarning routes compatibility warnings to the caller's output.
func WithWindowManagerWarning(warn func(string, bool)) LoadOption {
	return func(options *loadOptions) { options.windowManagerWarning = warn }
}

func (c WindowManagerConfig) warnRetiredKey(key string, translated bool) {
	if c.retiredKeyWarning != nil {
		c.retiredKeyWarning(key, translated)
		return
	}
	windowmanager.WarnRetiredConfigKey(key, translated)
}

func warnRetiredWindowManagerOverlay(o windowManagerOverlay, dst *WindowManagerConfig) {
	for _, field := range []struct {
		key   string
		value any
	}{
		{"raise_on_focus", o.RaiseOnFocus}, {"drag_away_policy", o.DragAwayPolicy}, {"group_move_modifier", o.GroupMoveModifier},
		{"snap.edge_band", o.Snap.EdgeBand}, {"snap.corner_reach", o.Snap.CornerReach}, {"snap.exit_slack", o.Snap.ExitSlack},
		{"snap.repeat_ratios", o.Snap.RepeatRatios}, {"bindings.top_center", o.Bindings.TopCenter}, {"bindings.bottom_center", o.Bindings.BottomCenter},
	} {
		if field.value != nil {
			dst.warnRetiredKey(field.key, false)
		}
	}
}
