package providers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/compozy/agh/internal/diagnostics"
	"github.com/kballard/go-shellquote"
	"golang.org/x/term"
)

const DefaultProviderAuthCommandTimeout = 30 * time.Second

// ProviderAuthCommandRunner executes a provider-owned auth command.
type ProviderAuthCommandRunner func(context.Context, ProviderAuthCommandSpec) (ProviderAuthCommandResult, error)

// ProviderAuthCommandSpec describes one provider-owned auth command execution.
type ProviderAuthCommandSpec struct {
	Command string
	Env     []string
	Timeout time.Duration
	NoTTY   bool
}

// ProviderAuthCommandResult is a redacted provider auth command result.
type ProviderAuthCommandResult struct {
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// DefaultProviderAuthCommandRunner runs a non-interactive auth status command.
func DefaultProviderAuthCommandRunner(
	ctx context.Context,
	spec ProviderAuthCommandSpec,
) (ProviderAuthCommandResult, error) {
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = DefaultProviderAuthCommandTimeout
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startedAt := time.Now()
	execCmd, err := commandContext(commandCtx, spec)
	if err != nil {
		return ProviderAuthCommandResult{}, err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	err = execCmd.Run()
	result := ProviderAuthCommandResult{
		ExitCode:   exitCodeFromError(err),
		Stdout:     diagnostics.RedactAndBound(stdout.String(), 4096),
		Stderr:     diagnostics.RedactAndBound(stderr.String(), 4096),
		DurationMs: time.Since(startedAt).Milliseconds(),
	}
	if commandCtx.Err() != nil {
		return result, commandCtx.Err()
	}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr != nil {
		return result, nil
	}
	return result, fmt.Errorf("provider auth: run command: %w", err)
}

// DefaultProviderAuthLoginRunner runs an operator-facing auth login command.
func DefaultProviderAuthLoginRunner(
	ctx context.Context,
	spec ProviderAuthCommandSpec,
) (ProviderAuthCommandResult, error) {
	commandCtx := ctx
	cancel := func() {}
	if spec.Timeout > 0 {
		commandCtx, cancel = context.WithTimeout(ctx, spec.Timeout)
	}
	defer cancel()

	startedAt := time.Now()
	execCmd, err := commandContext(commandCtx, spec)
	if err != nil {
		return ProviderAuthCommandResult{}, err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	attachTTY := !spec.NoTTY &&
		term.IsTerminal(int(os.Stdin.Fd())) &&
		term.IsTerminal(int(os.Stdout.Fd())) &&
		term.IsTerminal(int(os.Stderr.Fd()))
	if attachTTY {
		execCmd.Stdin = os.Stdin
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
	} else {
		execCmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		execCmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	}
	err = execCmd.Run()
	result := ProviderAuthCommandResult{
		ExitCode:   exitCodeFromError(err),
		Stdout:     diagnostics.RedactAndBound(stdout.String(), 4096),
		Stderr:     diagnostics.RedactAndBound(stderr.String(), 4096),
		DurationMs: time.Since(startedAt).Milliseconds(),
	}
	if commandCtx.Err() != nil {
		return result, commandCtx.Err()
	}
	if err == nil {
		return result, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr != nil {
		return result, nil
	}
	return result, fmt.Errorf("provider auth: run login command: %w", err)
}

func commandContext(ctx context.Context, spec ProviderAuthCommandSpec) (*exec.Cmd, error) {
	command := strings.TrimSpace(spec.Command)
	if command == "" {
		return nil, errors.New("provider auth command is required")
	}
	argv, err := shellquote.Split(command)
	if err != nil {
		return nil, fmt.Errorf("parse provider auth command: %w", err)
	}
	if len(argv) == 0 {
		return nil, errors.New("provider auth command is empty")
	}
	// #nosec G204 -- Provider auth commands are explicit operator config.
	execCmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	execCmd.Env = append([]string(nil), spec.Env...)
	return execCmd, nil
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode()
	}
	return -1
}
