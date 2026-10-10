package daemon

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func (d *Daemon) bootReplyWatches(ctx context.Context, state *bootState) error {
	manager, ok := state.sessions.(*session.Manager)
	if !ok {
		return nil
	}
	db, ok := state.registry.(store.ReplyWatchStore)
	if !ok {
		return errors.New("daemon: registry does not implement reply watch store")
	}
	service, err := session.NewReplyWatchService(db, manager)
	if err != nil {
		return err
	}
	if err := service.Recover(ctx); err != nil {
		return err
	}
	manager.SetReplyWatchService(service)
	return nil
}
