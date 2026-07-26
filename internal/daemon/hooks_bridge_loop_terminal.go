package daemon

import (
	"context"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

const loopHookObserverTimeout = 10 * time.Second

type loopTerminalObserver interface {
	OnLoopTerminal(context.Context, hookspkg.LoopTerminalPayload) error
}

func (n *hooksNotifier) AddLoopTerminalObserver(observer loopTerminalObserver) {
	if n == nil || observer == nil {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.loopTerminalHooks = append(n.loopTerminalHooks, observer)
}

func (n *hooksNotifier) loopTerminalObservers() []loopTerminalObserver {
	if n == nil {
		return nil
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	return append([]loopTerminalObserver(nil), n.loopTerminalHooks...)
}

func (n *hooksNotifier) notifyLoopTerminalObservers(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
) {
	for _, observer := range n.loopTerminalObservers() {
		notifyObserver(
			ctx,
			n,
			observer,
			payload,
			"loop terminal",
			[]any{
				daemonLoopRunIDKey, payload.LoopRunID,
				"parent_loop_run_id", payload.ParentLoopRunID,
				"loop_name", payload.LoopName,
			},
			func(ctx context.Context, observer loopTerminalObserver, payload hookspkg.LoopTerminalPayload) error {
				return observer.OnLoopTerminal(ctx, payload)
			},
		)
	}
}

func loopObserverContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(context.WithoutCancel(ctx), loopHookObserverTimeout)
}
