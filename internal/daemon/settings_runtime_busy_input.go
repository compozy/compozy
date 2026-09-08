package daemon

import (
	"errors"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/settings"
)

type busyInputDefaultSetter interface {
	SetBusyInputDefaultMode(string) error
}

func (a daemonSettingsRuntimeApplier) applyBusyInputDefault(previous, next *config.Config) *settings.ApplyFailure {
	if previous.Session.BusyInput.Normalize().DefaultMode == next.Session.BusyInput.Normalize().DefaultMode {
		return nil
	}
	setter, ok := a.state.sessions.(busyInputDefaultSetter)
	var err error
	if !ok {
		err = errors.New("daemon: session manager does not support follow-up configuration")
	} else {
		err = setter.SetBusyInputDefaultMode(next.Session.BusyInput.DefaultMode)
	}
	if err == nil {
		return nil
	}
	failure := configApplyFailure(
		"busy_input",
		diagnosticcontract.CategoryConfig,
		"Follow-up configuration failed",
		err,
	)
	return &failure
}
