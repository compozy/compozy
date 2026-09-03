//go:build !windows

package terminal

import "os"

func platformShellCandidates() []string {
	return []string{os.Getenv("SHELL"), "zsh", "bash", "sh"}
}
