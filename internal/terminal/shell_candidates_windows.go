//go:build windows

package terminal

import "os"

func platformShellCandidates() []string {
	return []string{os.Getenv("COMSPEC"), "pwsh.exe", "powershell.exe", "cmd.exe"}
}
