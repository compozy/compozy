package session

import (
	"context"
	"errors"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/store"
)

const recoveredStopPollInterval = 100 * time.Millisecond

func recoveredTerminationTarget(meta *store.SessionMeta) (*AgentProcess, terminationTarget) {
	proc := NewAgentProcess(AgentProcessOptions{Done: make(chan struct{})})
	if meta.Liveness != nil {
		proc.PID = meta.Liveness.SubprocessPID
		if meta.Liveness.SubprocessStartedAt != nil {
			proc.StartedAt = *meta.Liveness.SubprocessStartedAt
		}
	}
	verify := func(proc *AgentProcess) (bool, error) {
		if recoveredProcessRequiresRemoteProof(meta) {
			return false, errors.New("session: recovered remote process requires remote exit proof")
		}
		return procutil.VerifyProcessExit(proc.PID, proc.StartedAt)
	}
	return proc, terminationTarget{
		cooperative: func(context.Context, *AgentProcess) error { return nil },
		forced:      recoveredProcessSignal(syscall.SIGTERM, verify, procutil.SignalProcessGroupID),
		kill:        recoveredProcessSignal(syscall.SIGKILL, verify, procutil.SignalProcessGroupID),
		verifyExit:  verify,
	}
}

func recoveredProcessSignal(
	signal syscall.Signal, verify func(*AgentProcess) (bool, error),
	signalProcessGroup func(int, syscall.Signal) error,
) func(context.Context, *AgentProcess) error {
	return func(ctx context.Context, proc *AgentProcess) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		exited, err := verify(proc)
		if err != nil || exited {
			return err
		}
		if err := signalProcessGroup(proc.PID, signal); err != nil {
			return err
		}
		ticker := time.NewTicker(recoveredStopPollInterval)
		defer ticker.Stop()
		for {
			exited, err := verify(proc)
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

func recoveredProcessRequiresRemoteProof(meta *store.SessionMeta) bool {
	return meta.Sandbox != nil && meta.Sandbox.Backend != "local"
}
