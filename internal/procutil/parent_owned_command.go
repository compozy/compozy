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
		done <- errors.Join(cmd.Wait(), releaseParentOwnedCommand(cmd))
	}
}

func StopParentOwnedCommand(cmd *exec.Cmd, done <-chan error) error {
	select {
	case <-done:
		return nil
	default:
	}
	signalErr := SignalCommandProcessGroup(cmd, syscall.SIGTERM)
	timer := time.NewTimer(ownedProcessGrace)
	defer timer.Stop()
	select {
	case <-done:
		return signalErr
	case <-timer.C:
	}
	killErr := KillCommandProcessGroupAndWait(cmd, ownedProcessGrace)
	timer.Reset(ownedProcessGrace)
	select {
	case <-done:
		return errors.Join(signalErr, killErr)
	case <-timer.C:
		return errors.Join(signalErr, killErr, errors.New("timed out reaping connector"))
	}
}
