//go:build darwin

package providerusage

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestClaudeKeychainCommand(t *testing.T) {
	t.Run("Should distinguish missing items from denied access and other failures", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			code       int
			diagnostic string
			want       error
		}{
			{44, "", os.ErrNotExist},
			{1, "The specified item could not be found in the keychain.", os.ErrNotExist},
			{128, "User canceled the operation.", os.ErrPermission},
			{36, "User interaction is not allowed.", os.ErrPermission},
			{51, "Authorization denied.", os.ErrPermission},
		} {
			if err := claudeKeychainFailure(tc.code, []byte(tc.diagnostic)); !errors.Is(err, tc.want) {
				t.Fatalf("exit %d: got %v, want %v", tc.code, err, tc.want)
			}
		}
		err := claudeKeychainFailure(1, []byte("sensitive unexpected OS failure"))
		requireEqual(t, ErrorCodeProviderUnavailable, AsUsageError(err, "Claude").Code)
	})

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
