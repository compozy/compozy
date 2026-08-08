package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

type systemSSHOwnershipLease struct {
	command   *exec.Cmd
	stdin     io.WriteCloser
	stderr    *bytes.Buffer
	waitDone  chan struct{}
	waitErr   error
	closeOnce sync.Once
	closeErr  error
}

func startSystemSSHOwnershipLease(
	ctx context.Context,
	target sshTarget,
	remoteHome string,
	ownership *sshDaemonOwnership,
) (sshOwnershipLease, error) {
	if ctx == nil {
		return nil, errors.New("cli: SSH ownership lease context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ownership == nil || strings.TrimSpace(ownership.token) == "" {
		return nil, errors.New("cli: SSH daemon ownership token is required")
	}
	args := sshOwnershipLeaseArgs(target, remoteHome, ownership.token)
	//nolint:noctx // The lease outlives command setup and closes through stdin EOF.
	command := exec.Command("ssh", args...)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("cli: open SSH ownership lease stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		closeErr := stdin.Close()
		return nil, errors.Join(fmt.Errorf("cli: open SSH ownership lease stdout: %w", err), closeErr)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	procutil.ConfigureCommandProcessGroup(command)
	if err := command.Start(); err != nil {
		closeErr := stdin.Close()
		return nil, errors.Join(fmt.Errorf("cli: start SSH ownership lease: %w", err), closeErr)
	}
	if err := procutil.RegisterCommandProcessGroup(command); err != nil {
		closeErr := stdin.Close()
		killErr := procutil.KillCommandProcessGroupAndWait(command, sshTunnelCloseTimeout)
		return nil, errors.Join(
			fmt.Errorf("cli: register SSH ownership lease process group: %w", err),
			closeErr,
			killErr,
		)
	}
	lease := &systemSSHOwnershipLease{
		command: command, stdin: stdin, stderr: &stderr, waitDone: make(chan struct{}),
	}
	go lease.waitForExit()
	reader := bufio.NewReader(stdout)
	ready := make(chan struct {
		line string
		err  error
	}, 1)
	go func() {
		line, readErr := reader.ReadString('\n')
		ready <- struct {
			line string
			err  error
		}{line: strings.TrimSpace(line), err: readErr}
	}()
	select {
	case <-ctx.Done():
		return nil, errors.Join(ctx.Err(), lease.Close())
	case result := <-ready:
		if result.err != nil {
			return nil, errors.Join(
				fmt.Errorf("cli: read SSH ownership lease response: %w", result.err),
				lease.Close(),
			)
		}
		switch result.line {
		case sshOwnershipAcquired:
			return lease, nil
		case sshOwnershipBusy:
			return nil, errors.Join(newSSHBusyError(), lease.Close())
		default:
			return nil, errors.Join(errors.New("cli: remote SSH ownership response is invalid"), lease.Close())
		}
	}
}

func sshOwnershipLeaseArgs(target sshTarget, remoteHome, token string) []string {
	args := []string{
		"-o", sshStrictHostKeyOption,
		"-o", "ConnectTimeout=" + strconv.Itoa(sshConnectTimeoutSeconds),
		"-o", "ControlMaster=no",
		"-o", "ControlPath=none",
		"-o", "ControlPersist=no",
	}
	if target.port != 22 {
		args = append(args, "-p", strconv.Itoa(target.port))
	}
	return append(
		args,
		"--", target.destination(),
		shellJoin(sshOwnershipCommand(sshOwnershipLeaseScript, remoteHome, token)),
	)
}

func (l *systemSSHOwnershipLease) waitForExit() {
	l.waitErr = l.command.Wait()
	close(l.waitDone)
}

func (l *systemSSHOwnershipLease) Wait() error {
	<-l.waitDone
	if l.waitErr == nil {
		return nil
	}
	return &sshCommandError{cause: l.waitErr, output: l.stderr.String()}
}

func (l *systemSSHOwnershipLease) Close() error {
	l.closeOnce.Do(func() {
		if closeErr := l.stdin.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			l.closeErr = fmt.Errorf("cli: close SSH ownership lease stdin: %w", closeErr)
		}
		timer := time.NewTimer(sshTunnelCloseTimeout)
		defer timer.Stop()
		select {
		case <-l.waitDone:
			if waitErr := l.Wait(); waitErr != nil && !isExpectedSSHShutdown(waitErr) {
				l.closeErr = errors.Join(l.closeErr, waitErr)
			}
		case <-timer.C:
			signalErr := procutil.SignalCommandProcessGroup(l.command, syscall.SIGTERM)
			killErr := procutil.KillCommandProcessGroupAndWait(l.command, sshTunnelCloseTimeout)
			l.closeErr = errors.Join(l.closeErr, signalErr, killErr)
		}
	})
	return l.closeErr
}
