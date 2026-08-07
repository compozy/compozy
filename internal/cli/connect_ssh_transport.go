package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

const sshTunnelCloseTimeout = 5 * time.Second

type sshTarget struct {
	host string
	user string
	port int
}

func (t sshTarget) destination() string {
	if t.user == "" {
		return t.host
	}
	return t.user + "@" + t.host
}

type sshExecutor interface {
	Run(context.Context, sshTarget, []string) ([]byte, error)
	StartOwnershipLease(context.Context, sshTarget, string, *sshDaemonOwnership) (sshOwnershipLease, error)
	StartTunnel(context.Context, sshTarget, string, int) (sshTunnel, error)
}

type sshOwnershipLease interface {
	Wait() error
	Close() error
}

type sshTunnel interface {
	LocalPort() int
	Wait() error
	Close() error
}

type systemSSHExecutor struct{}

func (systemSSHExecutor) StartOwnershipLease(
	ctx context.Context,
	target sshTarget,
	remoteHome string,
	ownership *sshDaemonOwnership,
) (sshOwnershipLease, error) {
	return startSystemSSHOwnershipLease(ctx, target, remoteHome, ownership)
}

func (systemSSHExecutor) Run(ctx context.Context, target sshTarget, remoteCommand []string) ([]byte, error) {
	if len(remoteCommand) == 0 {
		return nil, errors.New("cli: SSH remote command is required")
	}
	args := sshRunArgs(target, remoteCommand)
	command := exec.CommandContext(ctx, "ssh", args...)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, &sshCommandError{cause: err, output: string(output)}
	}
	return output, nil
}

func (systemSSHExecutor) StartTunnel(
	ctx context.Context,
	target sshTarget,
	remoteHost string,
	remotePort int,
) (sshTunnel, error) {
	if ctx == nil {
		return nil, errors.New("cli: SSH tunnel context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	localPort, err := reserveLoopbackPort(ctx)
	if err != nil {
		return nil, err
	}
	args := sshTunnelArgs(target, localPort, remoteHost, remotePort)
	guardianArgs := append(
		[]string{"-c", sshTunnelGuardianScript, "compozy-ssh-tunnel", clientSSHProtocol},
		args...,
	)
	//nolint:noctx // The tunnel owns its lifecycle; CommandContext races process-group teardown in Close.
	command := exec.Command("sh", guardianArgs...)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("cli: open SSH tunnel guardian stdin: %w", err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	procutil.ConfigureCommandProcessGroup(command)
	if err := command.Start(); err != nil {
		closeErr := stdin.Close()
		return nil, errors.Join(fmt.Errorf("cli: start SSH tunnel: %w", err), closeErr)
	}
	if err := procutil.RegisterCommandProcessGroup(command); err != nil {
		closeErr := stdin.Close()
		killErr := procutil.KillCommandProcessGroupAndWait(command, sshTunnelCloseTimeout)
		return nil, errors.Join(fmt.Errorf("cli: register SSH tunnel process group: %w", err), closeErr, killErr)
	}
	tunnel := &systemSSHTunnel{
		command: command, stdin: stdin, localPort: localPort, stderr: &stderr, waitDone: make(chan struct{}),
	}
	go tunnel.waitForExit()
	return tunnel, nil
}

func sshRunArgs(target sshTarget, remoteCommand []string) []string {
	return append(sshBaseArgs(target), "--", target.destination(), shellJoin(remoteCommand))
}

func sshTunnelArgs(target sshTarget, localPort int, remoteHost string, remotePort int) []string {
	forward := net.JoinHostPort("127.0.0.1", strconv.Itoa(localPort)) + ":" +
		net.JoinHostPort(strings.Trim(remoteHost, "[]"), strconv.Itoa(remotePort))
	return append(
		sshBaseArgs(target),
		"-o", "ExitOnForwardFailure=yes", "-N", "-L", forward, "--", target.destination(),
	)
}

func sshBaseArgs(target sshTarget) []string {
	args := []string{
		"-o", sshStrictHostKeyOption,
		"-o", "ControlMaster=auto",
		"-o", "ControlPersist=60",
	}
	if target.port != 22 {
		args = append(args, "-p", strconv.Itoa(target.port))
	}
	return args
}

func reserveLoopbackPort(ctx context.Context) (port int, err error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("cli: reserve SSH loopback port: %w", err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: release SSH loopback port reservation: %w", closeErr))
		}
	}()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.Port <= 0 {
		return 0, errors.New("cli: resolve SSH loopback port")
	}
	return address.Port, nil
}

type systemSSHTunnel struct {
	command   *exec.Cmd
	stdin     io.WriteCloser
	localPort int
	stderr    *bytes.Buffer
	waitDone  chan struct{}
	waitErr   error
	closeOnce sync.Once
	closeErr  error
}

func (t *systemSSHTunnel) LocalPort() int { return t.localPort }

func (t *systemSSHTunnel) waitForExit() {
	t.waitErr = t.command.Wait()
	close(t.waitDone)
}

func (t *systemSSHTunnel) Wait() error {
	<-t.waitDone
	if t.waitErr == nil {
		return nil
	}
	return &sshCommandError{cause: t.waitErr, output: t.stderr.String()}
}

func (t *systemSSHTunnel) Close() error {
	t.closeOnce.Do(func() {
		if closeErr := t.stdin.Close(); closeErr != nil && !errors.Is(closeErr, os.ErrClosed) {
			t.closeErr = fmt.Errorf("cli: close SSH tunnel guardian stdin: %w", closeErr)
		}
		select {
		case <-t.waitDone:
			if waitErr := t.Wait(); waitErr != nil && !isExpectedSSHShutdown(waitErr) {
				t.closeErr = errors.Join(t.closeErr, waitErr)
			}
			return
		default:
		}
		timer := time.NewTimer(sshTunnelCloseTimeout)
		defer timer.Stop()
		select {
		case <-t.waitDone:
			if waitErr := t.Wait(); waitErr != nil && !isExpectedSSHShutdown(waitErr) {
				t.closeErr = errors.Join(t.closeErr, waitErr)
				return
			}
		case <-timer.C:
			signalErr := procutil.SignalCommandProcessGroup(t.command, syscall.SIGTERM)
			killErr := procutil.KillCommandProcessGroupAndWait(t.command, sshTunnelCloseTimeout)
			t.closeErr = errors.Join(t.closeErr, signalErr, killErr)
		}
	})
	return t.closeErr
}

const sshTunnelGuardianScript = `
set -u
exec 3<&0
"$@" </dev/null &
ssh_pid=$!
(
  cat <&3 >/dev/null
  kill -TERM "$ssh_pid" 2>/dev/null || true
) &
guard_pid=$!
set +e
wait "$ssh_pid"
status=$?
kill -TERM "$guard_pid" 2>/dev/null || true
wait "$guard_pid" 2>/dev/null || true
exit "$status"
`

func isExpectedSSHShutdown(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) || errors.Is(err, context.Canceled)
}

type sshCommandError struct {
	cause  error
	output string
}

func (e *sshCommandError) Error() string {
	if e == nil {
		return "SSH command failed"
	}
	message := strings.TrimSpace(e.output)
	if message == "" {
		return fmt.Sprintf("SSH command failed: %v", e.cause)
	}
	return "SSH command failed: " + message
}

func (e *sshCommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *sshCommandError) exitCode() int {
	var exitErr *exec.ExitError
	if errors.As(e.cause, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func shellJoin(arguments []string) string {
	quoted := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		quoted = append(quoted, shellQuote(argument))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
