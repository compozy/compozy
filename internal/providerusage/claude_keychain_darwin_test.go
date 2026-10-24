//go:build darwin

package providerusage

import (
	"encoding/base64"
	"errors"
	"os"
	"testing"
)

func TestClaudeKeychainCommand(t *testing.T) {
	t.Run("Should distinguish missing items from denied access and other failures", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			result string
			want   error
		}{
			{`{"status":-25300}`, os.ErrNotExist},
			{`{"status":-25308}`, os.ErrPermission},
			{`{"status":-25293}`, os.ErrPermission},
			{`{"status":-128}`, os.ErrPermission},
		} {
			_, err := decodeClaudeKeychainResult([]byte(tc.result))
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		}
		_, err := decodeClaudeKeychainResult([]byte(`{"status":-1,"data":"sensitive unexpected OS failure"}`))
		requireEqual(t, ErrorCodeProviderUnavailable, AsUsageError(err, "Claude").Code)
	})
	t.Run("Should decode successful bytes and reject malformed helper output", func(t *testing.T) {
		t.Parallel()
		want := `{"claudeAiOauth":{"accessToken":"synthetic"}}`
		data, err := decodeClaudeKeychainResult(
			[]byte(`{"status":0,"data":"` + base64.StdEncoding.EncodeToString([]byte(want)) + `"}`),
		)
		requireNoError(t, err)
		requireEqual(t, want, string(data))
		for _, result := range []string{`{`, `{}`, `{"status":0,"data":"%%%"}`} {
			_, err := decodeClaudeKeychainResult([]byte(result))
			requireEqual(t, ErrorCodeParseError, AsUsageError(err, "Claude").Code)
		}
	})
	t.Run("Should pass exact lookup values and interaction policy as separate arguments", func(t *testing.T) {
		t.Parallel()
		for _, interactive := range []bool{false, true} {
			command := claudeKeychainCommand(t.Context(), "service' ; injected", "account", interactive)
			requireEqual(t, "/usr/bin/osascript", command.Path)
			requireEqual(t, "--", command.Args[5])
			requireEqual(t, "service' ; injected", command.Args[6])
			requireEqual(t, "account", command.Args[7])
			requireEqual(t, interactive, command.Args[8] == "true")
		}
	})
}
