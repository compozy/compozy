package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	exec "os/exec"

	"strings"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/toolruntime"
)

func runSubprocessCommand(
	ctx context.Context,
	cmd *exec.Cmd,
	hook RegisteredHook,
	payload []byte,
	registry *toolruntime.Registry,
	registryTimeout time.Duration,
) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		return errors.Join(err, cleanupStartedSubprocessCommand(cmd))
	}

	record, err := registerSubprocessHook(ctx, cmd, hook, payload, registry)
	if err != nil {
		return errors.Join(err, cleanupStartedSubprocessCommand(cmd))
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case err := <-waitCh:
		return errors.Join(err, completeSubprocessHookDetached(ctx, registryTimeout, record, cmd, err))
	case <-ctx.Done():
		checkpointErr := checkpointSubprocessHookDetached(
			ctx,
			registryTimeout,
			record,
			toolruntime.ProcessStateInterrupting,
			ctx.Err().Error(),
		)
		shutdownErr := shutdownSubprocessCommand(cmd, waitCh)
		completeErr := completeSubprocessHookDetached(ctx, registryTimeout, record, cmd, shutdownErr)
		return errors.Join(checkpointErr, shutdownErr, completeErr)
	}
}

func checkpointSubprocessHookDetached(
	ctx context.Context,
	timeout time.Duration,
	record *toolruntime.Handle,
	state toolruntime.ProcessState,
	reason string,
) error {
	registryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	return checkpointSubprocessHook(registryCtx, record, state, reason)
}

func completeSubprocessHookDetached(
	ctx context.Context,
	timeout time.Duration,
	record *toolruntime.Handle,
	cmd *exec.Cmd,
	err error,
) error {
	registryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	return completeSubprocessHook(registryCtx, record, cmd, err)
}

func cleanupStartedSubprocessCommand(cmd *exec.Cmd) error {
	if cmd == nil {
		return nil
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	return shutdownSubprocessCommand(cmd, waitCh)
}

func shutdownSubprocessCommand(cmd *exec.Cmd, waitCh <-chan error) error {
	terminateErr := terminateSubprocessCommand(cmd)
	timer := time.NewTimer(subprocessShutdownGrace)
	defer timer.Stop()
	select {
	case waitErr := <-waitCh:
		return errors.Join(terminateErr, waitErr, forceSubprocessCommandExit(cmd, subprocessProcessGroupWait))
	case <-timer.C:
		groupErr := forceSubprocessCommandExit(cmd, subprocessProcessGroupWait)
		waitErr := waitForSubprocessCommand(waitCh, subprocessProcessGroupWait)
		return errors.Join(
			terminateErr,
			groupErr,
			waitErr,
		)
	}
}

func waitForSubprocessCommand(waitCh <-chan error, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case waitErr := <-waitCh:
		return waitErr
	case <-timer.C:
		return fmt.Errorf(
			"hooks: wait for subprocess exit after force kill: %w",
			context.DeadlineExceeded,
		)
	}
}

func registerSubprocessHook(
	ctx context.Context,
	cmd *exec.Cmd,
	hook RegisteredHook,
	payload []byte,
	registry *toolruntime.Registry,
) (*toolruntime.Handle, error) {
	if registry == nil || cmd == nil || cmd.Process == nil {
		return nil, nil
	}
	owner := subprocessHookOwner(hook, payload)
	command := cmd.Path
	args := []string(nil)
	if len(cmd.Args) > 0 {
		command = cmd.Args[0]
		args = cmd.Args[1:]
	}
	return registry.Register(ctx, toolruntime.RegisterConfig{
		Source:         toolruntime.ProcessSourceHook,
		Owner:          owner,
		PID:            cmd.Process.Pid,
		ProcessGroupID: cmd.Process.Pid,
		Command:        command,
		Args:           args,
		Cwd:            cmd.Dir,
		Interrupt: func(_ context.Context, _ toolruntime.ProcessRecord) error {
			return terminateSubprocessCommand(cmd)
		},
	})
}

func subprocessHookOwner(hook RegisteredHook, payload []byte) toolruntime.ProcessOwner {
	owner := toolruntime.ProcessOwner{HookName: strings.TrimSpace(hook.Name)}
	var contextPayload struct {
		SessionID  string `json:"session_id"`
		TurnID     string `json:"turn_id"`
		ToolCallID string `json:"tool_call_id"`
		SandboxID  string `json:"sandbox_id"`
	}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &contextPayload); err != nil {
			return owner
		}
	}
	owner.SessionID = contextPayload.SessionID
	owner.TurnID = contextPayload.TurnID
	owner.ToolCallID = contextPayload.ToolCallID
	owner.SandboxID = contextPayload.SandboxID
	return owner
}

func checkpointSubprocessHook(
	ctx context.Context,
	record *toolruntime.Handle,
	state toolruntime.ProcessState,
	reason string,
) error {
	if record == nil {
		return nil
	}
	return record.Checkpoint(ctx, toolruntime.ProcessCheckpoint{
		State: state,
		Error: reason,
	})
}

func completeSubprocessHook(
	ctx context.Context,
	record *toolruntime.Handle,
	cmd *exec.Cmd,
	err error,
) error {
	if record == nil {
		return nil
	}
	completion := toolruntime.ProcessCompletion{Err: err}
	if cmd != nil && cmd.ProcessState != nil {
		exitCode := cmd.ProcessState.ExitCode()
		if exitCode >= 0 {
			completion.ExitCode = &exitCode
		}
	}
	return record.Complete(ctx, completion)
}
