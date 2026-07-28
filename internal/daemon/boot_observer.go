package daemon

import (
	"context"
	"fmt"
)

func (d *Daemon) attachRuntimeObserver(ctx context.Context, state *bootState) error {
	observer, err := d.newObserver(ctx, state.deps)
	if err != nil {
		return fmt.Errorf("daemon: create observer: %w", err)
	}
	state.observer = observer
	state.deps.Observer = observer
	return nil
}
