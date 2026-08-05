package daytona

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/sandbox"
	shellquote "github.com/kballard/go-shellquote"
)

const remoteCommandOutputLimit = 4096

var _ sandbox.CommandRuntime = (*daytonaCommandRuntime)(nil)

type daytonaCommandRuntime struct {
	transport transport
	sandbox   sandboxInfo
}

func (l *daytonaLauncher) CommandRuntime() sandbox.CommandRuntime {
	if l == nil || l.transport == nil {
		return nil
	}
	return &daytonaCommandRuntime{transport: l.transport, sandbox: l.sandbox}
}

func (l *daytonaLauncher) PrepareLaunch(
	ctx context.Context,
	spec sandbox.LaunchSpec,
) (sandbox.LaunchSpec, error) {
	if ctx == nil {
		return spec, errors.New("sandbox/daytona: launch preparation context is required")
	}
	if err := ctx.Err(); err != nil {
		return spec, err
	}
	if spec.ResolvedExecutable != "" {
		if !path.IsAbs(spec.ResolvedExecutable) {
			return spec, errors.New("sandbox/daytona: resolved launch executable must be absolute")
		}
		next := spec
		next.Args = append([]string(nil), spec.Args...)
		return next, nil
	}
	argv, err := shellquote.Split(strings.TrimSpace(spec.Command))
	if err != nil {
		return spec, fmt.Errorf("sandbox/daytona: parse launch command: %w", err)
	}
	if len(argv) == 0 {
		return spec, errors.New("sandbox/daytona: launch command is required")
	}
	runtime := l.CommandRuntime()
	if runtime == nil {
		return spec, errors.New("sandbox/daytona: command runtime is required")
	}
	resolved, err := runtime.Resolve(ctx, argv[0], spec.Env, spec.Cwd)
	if err != nil {
		return spec, fmt.Errorf("sandbox/daytona: prepare remote launch identity: %w", err)
	}
	next := spec
	next.ResolvedExecutable = resolved
	next.Args = append([]string(nil), argv[1:]...)
	return next, nil
}

func (r *daytonaCommandRuntime) Resolve(
	ctx context.Context,
	command string,
	env []string,
	cwd string,
) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", errors.New("sandbox/daytona: remote executable is required")
	}
	payload := "command -v " + shellquote.Join(command)
	result, err := r.execute(ctx, remoteShellCommand(cwd, env, payload))
	if err != nil {
		return "", err
	}
	switch result.ExitCode {
	case 0:
		resolved := strings.TrimSpace(result.Stdout)
		if resolved == "" || strings.ContainsAny(resolved, "\r\n") || !path.IsAbs(resolved) {
			return "", newCommandRuntimeError(
				"validate remote executable resolution",
				errors.New("resolver returned an invalid absolute path"),
			)
		}
		return resolved, nil
	case 127:
		return "", fmt.Errorf("sandbox/daytona: remote executable not found: %w", exec.ErrNotFound)
	default:
		return "", newCommandRuntimeError(
			"resolve remote executable",
			fmt.Errorf("resolver exited with code %d", result.ExitCode),
		)
	}
}

func (r *daytonaCommandRuntime) Run(
	ctx context.Context,
	spec sandbox.CommandSpec,
) (sandbox.CommandResult, error) {
	if !path.IsAbs(spec.Executable) {
		return sandbox.CommandResult{}, errors.New(
			"sandbox/daytona: remote command executable must be absolute",
		)
	}
	command := remoteExecutableCommand(spec.Executable, spec.Args)
	return r.execute(ctx, remoteShellCommand(spec.Cwd, spec.Env, command))
}

func (r *daytonaCommandRuntime) execute(
	ctx context.Context,
	command string,
) (sandbox.CommandResult, error) {
	if ctx == nil {
		return sandbox.CommandResult{}, errors.New("sandbox/daytona: remote command context is required")
	}
	if r == nil || r.transport == nil {
		return sandbox.CommandResult{}, errors.New("sandbox/daytona: remote command transport is required")
	}
	startedAt := time.Now()
	session, err := r.transport.Dial(ctx, r.sandbox, command)
	if err != nil {
		return sandbox.CommandResult{}, newCommandRuntimeError("dial remote command", err)
	}
	if session == nil {
		return sandbox.CommandResult{}, errors.New("sandbox/daytona: remote command session is required")
	}
	if err := normalizeFinishedStreamError(session.CloseWrite()); err != nil {
		cleanupErr := stopCommandSession(ctx, session)
		return sandbox.CommandResult{}, newCommandRuntimeError(
			"close remote command input",
			errors.Join(err, cleanupErr),
		)
	}

	output := &boundedCommandOutput{limit: remoteCommandOutputLimit}
	readDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(output, session)
		readDone <- copyErr
	}()

	select {
	case readErr := <-readDone:
		return finishCommandSession(ctx, session, output.String(), startedAt, readErr)
	case <-ctx.Done():
		stopErr := stopCommandSession(ctx, session)
		readErr := <-readDone
		cause := context.Cause(ctx)
		if cause == nil {
			cause = ctx.Err()
		}
		cleanupErr := errors.Join(readErr, stopErr, closeFinishedSession(session))
		if cleanupErr == nil {
			return sandbox.CommandResult{}, cause
		}
		return sandbox.CommandResult{}, errors.Join(
			cause,
			newCommandRuntimeError("stop canceled remote command", cleanupErr),
		)
	}
}

func finishCommandSession(
	ctx context.Context,
	session transportSession,
	stdout string,
	startedAt time.Time,
	readErr error,
) (sandbox.CommandResult, error) {
	if readErr != nil {
		stopErr := stopCommandSession(ctx, session)
		return sandbox.CommandResult{}, newCommandRuntimeError(
			"read remote command output",
			errors.Join(readErr, stopErr, closeFinishedSession(session)),
		)
	}
	waitErr := session.Wait()
	exitCode, hasExitCode := session.ExitCode()
	result := sandbox.CommandResult{
		ExitCode: exitCode,
		Stdout:   diagnostics.RedactAndBound(stdout, remoteCommandOutputLimit),
		Stderr:   diagnostics.RedactAndBound(session.Stderr(), remoteCommandOutputLimit),
		Duration: time.Since(startedAt),
	}
	closeErr := closeFinishedSession(session)
	if closeErr != nil {
		return result, newCommandRuntimeError("close completed remote command", closeErr)
	}
	if !hasExitCode {
		if waitErr == nil {
			waitErr = errors.New("remote command exit status is missing")
		}
		return result, newCommandRuntimeError("wait for remote command", waitErr)
	}
	if exitCode == 0 && waitErr != nil {
		return result, newCommandRuntimeError("wait for remote command", waitErr)
	}
	return result, nil
}

func stopCommandSession(ctx context.Context, session transportSession) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), sidecarCloseTimeout)
	defer cancel()
	return session.Stop(stopCtx)
}

func closeFinishedSession(session transportSession) error {
	return normalizeFinishedStreamError(session.Close())
}

func normalizeFinishedStreamError(err error) error {
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

type boundedCommandOutput struct {
	buffer bytes.Buffer
	limit  int
}

func (w *boundedCommandOutput) Write(payload []byte) (int, error) {
	written := len(payload)
	remaining := w.limit - w.buffer.Len()
	if remaining <= 0 {
		return written, nil
	}
	if len(payload) > remaining {
		payload = payload[:remaining]
	}
	_, err := w.buffer.Write(payload)
	return written, err
}

func (w *boundedCommandOutput) String() string {
	return w.buffer.String()
}

type commandRuntimeError struct {
	operation string
	cause     error
}

func newCommandRuntimeError(operation string, cause error) error {
	if cause == nil {
		return nil
	}
	return &commandRuntimeError{operation: operation, cause: cause}
}

func (e *commandRuntimeError) Error() string {
	detail := diagnostics.RedactAndBound(e.cause.Error(), 1024)
	if detail == "" {
		return "sandbox/daytona: " + e.operation
	}
	return "sandbox/daytona: " + e.operation + ": " + detail
}

func (e *commandRuntimeError) Unwrap() error {
	return e.cause
}
