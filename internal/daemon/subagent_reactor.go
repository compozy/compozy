package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func (d *Daemon) bootSubagents(ctx context.Context, state *bootState) error {
	manager, ok := state.sessions.(*session.Manager)
	if !ok {
		return nil
	}
	db, ok := state.registry.(store.SubagentStore)
	if !ok {
		return errors.New("daemon: registry does not implement the subagent store")
	}
	service, err := session.NewSubagentService(db, manager,
		session.WithSubagentResultLimit(func() int {
			d.mu.Lock()
			cfg := d.config
			booting := d.booting
			d.mu.Unlock()
			if booting {
				cfg = state.cfg
			}
			if source, ok := any(cfg).(interface{ SubagentResultMaxChars() int }); ok {
				return source.SubagentResultMaxChars()
			}
			return 60000
		}),
		session.WithSubagentSettledDispatcher(subagentSettledBridge{state: state}),
	)
	if err != nil {
		return fmt.Errorf("daemon: create subagent service: %w", err)
	}
	manager.SetSubagentService(service)
	state.subagents = service
	if err := service.Recover(ctx); err != nil {
		return fmt.Errorf("daemon: recover subagents: %w", err)
	}
	return nil
}

type subagentSettledBridge struct{ state *bootState }

var _ session.SubagentSettledDispatcher = subagentSettledBridge{}

func (b subagentSettledBridge) DispatchSubagentSettled(ctx context.Context, row store.SessionSubagent) error {
	if b.state.notifier == nil {
		return nil
	}
	if dispatcher, ok := any(b.state.notifier).(session.SubagentSettledDispatcher); ok {
		return dispatcher.DispatchSubagentSettled(ctx, row)
	}
	return errors.New("daemon: subagent settled hook adapter is not installed")
}

func (d *Daemon) SubagentService() session.SubagentService {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.subagents
}
