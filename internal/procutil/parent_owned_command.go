package procutil

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

const ownedProcessGrace = 2 * time.Second

// StartParentOwnedCommand runs a process group tied to the daemon lifetime where the OS supports it.
func StartParentOwnedCommand(cmd *exec.Cmd) (<-chan error, error) {
	started := make(chan error, 1)
	done := make(chan error, 1)
	go waitParentOwnedCommand(cmd, started, done)
	if err := <-started; err != nil {
		return nil, err
	}
	return done, nil
}

func waitParentOwnedCommand(cmd *exec.Cmd, started, done chan<- error) {
	defer close(done)
	unpin := pinParentCommandThread(cmd)
	defer unpin()
	err := startParentOwnedCommand(cmd)
	started <- err
	if err == nil {
		done <- errors.Join(waitParentOwnedExit(cmd), releaseParentOwnedCommand(cmd))
	}
}

func waitParentOwnedExit(cmd *exec.Cmd) error {
	err := cmd.Wait()
	// An exit status confirms the child was reaped, including after a shutdown signal.
	if _, exited := errors.AsType[*exec.ExitError](err); exited {
		return nil
	}
	return err
}

func StopParentOwnedCommand(cmd *exec.Cmd, done <-chan error) error {
	select {
	case err := <-done:
		return err
	default:
	}
	signalErr := SignalCommandProcessGroup(cmd, syscall.SIGTERM)
	timer := time.NewTimer(ownedProcessGrace)
	defer timer.Stop()
	select {
	case err := <-done:
		return errors.Join(signalErr, err)
	case <-timer.C:
	}
	killErr := KillCommandProcessGroupAndWait(cmd, ownedProcessGrace)
	timer.Reset(ownedProcessGrace)
	select {
	case err := <-done:
		return errors.Join(signalErr, killErr, err)
	case <-timer.C:
		return errors.Join(signalErr, killErr, errors.New("timed out reaping connector"))
	}
}
