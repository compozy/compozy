//go:build !windows

package terminal

import "os"

// platformShellCandidates orders the account login shell before the daemon's
// inherited SHELL, which reflects its launcher rather than the operator's choice.
func platformShellCandidates(accountShell string) []string {
	return []string{accountShell, os.Getenv("SHELL"), "zsh", "bash", "sh"}
}
