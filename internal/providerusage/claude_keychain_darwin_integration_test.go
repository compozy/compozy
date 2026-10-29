//go:build darwin && integration

package providerusage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestClaudeKeychainIntegration(t *testing.T) {
	if os.Getenv("COMPOZY_KEYCHAIN_TEST") != "1" {
		t.Skip("set COMPOZY_KEYCHAIN_TEST=1 to authorize disposable Keychain items")
	}
	t.Run(
		"Should read rotated credentials with security without renewing a foreign partition grant",
		func(t *testing.T) {
			t.Parallel()
			service := claudeTestKeychainItem(t, true)
			reader := defaultClaudeKeychain()
			for _, value := range []string{"synthetic-initial", "synthetic-rotated"} {
				cmd := exec.CommandContext(
					t.Context(),
					"/usr/bin/security",
					"add-generic-password",
					"-U",
					"-s",
					service,
					"-a",
					"compozy-qa",
					"-X",
					hex.EncodeToString([]byte(value)),
				)
				requireNoError(t, cmd.Run())
				data, err := reader.Read(t.Context(), service, "compozy-qa", false)
				requireNoError(t, err)
				requireEqual(t, value, string(data))
				clear(data)
			}
			data, err := reader.Read(t.Context(), service, "compozy-qa", true)
			requireNoError(t, err)
			requireEqual(t, "synthetic-rotated", string(data))
			clear(data)
		},
	)
	t.Run("Should refuse an untrusted item before launching security", func(t *testing.T) {
		t.Parallel()
		service := claudeTestKeychainItem(t, false)
		reader := claudeExecKeychain{run: func(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
			if cmd.Path == "/usr/bin/security" {
				t.Fatal("untrusted item reached secret reader")
			}
			return runClaudeKeychainCommand(ctx, cmd)
		}}
		_, err := reader.Read(t.Context(), service, "compozy-qa", false)
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("got %v, want denied", err)
		}
	})
}

func claudeTestKeychainItem(t *testing.T, trusted bool) string {
	t.Helper()
	service := fmt.Sprintf("compozy-qa-1023-%x", sha256.Sum256([]byte(t.TempDir())))
	args := []string{"add-generic-password", "-s", service, "-a", "compozy-qa", "-w", "synthetic"}
	if !trusted {
		args = append(args, "-T", "/usr/bin/osascript")
	}
	requireNoError(t, exec.CommandContext(t.Context(), "/usr/bin/security", args...).Run())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "/usr/bin/security", "delete-generic-password", "-s", service, "-a", "compozy-qa").Run(); err != nil {
			t.Errorf("remove disposable Keychain item: %v", err)
		}
	})
	return service
}
