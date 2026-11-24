package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/store"
)

func WithComputersOptions(options ...computers.Option) Option {
	return func(d *Daemon) { d.computersOptions = append(d.computersOptions, options...) }
}

func (d *Daemon) bootComputers(ctx context.Context, state *bootState, cleanup *bootCleanup) {
	storage, ok := state.registry.(store.ComputerStore)
	workers := &state.runtimeWorkers
	if !ok || workers.account == nil {
		return
	}
	options := append([]computers.Option{computers.WithLogger(state.logger)}, d.computersOptions...)
	service := computers.New(ctx, storage, workers.account, workers.cloud, options...)
	workers.computers = service
	state.deps.Computers = service
	cleanup.add(func(context.Context) error { service.Close(); return nil })
}
