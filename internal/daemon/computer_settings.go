package daemon

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/config"
)

func (d *Daemon) computerSettings() config.ComputersConfig {
	if settings := d.localComputers.Load(); settings != nil {
		return *settings
	}
	return config.DefaultComputersConfig()
}

func (d *Daemon) checkLocalExecution() error {
	if !d.computerSettings().LocalRunAgents {
		return admission.ErrLocalExecutionDisabled
	}
	return nil
}

func (d *Daemon) patchLocalComputers(
	ctx context.Context, registry *computers.Registry, patch computers.LocalSettingsPatch,
) (config.ComputersConfig, error) {
	target, err := config.ResolveConfigWriteTarget(d.homePaths, "", config.WriteScopeUser, "")
	if err != nil {
		return config.ComputersConfig{}, err
	}
	next, err := config.EditConfigOverlayAndApply(d.homePaths, "", target, func(editor *config.OverlayEditor) error {
		fields := []struct {
			key string
			val any
		}{
			{"local_run_agents", patch.RunAgents},
			{"load_balancing", patch.LoadBalancing},
			{"local_load_balancing_weight", patch.Weight},
		}
		for _, field := range fields {
			switch val := field.val.(type) {
			case *bool:
				if val != nil {
					if err := editor.SetValue([]string{"computers", field.key}, *val); err != nil {
						return err
					}
				}
			case *int:
				if val != nil {
					if err := editor.SetValue([]string{"computers", field.key}, *val); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}, func(next config.Config) error {
		if err := validateComputerSettings(ctx, registry, d.computerSettings(), next.Computers); err != nil {
			return err
		}
		d.localComputers.Store(new(next.Computers))
		return nil
	})
	if invalid, ok := errors.AsType[config.ValidationError](err); ok {
		return config.ComputersConfig{}, &computers.Failure{
			Status: http.StatusBadRequest, Code: "config_invalid", Message: invalid.Error(),
		}
	}
	return next.Computers, err
}

func validateComputerSettings(
	ctx context.Context, registry *computers.Registry, previous, next config.ComputersConfig,
) error {
	if !previous.LocalRunAgents || next.LocalRunAgents {
		return ctx.Err()
	}
	var added int
	if registry != nil {
		rows, err := registry.List(ctx)
		if err != nil {
			return err
		}
		added = len(rows)
	}
	return computers.ValidateLocalExecutionChange(previous, next, added)
}
