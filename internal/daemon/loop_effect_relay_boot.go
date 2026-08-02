package daemon

import (
	"context"
	"errors"
	"fmt"
)

func (d *Daemon) bootLoopEffectRelay(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state == nil || state.registry == nil {
		return nil
	}
	store, ok := state.registry.(loopEffectStore)
	if !ok {
		return nil
	}
	tools, ok := state.toolRegistry.(loopEffectToolCaller)
	if !ok || tools == nil {
		return errors.New("daemon: tool registry does not implement loop effect calls")
	}
	relayCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	relay := &loopEffectRelay{store: store, tools: tools, logger: state.logger, now: d.now}
	if state.notifier != nil {
		state.notifier.AddTaskRunTerminalObserver(relay)
		state.notifier.AddLoopTerminalObserver(relay)
	}
	go func() {
		defer close(done)
		relay.Run(relayCtx)
	}()
	state.effectRelayCancel = cancel
	state.effectRelayDone = done
	cleanup.add(func(cleanupCtx context.Context) error {
		return stopLoopEffectRelay(cleanupCtx, cancel, done)
	})
	return nil
}

func stopLoopEffectRelay(
	ctx context.Context,
	cancel context.CancelFunc,
	done <-chan struct{},
) error {
	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: wait for loop effect relay: %w", ctx.Err())
	}
}
