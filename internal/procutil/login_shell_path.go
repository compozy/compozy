package procutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"
)

const loginShellPathProcessDrainDeadline = 250 * time.Millisecond

var errLoginShellPathOutputTooLarge = errors.New("procutil: login shell PATH output exceeds limit")

// LoginShellPathOptions bounds login shell PATH resolution.
type LoginShellPathOptions struct {
	Timeout        time.Duration
	MaxOutputBytes int
}

// ResolveLoginShellPath returns the PATH produced by the operator's login shell.
func ResolveLoginShellPath(
	ctx context.Context,
	environment []string,
	options LoginShellPathOptions,
) (string, error) {
	if ctx == nil {
		return "", errors.New("procutil: login shell PATH context is required")
	}
	if options.Timeout <= 0 {
		return "", errors.New("procutil: login shell PATH timeout must be positive")
	}
	if options.MaxOutputBytes <= 0 {
		return "", errors.New("procutil: login shell PATH output limit must be positive")
	}
	if runtime.GOOS == "windows" {
		return validateLoginShellPath(environmentValue(environment, "PATH"))
	}

	home := strings.TrimSpace(environmentValue(environment, "HOME"))
	if home == "" || !filepath.IsAbs(home) {
		return "", errors.New("procutil: operator home must be an absolute path")
	}
	shell := strings.TrimSpace(environmentValue(environment, "SHELL"))
	if !filepath.IsAbs(shell) {
		shell = defaultLoginShell()
	}

	commandCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	output := &boundedCommandOutput{limit: options.MaxOutputBytes}
	command := exec.CommandContext(commandCtx, shell, "-ilc", `printf '\000%s\000' "$PATH"`)
	command.Cancel = func() error {
		return SignalCommandProcessGroup(command, syscall.SIGKILL)
	}
	command.Dir = filepath.Clean(home)
	command.Env = withEnvironmentValue(environment, "DISABLE_AUTO_UPDATE", "true")
	command.Stdout = output
	ConfigureCommandProcessGroup(command)
	if err := command.Start(); err != nil {
		return "", fmt.Errorf("procutil: start login shell PATH resolver: %w", err)
	}
	if err := RegisterCommandProcessGroup(command); err != nil {
		killErr := KillCommandProcessGroupAndWait(command, loginShellPathProcessDrainDeadline)
		waitErr := command.Wait()
		return "", errors.Join(
			fmt.Errorf("procutil: register login shell process group: %w", err),
			killErr,
			waitErr,
		)
	}
	if err := command.Wait(); err != nil {
		if ctxErr := commandCtx.Err(); ctxErr != nil {
			return "", fmt.Errorf("procutil: resolve login shell PATH: %w", ctxErr)
		}
		return "", fmt.Errorf("procutil: wait for login shell PATH resolver: %w", err)
	}
	if output.exceeded {
		return "", fmt.Errorf(
			"procutil: login shell PATH output exceeded %d bytes: %w",
			options.MaxOutputBytes,
			errLoginShellPathOutputTooLarge,
		)
	}

	path, err := parseLoginShellPath(output.data)
	if err != nil {
		return "", err
	}
	return validateLoginShellPath(path)
}

type boundedCommandOutput struct {
	data     []byte
	limit    int
	exceeded bool
}

func (o *boundedCommandOutput) Write(chunk []byte) (int, error) {
	remaining := o.limit - len(o.data)
	if remaining > 0 {
		keep := min(len(chunk), remaining)
		o.data = append(o.data, chunk[:keep]...)
	}
	if len(chunk) > remaining {
		o.exceeded = true
	}
	return len(chunk), nil
}

func parseLoginShellPath(output []byte) (string, error) {
	end := bytes.LastIndexByte(output, 0)
	if end < 0 {
		return "", errors.New("procutil: login shell PATH terminator is missing")
	}
	start := bytes.LastIndexByte(output[:end], 0)
	if start < 0 {
		return "", errors.New("procutil: login shell PATH delimiter is missing")
	}
	return string(output[start+1 : end]), nil
}

func validateLoginShellPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("procutil: login shell PATH is empty")
	}
	if strings.ContainsAny(path, "\x00\r\n") {
		return "", errors.New("procutil: login shell PATH contains an invalid delimiter")
	}
	return path, nil
}

func defaultLoginShell() string {
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/sh"
}

func environmentValue(environment []string, key string) string {
	for _, entry := range slices.Backward(environment) {
		name, value, found := strings.Cut(entry, "=")
		if found && name == key {
			return value
		}
	}
	return ""
}

func withEnvironmentValue(environment []string, key string, value string) []string {
	result := make([]string, 0, len(environment)+1)
	written := false
	for _, entry := range environment {
		name, _, found := strings.Cut(entry, "=")
		if !found || name != key {
			result = append(result, entry)
			continue
		}
		if !written {
			result = append(result, key+"="+value)
			written = true
		}
	}
	if !written {
		result = append(result, key+"="+value)
	}
	return result
}
