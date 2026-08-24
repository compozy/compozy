package daemon

import (
	"errors"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

type attentionConfigSetter interface {
	SetAttentionConfig(compozyconfig.AttentionConfig) error
}

func (a daemonSettingsRuntimeApplier) applyAttentionConfigChange(
	previous *compozyconfig.Config,
	next *compozyconfig.Config,
) *settingspkg.ApplyFailure {
	if attentionConfigsEqual(previous.Attention, next.Attention) {
		return nil
	}
	setter, ok := a.state.sessions.(attentionConfigSetter)
	if !ok {
		failure := configApplyFailure(
			"attention",
			diagnosticcontract.CategoryConfig,
			"Attention config sync failed",
			errors.New("daemon: session manager does not support attention config"),
		)
		return &failure
	}
	if err := setter.SetAttentionConfig(next.Attention); err != nil {
		failure := configApplyFailure(
			"attention",
			diagnosticcontract.CategoryConfig,
			"Attention config sync failed",
			err,
		)
		return &failure
	}
	return nil
}

func attentionConfigsEqual(left compozyconfig.AttentionConfig, right compozyconfig.AttentionConfig) bool {
	return left.Toasts == right.Toasts &&
		left.Sound == right.Sound &&
		left.System == right.System
}
