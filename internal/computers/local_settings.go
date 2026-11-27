package computers

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/config"
)

type LocalSettingsPatch struct {
	RunAgents     *bool
	LoadBalancing *bool
	Weight        *int
}

func WithLocalSettings(
	read func() config.ComputersConfig,
	patch func(context.Context, LocalSettingsPatch) (config.ComputersConfig, error),
) Option {
	return func(r *Registry) { r.localConfig, r.patchLocal = read, patch }
}

func (r *Registry) LocalSettings() config.ComputersConfig {
	if r.localConfig == nil {
		return config.DefaultComputersConfig()
	}
	return r.localConfig()
}

func (r *Registry) PatchLocalSettings(ctx context.Context, patch LocalSettingsPatch) (config.ComputersConfig, error) {
	if _, err := r.current(); err != nil {
		return config.ComputersConfig{}, err
	}
	if r.patchLocal == nil {
		return config.ComputersConfig{}, invalidComputerRequest("Local computer settings are unavailable.")
	}
	return r.patchLocal(ctx, patch)
}

func ValidateLocalExecutionChange(previous, next config.ComputersConfig, added int) error {
	if previous.LocalRunAgents && !next.LocalRunAgents && added == 0 {
		return &Failure{Status: http.StatusBadRequest, Code: "config_invalid", Message: "Add a computer first",
			Details: map[string]string{"key": "computers.local_run_agents"}}
	}
	return nil
}
