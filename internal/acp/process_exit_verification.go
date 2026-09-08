package acp

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

// VerifyExit distinguishes confirmed process exit from a failed status lookup.
func (d *Driver) VerifyExit(proc *AgentProcess) (bool, error) {
	if proc == nil {
		return false, errors.New("acp: process exit verification requires a process")
	}
	if proc.PID > 0 {
		return procutil.VerifyProcessExit(proc.PID, proc.StartedAt)
	}
	// Remote launchers do not expose a host PID. Only a received remote exit
	// status proves termination; a transport disconnect by itself does not.
	if proc.handle != nil {
		_, confirmed := proc.ExitStatus()
		return confirmed, nil
	}
	return false, errors.New("acp: process identity is unavailable for exit verification")
}

func (d *Driver) verifyStoppedProcess(proc *AgentProcess) error {
	verified, err := d.VerifyExit(proc)
	if err != nil {
		return err
	}
	if !verified {
		return errors.New("acp: process exit could not be verified")
	}
	return nil
}

// Kill attempts the final process-tree termination phase independently of a canceled request.
func (d *Driver) Kill(ctx context.Context, proc *AgentProcess) error {
	if ctx == nil || proc == nil {
		return errors.New("acp: kill requires context and process")
	}
	verified, err := d.VerifyExit(proc)
	if err != nil || verified {
		return err
	}
	proc.markStopRequested()
	if proc.PID > 0 {
		return procutil.KillProcessGroupIDAndWait(proc.PID, time.Second)
	}
	if proc.handle != nil {
		return proc.handle.Stop(ctx)
	}
	return errors.New("acp: process owner cannot force termination")
}
