//go:build darwin

package providerusage

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

type claudeExecKeychain struct{}

var _ KeychainReader = claudeExecKeychain{}

func defaultClaudeKeychain() KeychainReader { return claudeExecKeychain{} }
func claudeKeychainCommand(ctx context.Context, service, account string) *exec.Cmd {
	return exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-a", account, "-w", "-s", service)
}
func (claudeExecKeychain) Read(ctx context.Context, service, account string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	command := claudeKeychainCommand(ctx, service, account)
	command.WaitDelay = time.Second
	output := &claudeSecretBuffer{}
	command.Stdout = output
	diagnostic := &claudeSecretBuffer{}
	command.Stderr = diagnostic
	defer func() { clear(diagnostic.data) }()
	if err := command.Run(); err != nil {
		clear(output.data)
		code := -1
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exit.ExitCode()
		}
		return nil, claudeKeychainFailure(code, diagnostic.data)
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

// claudeKeychainFailure classifies OS results without exposing diagnostic text.
func claudeKeychainFailure(code int, diagnostic []byte) error {
	message := strings.ToLower(string(diagnostic))
	if code == 44 || strings.Contains(message, "could not be found") {
		return os.ErrNotExist
	}
	if code == 36 || code == 128 || code == 51 ||
		strings.Contains(message, "user canceled") ||
		strings.Contains(message, "interaction is not allowed") || strings.Contains(message, "authorization denied") {
		return os.ErrPermission
	}
	return NewUsageError(ErrorCodeProviderUnavailable, "Claude")
}
