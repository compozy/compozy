package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/hooks"
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
		session.WithSubagentResultLimit(func(ctx context.Context, workspaceID string) (int, error) {
			workspace, err := state.workspaceResolver.Resolve(ctx, workspaceID)
			if err != nil {
				return 0, err
			}
			return workspace.Config.SubagentResultMaxChars(), nil
		}),
		session.WithSubagentSettledDispatcher(subagentSettledBridge{state: state}),
	)
	if err != nil {
		return fmt.Errorf("daemon: create subagent service: %w", err)
	}
	manager.SetSubagentService(service)
	state.subagents = service
	// The runtime deps were built before this step; the HTTP/UDS transports read
	// the service from them, so publish it there too.
	state.deps.Subagents = service
	if err := service.Recover(ctx); err != nil {
		return fmt.Errorf("daemon: recover subagents: %w", err)
	}
	return nil
}

type subagentSettledBridge struct{ state *bootState }

var _ session.SubagentSettledDispatcher = subagentSettledBridge{}

func (b subagentSettledBridge) DispatchSubagentSettled(ctx context.Context, row store.SessionSubagent) error {
	if b.state.hooks == nil {
		return nil
	}
	dispatcher, ok := b.state.hooks.(interface {
		DispatchSubagentSettled(context.Context, hooks.SubagentSettledPayload) (hooks.SubagentSettledPayload, error)
	})
	if !ok {
		return errors.New("daemon: hook runtime lacks subagent dispatch")
	}
	payload := hooks.SubagentSettledPayload{
		Event:           hooks.HookSubagentSettled,
		Timestamp:       row.UpdatedAt,
		WorkspaceID:     row.WorkspaceID,
		SubagentID:      row.ID,
		ParentSessionID: row.ParentSessionID,
		ChildSessionID:  row.ChildSessionID,
		Origin:          row.Origin,
		Status:          row.Status,
		Runtime:         hooks.SubagentRuntimePayload{Provider: row.RuntimeProvider, Model: row.RuntimeModel},
	}
	if row.SettledAt != nil {
		payload.Timestamp = *row.SettledAt
	}
	if row.StartedAt != nil && row.SettledAt != nil {
		payload.DurationMS = row.SettledAt.Sub(*row.StartedAt).Milliseconds()
	}
	if b.state.notifier != nil {
		profile, err := b.state.notifier.sessionProfile(ctx, row.ParentSessionID)
		if err != nil {
			return err
		}
		payload.ProfileID = profile
	}
	_, err := dispatcher.DispatchSubagentSettled(ctx, payload)
	return err
}

func (d *Daemon) SubagentService() session.SubagentService {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.subagents
}
