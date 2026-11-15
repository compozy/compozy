package daemon

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"

	"github.com/compozy/compozy/internal/loop/gate"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/toolruntime"
)

func (r loopGateCommandRunner) registerCommand(
	ctx context.Context, req gate.CommandRequest, cmd *exec.Cmd, cancel context.CancelFunc, finished <-chan struct{},
) (*toolruntime.Handle, error) {
	if r.processRegistry == nil {
		return nil, nil
	}
	owner := toolruntime.ProcessOwner{RunID: req.LoopRunID}
	if identity, ok := taskpkg.CoordinatorExecution(ctx); ok {
		owner.RunID = identity.LoopRunID
		owner.ToolCallID = identity.EvaluationID
	}
	return r.processRegistry.Register(ctx, toolruntime.RegisterConfig{
		Source: toolruntime.ProcessSourceSubprocess, Owner: owner,
		PID: cmd.Process.Pid, ProcessGroupID: cmd.Process.Pid,
		Command: "sh", Cwd: cmd.Dir,
		Interrupt: func(stopCtx context.Context, _ toolruntime.ProcessRecord) error {
			cancel()
			select {
			case <-finished:
				return nil
			case <-stopCtx.Done():
				return stopCtx.Err()
			}
		},
	})
}

func (r loopGateCommandRunner) completeCommand(
	ctx context.Context, handle *toolruntime.Handle, runErr error,
) {
	if handle == nil {
		return
	}
	completionCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	defer cancel()
	completion := toolruntime.ProcessCompletion{Err: runErr}
	if exitErr, ok := errors.AsType[*loopGateCommandExitError](runErr); ok {
		completion.ExitCode = new(exitErr.code)
	} else if runErr == nil {
		completion.ExitCode = new(0)
	}
	if err := handle.Complete(completionCtx, completion); err != nil {
		slog.WarnContext(completionCtx, "gate process exited; could not complete process record", "error", err)
	}
}
