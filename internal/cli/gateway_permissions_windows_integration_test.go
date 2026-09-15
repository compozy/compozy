//go:build windows && integration

package cli

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// Invariant: separate native CLI processes reuse private state and recover with the real Windows keyring.
func TestWindowsGatewayCLI(t *testing.T) {
	t.Run("Should reopen credentials and recover a journal across repeated CLI processes", func(t *testing.T) {
		t.Parallel()
		binary := os.Getenv("COMPOZY_TEST_BINARY")
		if binary == "" {
			t.Fatal("COMPOZY_TEST_BINARY must name the CI-built Windows CLI")
		}
		home := t.TempDir()
		paths, err := compozyconfig.ResolveHomePathsFrom(home)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := RemoveGatewayCredential(paths.GatewayCredentialsDir, "windows-test"); err != nil {
				t.Error(err)
			}
		})
		credential := testGatewayCredential('w')
		for range 2 {
			if _, err := WriteGatewayCredential(paths.GatewayCredentialsDir, "windows-test", credential); err != nil {
				t.Fatal(err)
			}
			if got, err := ReadGatewayCredential(
				paths.GatewayCredentialsDir,
				"windows-test",
			); err != nil ||
				got != credential {
				t.Fatalf("credential round trip failed: %v", err)
			}
		}
		journal, err := newGatewayProfileTransactionJournal(gatewayProfileTransactionPlan{
			operation: gatewayProfileTransactionRemove,
			profile:   compozyconfig.GatewayConnectionConfig{Name: "windows-test"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := writeGatewayProfileTransactionJournal(paths.GatewayCredentialsDir, journal); err != nil {
			t.Fatal(err)
		}
		for range 3 {
			cmd := exec.CommandContext(t.Context(), binary, "connect", "list", "-o", "json")
			cmd.Dir = home
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				if !strings.HasPrefix(strings.ToUpper(key), "COMPOZY_") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, "COMPOZY_HOME="+home)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("native connect list: %v: %s", err, output)
			}
			if strings.Contains(string(output), credential) {
				t.Fatal("CLI exposed a credential")
			}
		}
		if _, err := readGatewayProfileTransactionJournal(
			paths.GatewayCredentialsDir,
			"windows-test",
		); !errors.Is(
			err,
			os.ErrNotExist,
		) {
			t.Fatalf("recovered journal remains: %v", err)
		}
		if _, err := ReadGatewayCredential(
			paths.GatewayCredentialsDir,
			"windows-test",
		); !errors.Is(
			err,
			os.ErrNotExist,
		) {
			t.Fatalf("recovered credential remains: %v", err)
		}
	})
}
