//go:build darwin

package providerusage

import (
	"context"
	"os/exec"
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
	if err := command.Run(); err != nil {
		clear(output.data)
		return nil, NewUsageError(ErrorCodeKeychainDenied, "Claude")
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
