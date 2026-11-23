package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/cloudclient"
)

// WithAccountOptions configures the installation's account service.
func WithAccountOptions(options ...account.Option) Option {
	return func(d *Daemon) { d.accountOptions = append(d.accountOptions, options...) }
}

func (d *Daemon) bootAccount(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	store, ok := state.registry.(account.Store)
	if !ok {
		return nil
	}
	cloud, err := cloudclient.New(cloudclient.WithClock(d.now))
	if err != nil {
		return fmt.Errorf("daemon: configure Compozy Cloud: %w", err)
	}
	options := append([]account.Option{account.WithLogger(state.logger)}, d.accountOptions...)
	service, err := account.New(ctx, store, cloud, d.homePaths.HomeDir, options...)
	if err != nil {
		return fmt.Errorf("daemon: open installation account: %w", err)
	}
	state.runtimeWorkers.account = service
	cleanup.add(func(context.Context) error { service.Close(); return nil })
	return nil
}

func accountRuntimeService(state *bootState) core.AccountService {
	if state.runtimeWorkers.account == nil {
		return nil
	}
	return state.runtimeWorkers.account
}
