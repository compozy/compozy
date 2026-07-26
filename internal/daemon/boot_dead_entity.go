package daemon

import (
	"errors"

	"github.com/compozy/agh/internal/deadentity"
	"github.com/compozy/agh/internal/store"
)

func (d *Daemon) bootDeadEntityRegistry(state *bootState) error {
	if state == nil || state.registry == nil {
		return errors.New("daemon: global registry is required for dead-entity recovery")
	}
	deadStore, ok := state.registry.(store.DeadEntityStore)
	if !ok {
		return errors.New("daemon: global registry does not support dead-entity recovery")
	}
	state.deadEntities = deadentity.New(
		deadStore,
		deadentity.WithClock(d.now),
		deadentity.WithLogger(state.logger),
		deadentity.WithEventStore(state.registry),
	)
	return nil
}
