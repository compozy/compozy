//go:build darwin

package providerusage

import (
	"reflect"
	"testing"
)

func TestClaudeKeychainCommand(t *testing.T) {
	t.Run("Should bind security lookup to exactly one account and service without executing", func(t *testing.T) {
		t.Parallel()
		command := claudeKeychainCommand(t.Context(), "Claude Code-credentials-route", "test-user")
		want := []string{
			"/usr/bin/security",
			"find-generic-password",
			"-a",
			"test-user",
			"-w",
			"-s",
			"Claude Code-credentials-route",
		}
		if !reflect.DeepEqual(want, command.Args) {
			t.Fatalf("unexpected command arguments: %v", command.Args)
		}
		requireEqual(t, "/usr/bin/security", command.Path)
	})
}
