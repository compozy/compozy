package daemon

import (
	"context"
	"fmt"
	"maps"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"
)

func (o *loopWatchEventsObserver) Hydrate(ctx context.Context) error {
	parked, err := o.store.ListParkedWatchEventSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("daemon: hydrate loop watch-events observer: %w", err)
	}
	o.replaceIndex(parked)
	return nil
}

func (o *loopWatchEventsObserver) refreshLoopRun(ctx context.Context, loopRunID string) error {
	loopRunID = strings.TrimSpace(loopRunID)
	parked, err := o.store.ListParkedWatchEventSubscriptionsForLoopRun(ctx, loopRunID)
	if err != nil {
		return fmt.Errorf("daemon: refresh loop watch-events observer for loop run %q: %w", loopRunID, err)
	}
	o.replaceLoopRun(loopRunID, parked)
	return nil
}

func (o *loopWatchEventsObserver) replaceIndex(entries []looppkg.ParkedWatchEventSubscription) {
	byKind := make(map[hookspkg.HookEvent][]looppkg.ParkedWatchEventSubscription)
	byLoopRun := make(map[string]map[hookspkg.HookEvent]struct{})
	for _, entry := range entries {
		indexParkedWatchEventSubscription(byKind, byLoopRun, entry)
	}
	o.mu.Lock()
	o.byKind = byKind
	o.byLoopRun = byLoopRun
	o.mu.Unlock()
}

func (o *loopWatchEventsObserver) replaceLoopRun(
	loopRunID string,
	entries []looppkg.ParkedWatchEventSubscription,
) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.removeLoopRunLocked(loopRunID)
	for _, entry := range entries {
		indexParkedWatchEventSubscription(o.byKind, o.byLoopRun, entry)
	}
}

func (o *loopWatchEventsObserver) removeLoopRun(loopRunID string) {
	loopRunID = strings.TrimSpace(loopRunID)
	if loopRunID == "" {
		return
	}
	o.mu.Lock()
	o.removeLoopRunLocked(loopRunID)
	o.mu.Unlock()
}

func (o *loopWatchEventsObserver) removeLoopRunLocked(loopRunID string) {
	for kind := range o.byLoopRun[loopRunID] {
		entries := o.byKind[kind]
		kept := entries[:0]
		for _, entry := range entries {
			if strings.TrimSpace(entry.LoopRunID) != loopRunID {
				kept = append(kept, entry)
			}
		}
		clear(entries[len(kept):])
		if len(kept) == 0 {
			delete(o.byKind, kind)
		} else {
			o.byKind[kind] = kept
		}
	}
	delete(o.byLoopRun, loopRunID)
}

func (o *loopWatchEventsObserver) subscriptionsForKind(
	kind hookspkg.HookEvent,
) []looppkg.ParkedWatchEventSubscription {
	o.mu.RLock()
	entries := append([]looppkg.ParkedWatchEventSubscription(nil), o.byKind[kind]...)
	o.mu.RUnlock()
	return entries
}

func indexParkedWatchEventSubscription(
	byKind map[hookspkg.HookEvent][]looppkg.ParkedWatchEventSubscription,
	byLoopRun map[string]map[hookspkg.HookEvent]struct{},
	entry looppkg.ParkedWatchEventSubscription,
) {
	cloned := cloneParkedWatchEventSubscription(entry)
	loopRunID := strings.TrimSpace(cloned.LoopRunID)
	if loopRunID == "" {
		return
	}
	seen := map[hookspkg.HookEvent]struct{}{}
	for _, ref := range cloned.Subscriptions {
		kind := hookspkg.HookEvent(strings.TrimSpace(ref.Kind))
		if _, ok := cloned.Contracts[kind]; !ok {
			continue
		}
		if _, ok := seen[kind]; ok {
			continue
		}
		seen[kind] = struct{}{}
		byKind[kind] = append(byKind[kind], cloned)
		if byLoopRun[loopRunID] == nil {
			byLoopRun[loopRunID] = map[hookspkg.HookEvent]struct{}{}
		}
		byLoopRun[loopRunID][kind] = struct{}{}
	}
}

func cloneParkedWatchEventSubscription(
	src looppkg.ParkedWatchEventSubscription,
) looppkg.ParkedWatchEventSubscription {
	src.Inputs = maps.Clone(src.Inputs)
	src.Subscriptions = append(src.Subscriptions[:0:0], src.Subscriptions...)
	src.Cursors = maps.Clone(src.Cursors)
	src.Contracts = maps.Clone(src.Contracts)
	return src
}
