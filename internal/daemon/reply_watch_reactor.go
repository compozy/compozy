package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func (d *Daemon) bootReplyWatches(_ context.Context, state *bootState) error {
	manager, ok := state.sessions.(*session.Manager)
	if !ok {
		return nil
	}
	db, ok := state.registry.(store.ReplyWatchStore)
	if !ok {
		return errors.New("daemon: registry does not implement reply watch store")
	}
	state.replyWatchRecoveryGate = make(chan struct{})
	service, err := session.NewReplyWatchService(
		db, manager, session.WithReplyWatchRecoveryGate(state.replyWatchRecoveryGate),
	)
	if err != nil {
		return err
	}
	manager.SetReplyWatchService(service)
	return nil
}

// Durable monitors must already be registered before a reply watch can classify a target as settled.
func recoverBootReplyWatches(ctx context.Context, state *bootState) error {
	manager, ok := state.sessions.(*session.Manager)
	if !ok || state.replyWatchRecoveryGate == nil {
		return nil
	}
	close(state.replyWatchRecoveryGate)
	if err := manager.ReplyWatches().Recover(ctx); err != nil {
		return fmt.Errorf("daemon: recover reply watches: %w", err)
	}
	return nil
}
