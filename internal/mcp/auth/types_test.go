package auth

import (
	"strings"
	"testing"
)

func TestTargetKeyRejectsIdentitySeparatorBytes(t *testing.T) {
	t.Parallel()

	t.Run("Should reject NUL in workspace and server identity fields", func(t *testing.T) {
		t.Parallel()

		for _, target := range []Target{
			{Scope: ScopeGlobal, ServerName: "linear\x00workspace"},
			{Scope: ScopeWorkspace, WorkspaceID: "workspace\x00linear", ServerName: "linear"},
		} {
			if _, err := target.Key(); err == nil || !strings.Contains(err.Error(), "NUL") {
				t.Fatalf("Target.Key(%#v) error = %v, want NUL rejection", target, err)
			}
		}
	})
}
