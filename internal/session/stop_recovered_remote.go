package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/sandbox"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) recoveredTerminationTarget(meta *store.SessionMeta) (*AgentProcess, terminationTarget) {
	proc, target := recoveredTerminationTarget(meta)
	if !recoveredProcessRequiresRemoteProof(meta) {
		return proc, target
	}
	state := sessionSandboxStateFromMeta(meta.Sandbox)
	var controller sandbox.ProcessController
	var controllerErr error
	if m.sandbox == nil {
		controllerErr = errors.New("session: remote process provider is unavailable")
	} else {
		provider, err := m.sandbox.Provider(state.Backend)
		controllerErr = err
		if err == nil {
			var ok bool
			controller, ok = provider.(sandbox.ProcessController)
			if !ok {
				controllerErr = errors.New("session: remote provider does not support process recovery")
			}
		}
	}
	verify := func(ctx context.Context, _ *AgentProcess) (bool, error) {
		if controllerErr != nil {
			return false, controllerErr
		}
		return controller.ProcessExitVerified(ctx, state)
	}
	signal := func(action sandbox.ProcessSignal) func(context.Context, *AgentProcess) error {
		return func(ctx context.Context, proc *AgentProcess) error {
			exited, err := verify(ctx, proc)
			if err != nil || exited {
				return err
			}
			if err := controller.SignalProcess(ctx, state, action); err != nil {
				return err
			}
			ticker := time.NewTicker(recoveredStopPollInterval)
			defer ticker.Stop()
			for {
				exited, err := verify(ctx, proc)
				if err != nil || exited {
					return err
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
				}
			}
		}
	}
	return proc, terminationTarget{
		cooperative: signal(sandbox.ProcessSignalCloseInput), forced: signal(sandbox.ProcessSignalTerminate),
		kill: signal(sandbox.ProcessSignalKill), verifyExitContext: verify,
	}
}
