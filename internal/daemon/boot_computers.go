package daemon

import (
	"cmp"
	"context"

	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/version"
)

func WithComputersOptions(options ...computers.Option) Option {
	return func(d *Daemon) { d.computersOptions = append(d.computersOptions, options...) }
}

func WithSSHSetupOptions(options sshsetup.Options) Option {
	return func(d *Daemon) { d.sshSetupOptions = options }
}

func (d *Daemon) bootComputers(ctx context.Context, state *bootState, cleanup *bootCleanup) {
	storage, ok := state.registry.(store.ComputerStore)
	workers := &state.runtimeWorkers
	if !ok || workers.account == nil {
		return
	}
	var service *computers.Registry
	options := []computers.Option{
		computers.WithLogger(state.logger),
		computers.WithLocalSettings(
			d.computerSettings,
			func(ctx context.Context, patch computers.LocalSettingsPatch) (config.ComputersConfig, error) {
				return d.patchLocalComputers(ctx, service, patch)
			},
		),
	}
	if setups, ok := state.registry.(store.ComputerSSHSetupStore); ok {
		if home, err := config.ResolveOperatorHomeDir(d.homePaths); err == nil {
			settings := d.sshSetupOptions
			settings.Home, settings.OperatorHome = d.homePaths.HomeDir, home
			settings.Version, settings.Logger = cmp.Or(settings.Version, version.Current().Version), state.logger
			options = append(options, computers.WithManagedSSH(func(registry *computers.Registry) *sshsetup.Manager {
				return sshsetup.New(ctx, workers.account, workers.cloud, setups, registry, settings)
			}))
		} else {
			state.logger.WarnContext(ctx, "SSH setup is unavailable", "error", err)
		}
	}
	options = append(options, d.computersOptions...)
	service = computers.New(ctx, storage, workers.account, workers.cloud, options...)
	workers.computers = service
	state.deps.Computers = service
	cleanup.add(func(context.Context) error { service.Close(); return nil })
}
