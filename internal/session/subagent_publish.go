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
	s.dispatchSettledHook(row)
}

func (s *subagentService) dispatchSettledHook(row store.SessionSubagent) {
	if s.settled != nil {
		s.launch(func() {
			// The hook pool owns execution timeouts; dispatch return must not cancel queued hooks.
			s.logError(s.ctx, "settled_hook", s.settled.DispatchSubagentSettled(s.ctx, row))
		})
	}
}

// deferSettledHookForFacts holds the once-only settled hook of a canceled isolated subagent until its
// final worktree facts are persisted (persistTerminalWorktreeFacts). It defers only while the child is still
// live: the cancel's stop then settles it and the settle edge observes the facts outside the parent lock.
func (s *subagentService) deferSettledHookForFacts(ctx context.Context, row store.SessionSubagent) bool {
	if s.worktrees == nil || row.Isolation != SubagentIsolationWorktree || row.ChildSessionID == nil ||
		row.WorktreeState().ID == "" || row.WorktreeState().Facts.PRStatus != "" {
		return false
	}
	snap, err := s.runtime.Snapshot(ctx, *row.ChildSessionID)
	if err != nil || snap.Info == nil || snap.Info.State == StateStopped {
		return false
	}
	s.awaitingFacts.Store(row.ID, struct{}{})
	return true
}
