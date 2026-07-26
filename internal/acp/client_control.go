package acp

import (
	"context"
	"errors"
	"fmt"

	"strings"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/agh/internal/toolruntime"
)

// Prompt starts one prompt turn and returns the streamed event channel.
func (d *Driver) Prompt(ctx context.Context, proc *AgentProcess, req PromptRequest) (<-chan AgentEvent, error) {
	if ctx == nil {
		return nil, errors.New("acp: context is required")
	}
	if proc == nil {
		return nil, errors.New("acp: agent process is required")
	}
	if proc.conn == nil {
		return nil, errProcessConnectionUninitialized
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	promptCtx, cancelPrompt := context.WithCancel(ctx)
	active, err := proc.beginPrompt(req.TurnID, d.promptBufferCap, cancelPrompt)
	if err != nil {
		cancelPrompt()
		return nil, err
	}

	go d.runPrompt(promptCtx, proc, active, req)
	return active.events, nil
}

// Cancel cancels the local active prompt or falls back to ACP cooperative cancellation.
func (d *Driver) Cancel(ctx context.Context, proc *AgentProcess) error {
	if ctx == nil {
		return errors.New("acp: context is required")
	}
	if proc == nil {
		return errors.New("acp: agent process is required")
	}
	if proc.conn == nil {
		return errProcessConnectionUninitialized
	}
	if strings.TrimSpace(proc.SessionID) == "" {
		return errors.New("acp: session id is required")
	}
	if proc.cancelCurrentPrompt() {
		return nil
	}
	return proc.conn.SendNotification(ctx, acpsdk.AgentMethodSessionCancel, acpsdk.CancelNotification{
		SessionId: acpsdk.SessionId(proc.SessionID),
	})
}

// Interrupt signals processes matching a scoped toolruntime selector.
func (d *Driver) Interrupt(
	ctx context.Context,
	scope toolruntime.InterruptScope,
) (toolruntime.InterruptReport, error) {
	if ctx == nil {
		return toolruntime.InterruptReport{}, errors.New("acp: interrupt context is required")
	}
	if d == nil || d.processRegistry == nil {
		return toolruntime.InterruptReport{}, toolruntime.ErrProcessNotFound
	}
	return d.processRegistry.Interrupt(ctx, scope)
}

// ApprovePermission resolves a pending interactive permission request for the process.
func (d *Driver) ApprovePermission(ctx context.Context, proc *AgentProcess, req ApproveRequest) error {
	if ctx == nil {
		return errors.New("acp: context is required")
	}
	if proc == nil {
		return errors.New("acp: agent process is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return proc.ResolvePermission(req)
}

// Stop terminates the subprocess and waits for it to exit.
func (d *Driver) Stop(ctx context.Context, proc *AgentProcess) error {
	if ctx == nil {
		return errors.New("acp: context is required")
	}
	if proc == nil {
		return errors.New("acp: agent process is required")
	}
	if proc.done == nil {
		return errProcessLifecycleUninitialized
	}

	select {
	case <-proc.Done():
		return proc.Wait()
	default:
	}
	if proc.handle == nil && proc.managed == nil && proc.cmd == nil {
		return errProcessLifecycleUninitialized
	}

	proc.markStopRequested()
	if proc.processRecord != nil {
		recordCtx, cancelRecord := processRecordContext(ctx, d.processRecordTimeout)
		err := proc.processRecord.Checkpoint(recordCtx, toolruntime.ProcessCheckpoint{
			State: toolruntime.ProcessStateInterrupting,
			Error: "ACP stop requested",
		})
		cancelRecord()
		if err != nil && d.logger != nil {
			d.logger.Warn("acp: checkpoint process interrupt", "pid", proc.PID, "error", err)
		}
	}
	errs := d.cancelSessionForStop(ctx, proc)
	if proc.handle != nil {
		return stopAgentProcessAndWait(ctx, proc, errs, proc.handle.Stop)
	}
	if proc.managed != nil {
		return stopAgentProcessAndWait(ctx, proc, errs, proc.managed.Shutdown)
	}

	return d.stopExecCommand(ctx, proc, errs)
}

func (d *Driver) cancelSessionForStop(ctx context.Context, proc *AgentProcess) []error {
	if strings.TrimSpace(proc.SessionID) == "" {
		return nil
	}
	cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()

	if err := d.Cancel(cancelCtx, proc); err != nil && !errors.Is(err, context.Canceled) {
		return []error{fmt.Errorf("acp: cancel session prompt: %w", err)}
	}
	return nil
}

func stopAgentProcessAndWait(
	ctx context.Context,
	proc *AgentProcess,
	errs []error,
	stopFn func(context.Context) error,
) error {
	if err := stopFn(ctx); err != nil {
		errs = append(errs, err)
	}
	select {
	case <-proc.Done():
		return errors.Join(append(errs, proc.Wait())...)
	case <-ctx.Done():
		return errors.Join(append(errs, ctx.Err())...)
	}
}

func (d *Driver) stopExecCommand(ctx context.Context, proc *AgentProcess, errs []error) error {
	if err := terminateManagedProcess(proc.cmd); err != nil {
		errs = append(errs, fmt.Errorf("acp: terminate subprocess tree: %w", err))
	}
	if proc.cancelProcess != nil {
		proc.cancelProcess()
	}

	waitCtx, cancelWait := context.WithTimeout(context.Background(), d.stopTimeout)
	defer cancelWait()

	select {
	case <-proc.Done():
	case <-waitCtx.Done():
		if err := killManagedProcess(proc.cmd); err != nil {
			errs = append(errs, fmt.Errorf("acp: kill subprocess tree: %w", err))
		}
		select {
		case <-proc.Done():
		case <-ctx.Done():
			return errors.Join(append(errs, ctx.Err())...)
		}
	case <-ctx.Done():
		return errors.Join(append(errs, ctx.Err())...)
	}

	return errors.Join(append(errs, proc.Wait())...)
}
