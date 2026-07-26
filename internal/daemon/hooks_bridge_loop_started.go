package daemon

import (
	"context"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

type loopStartedObserver interface {
	OnLoopStarted(context.Context, hookspkg.LoopStartedPayload) error
}

func (n *hooksNotifier) AddLoopStartedObserver(observer loopStartedObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.loopStartedHooks = append(n.loopStartedHooks, observer)
}

func (n *hooksNotifier) loopStartedObservers() []loopStartedObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]loopStartedObserver(nil), n.loopStartedHooks...)
}

func (n *hooksNotifier) notifyLoopStartedObservers(
	ctx context.Context,
	payload hookspkg.LoopStartedPayload,
) {
	for _, observer := range n.loopStartedObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"loop started",
			[]any{daemonLoopRunIDKey, payload.LoopRunID, "loop_name", payload.LoopName},
			func(ctx context.Context, observer loopStartedObserver, payload hookspkg.LoopStartedPayload) error {
				return observer.OnLoopStarted(ctx, payload)
			},
		)
	}
}
