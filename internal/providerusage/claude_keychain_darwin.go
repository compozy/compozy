//go:build darwin

package providerusage

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

// The CLI has no no-UI flag. Bound a prompt if the item changes after preflight.
const claudeBackgroundReadTimeout = 1500 * time.Millisecond

type claudeExecKeychain struct {
	run func(context.Context, *exec.Cmd) ([]byte, error)
}

var _ KeychainReader = claudeExecKeychain{}

func defaultClaudeKeychain() KeychainReader { return claudeExecKeychain{run: runClaudeKeychainCommand} }

func claudeKeychainCommand(ctx context.Context, service, account string) *exec.Cmd {
	return exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-s", service, "-a", account, "-w")
}

// Read returns secret bytes for the caller to clear. Explicit reads may authorize;
// background reads require safe metadata and bound the remaining check/read race.
func (c claudeExecKeychain) Read(ctx context.Context, service, account string, interactive bool) ([]byte, error) {
	if !interactive {
		if err := c.preflight(ctx, service, account); err != nil {
			return nil, err
		}
	}
	timeout := claudeBackgroundReadTimeout
	if interactive {
		timeout = 60 * time.Second
	}
	readCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	data, err := c.run(readCtx, claudeKeychainCommand(readCtx, service, account))
	if err != nil {
		clear(data)
		if !interactive && errors.Is(readCtx.Err(), context.DeadlineExceeded) {
			return nil, NewUsageError(ErrorCodeKeychainReadTimeout, "Claude")
		}
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			switch exit.ExitCode() {
			case 44:
				return nil, os.ErrNotExist
			case 36, 51, 128:
				return nil, os.ErrPermission
			}
		}
		return nil, AsUsageError(err, "Claude")
	}
	return bytes.TrimSuffix(data, []byte{'\n'}), nil
}

// preflight inspects metadata with UI disabled and never requests secret data.
func (c claudeExecKeychain) preflight(ctx context.Context, service, account string) error {
	checkCtx, cancel := context.WithTimeout(ctx, claudeBackgroundReadTimeout)
	defer cancel()
	command := exec.CommandContext(checkCtx, "/usr/bin/osascript", "-l", "JavaScript", "-e",
		claudeKeychainPreflightScript, "--", service, account)
	data, err := c.run(checkCtx, command)
	defer clear(data)
	if err != nil {
		if errors.Is(checkCtx.Err(), context.DeadlineExceeded) {
			return NewUsageError(ErrorCodeKeychainReadTimeout, "Claude")
		}
		return os.ErrPermission
	}
	return decodeClaudeKeychainPreflight(data)
}

func decodeClaudeKeychainPreflight(data []byte) error {
	var result struct {
		Status *int `json:"status"`
		Safe   bool `json:"safe"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.Status == nil {
		return os.ErrPermission
	}
	if *result.Status == -25300 {
		return os.ErrNotExist
	}
	if *result.Status != 0 || !result.Safe {
		return os.ErrPermission
	}
	return nil
}

// runClaudeKeychainCommand kills the isolated process group on cancellation,
// discards stderr, and transfers ownership only of a successful bounded stdout.
func runClaudeKeychainCommand(ctx context.Context, command *exec.Cmd) ([]byte, error) {
	procutil.ConfigureCommandProcessGroup(command)
	command.Cancel = func() error { return procutil.SignalCommandProcessGroup(command, syscall.SIGKILL) }
	command.WaitDelay = 100 * time.Millisecond
	output := &claudeSecretBuffer{}
	command.Stdout = output
	if err := command.Run(); err != nil {
		clear(output.data)
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		clear(output.data)
		return nil, err
	}
	return output.data, nil
}

type claudeSecretBuffer struct{ data []byte }

func (b *claudeSecretBuffer) Write(data []byte) (int, error) {
	if len(b.data)+len(data) > MaxResponseBytes {
		return 0, NewUsageError(ErrorCodeParseError, "Claude")
	}
	b.data = append(b.data, data...)
	return len(data), nil
}
