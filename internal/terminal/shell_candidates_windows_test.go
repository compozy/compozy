//go:build windows

package terminal

// Suite: Windows terminal shell resolution.
// Invariant: an unspecified local terminal resolves a native Windows shell without a platform-only flag.
// Boundary IN: default shell request. Boundary OUT: host executable lookup.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsDefaultShellResolution(t *testing.T) { // IT-038
	t.Parallel()

	t.Run("Should resolve a native Windows shell when no default is configured", func(t *testing.T) {
		t.Parallel()
		resolved, err := resolveShell("", "")
		if err != nil {
			t.Fatalf("resolveShell() error = %v", err)
		}
		name := strings.ToLower(filepath.Base(resolved))
		if name != "pwsh.exe" && name != "powershell.exe" && name != "cmd.exe" {
			t.Fatalf("resolveShell() = %q, want a native Windows shell", resolved)
		}
	})
}
