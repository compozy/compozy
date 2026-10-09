package session

import (
	"context"
	"sync"

	"github.com/compozy/compozy/internal/store"
)

type subagentSubscription struct {
	channel chan SubagentUpdate
	pending map[string]SubagentUpdate
	signal  chan struct{}
	done    chan struct{}
}

func (s *subagentService) SubscribeSubagentUpdates(
	ctx context.Context,
	parent string,
) (<-chan SubagentUpdate, func(), error) {
	sub := &subagentSubscription{
		channel: make(chan SubagentUpdate),
		pending: make(map[string]SubagentUpdate),
		signal:  make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	s.mu.Lock()
	if s.subscribers[parent] == nil {
		s.subscribers[parent] = make(map[*subagentSubscription]struct{})
	}
	s.subscribers[parent][sub] = struct{}{}
	s.mu.Unlock()
	cancel := sync.OnceFunc(func() {
		s.mu.Lock()
		delete(s.subscribers[parent], sub)
		if len(s.subscribers[parent]) == 0 {
			delete(s.subscribers, parent)
		}
		close(sub.done)
		s.mu.Unlock()
	})
	s.launch(func() {
		defer close(sub.channel)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.ctx.Done():
				return
			case <-sub.done:
				return
			case <-sub.signal:
			}
			for {
				s.mu.Lock()
				var update SubagentUpdate
				found := false
				for id := range sub.pending {
					update = sub.pending[id]
					delete(sub.pending, id)
					found = true
					break
				}
				s.mu.Unlock()
				if !found {
					break
				}
				select {
				case <-ctx.Done():
					return
				case <-s.ctx.Done():
					return
				case <-sub.done:
					return
				case sub.channel <- update:
				}
			}
		}
	})
	return sub.channel, cancel, nil
}

func (s *subagentService) publish(_ context.Context, row store.SessionSubagent) {
	update := SubagentUpdate{ParentSessionID: row.ParentSessionID, Subagent: presentSubagent(row)}
	s.mu.Lock()
	for sub := range s.subscribers[row.ParentSessionID] {
		sub.pending[row.ID] = update
		select {
		case sub.signal <- struct{}{}:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *subagentService) publishTerminal(ctx context.Context, row store.SessionSubagent) {
	s.publish(ctx, row)
	s.runtime.PublishParent(ctx, row.ParentSessionID)
	if s.settled != nil {
		s.launch(func() {
			hookCtx, cancel := context.WithTimeout(s.ctx, defaultLifecycleTimeout)
			defer cancel()
			s.logError(hookCtx, "settled_hook", s.settled.DispatchSubagentSettled(hookCtx, row))
		})
	}
}
