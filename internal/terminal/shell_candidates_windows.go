//go:build windows

package terminal

import "os"

// Windows accounts record no login shell, so the account candidate is ignored.
func platformShellCandidates(string) []string {
	return []string{os.Getenv("COMSPEC"), "pwsh.exe", "powershell.exe", "cmd.exe"}
}
