package providers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/providerauth"
	"github.com/compozy/compozy/internal/subprocess"
	"golang.org/x/term"
)

const DefaultProviderAuthCommandTimeout = 30 * time.Second

// ProviderAuthCommandRunner executes a provider-owned auth command.
type ProviderAuthCommandRunner func(context.Context, ProviderAuthCommandSpec) (ProviderAuthCommandResult, error)

// ProviderAuthCommandSpec describes one provider-owned auth command execution.
type ProviderAuthCommandSpec struct {
	Command    string
	Executable string
	Args       []string
	Dir        string
	Env        []string
	Timeout    time.Duration
	NoTTY      bool
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
	environment := append([]string(nil), spec.Env...)
	if len(environment) == 0 {
		environment = os.Environ()
	}
	spec.Env = environment
	prepared, err := resolveProviderAuthCommandSpec(
		spec,
		subprocess.ResolveExecutable,
	)
	if err != nil {
		return nil, err
	}
	// #nosec G204 -- Provider auth commands are explicit operator config.
	execCmd := exec.CommandContext(ctx, prepared.Executable, prepared.Args...)
	execCmd.Dir = prepared.Dir
	execCmd.Env = prepared.Env
	return execCmd, nil
}

func resolveProviderAuthCommandSpec(
	spec ProviderAuthCommandSpec,
	resolveExecutable func(string, []string, string) (string, error),
) (ProviderAuthCommandSpec, error) {
	command := strings.TrimSpace(spec.Command)
	if command == "" {
		return ProviderAuthCommandSpec{}, errors.New("provider auth command is required")
	}
	parsed, err := providerauth.ParseCommand(command)
	if err != nil {
		return ProviderAuthCommandSpec{}, fmt.Errorf("parse provider auth command: %w", err)
	}
	environment := parsed.ApplyEnvironment(spec.Env)

	resolved := spec.Executable
	args := append([]string(nil), spec.Args...)
	if resolved == "" {
		if resolveExecutable == nil {
			return ProviderAuthCommandSpec{}, errors.New("provider auth executable resolver is required")
		}
		resolved, err = resolveExecutable(parsed.Executable, environment, spec.Dir)
		if err != nil {
			return ProviderAuthCommandSpec{}, fmt.Errorf(
				"resolve provider auth executable %q: %w",
				parsed.Executable,
				err,
			)
		}
		args = append([]string(nil), parsed.Args...)
	}
	if !filepath.IsAbs(resolved) {
		return ProviderAuthCommandSpec{}, fmt.Errorf(
			"provider auth executable %q must resolve to an absolute path",
			resolved,
		)
	}

	prepared := spec
	prepared.Command = command
	prepared.Executable = resolved
	prepared.Args = args
	prepared.Env = environment
	return prepared, nil
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
