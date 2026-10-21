//go:build !darwin

package providerusage

import "context"

type claudeUnavailableKeychain struct{}

var _ KeychainReader = claudeUnavailableKeychain{}

func defaultClaudeKeychain() KeychainReader { return claudeUnavailableKeychain{} }
func (claudeUnavailableKeychain) Read(context.Context, string, string) ([]byte, error) {
	return nil, NewUsageError(ErrorCodeAuthMissing, "Claude")
}
