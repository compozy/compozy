//go:build darwin

package providerusage

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

func TestClaudeKeychainCommand(t *testing.T) {
	t.Run("Should fail closed unless metadata explicitly admits the read", func(t *testing.T) {
		t.Parallel()
		for _, data := range []string{`{`, `{}`, `{"status":0}`, `{"status":0,"safe":false}`, `{"status":-25293,"safe":true}`} {
			if !errors.Is(decodeClaudeKeychainPreflight([]byte(data)), os.ErrPermission) {
				t.Fatalf("preflight admitted %s", data)
			}
		}
		requireNoError(t, decodeClaudeKeychainPreflight([]byte(`{"status":0,"safe":true}`)))
		if !errors.Is(decodeClaudeKeychainPreflight([]byte(`{"status":-25300}`)), os.ErrNotExist) {
			t.Fatal("missing item was not classified")
		}
	})
	t.Run("Should preserve lookup arguments without interpreting shell syntax", func(t *testing.T) {
		t.Parallel()
		command := claudeKeychainCommand(t.Context(), "service' ; injected", "account")
		requireEqual(t, "/usr/bin/security", command.Path)
		requireEqual(t, "service' ; injected", command.Args[3])
		requireEqual(t, "account", command.Args[5])
		requireEqual(t, "-w", command.Args[6])
	})
	t.Run("Should never launch a secret read after a rejected or failed preflight", func(t *testing.T) {
		t.Parallel()
		for _, failed := range []bool{false, true} {
			calls := 0
			reader := claudeExecKeychain{run: func(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
				calls++
				requireEqual(t, "/usr/bin/osascript", cmd.Path)
				if failed {
					return nil, errors.New("sensitive upstream diagnostic")
				}
				return []byte(`{"status":0,"safe":false}`), nil
			}}
			_, err := reader.Read(t.Context(), "service", "account", false)
			if !errors.Is(err, os.ErrPermission) {
				t.Fatalf("got %v", err)
			}
			requireEqual(t, 1, calls)
		}
	})
	t.Run("Should read with security after approval and bypass preflight only for explicit intent", func(t *testing.T) {
		t.Parallel()
		for _, interactive := range []bool{false, true} {
			calls := 0
			reader := claudeExecKeychain{run: func(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
				calls++
				if cmd.Path == "/usr/bin/osascript" {
					return []byte(`{"status":0,"safe":true}`), nil
				}
				requireEqual(t, "/usr/bin/security", cmd.Path)
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing subprocess deadline")
				}
				return []byte("synthetic\n"), nil
			}}
			data, err := reader.Read(t.Context(), "service", "account", interactive)
			requireNoError(t, err)
			requireEqual(t, "synthetic", string(data))
			want := 2
			if interactive {
				want = 1
			}
			requireEqual(t, want, calls)
		}
	})
	t.Run("Should sanitize subprocess errors and distinguish OS denial from missing credentials", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			code int
			want error
		}{{44, os.ErrNotExist}, {36, os.ErrPermission}, {51, os.ErrPermission}, {128, os.ErrPermission}} {
			cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit \"$1\"", "sh", strconv.Itoa(tc.code))
			exitErr := cmd.Run()
			reader := claudeExecKeychain{run: func(context.Context, *exec.Cmd) ([]byte, error) { return []byte("secret"), exitErr }}
			_, err := reader.Read(t.Context(), "service", "account", true)
			if !errors.Is(err, tc.want) {
				t.Fatalf("code %d: got %v", tc.code, err)
			}
		}
		reader := claudeExecKeychain{run: func(context.Context, *exec.Cmd) ([]byte, error) { return nil, errors.New("secret") }}
		_, err := reader.Read(t.Context(), "service", "account", true)
		requireEqual(t, ErrorCodeProviderUnavailable, AsUsageError(err, "Claude").Code)
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("upstream output escaped")
		}
	})
	t.Run("Should terminate a timed out command and return no partial secret", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", "printf synthetic; sleep 20 & wait")
		data, err := runClaudeKeychainCommand(ctx, cmd)
		if err == nil || len(data) != 0 {
			t.Fatal("timed out command returned data")
		}
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("deadline: %v", ctx.Err())
		}
		if cmd.ProcessState == nil {
			t.Fatal("process was not reaped")
		}
		requireNoError(t, procutil.WaitForCommandProcessGroupExit(cmd, time.Second))
	})
	t.Run("Should classify a background timeout as consent recovery without exposing output", func(t *testing.T) {
		t.Parallel()
		reader := claudeExecKeychain{run: func(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
			if cmd.Path == "/usr/bin/osascript" {
				return []byte(`{"status":0,"safe":true}`), nil
			}
			<-ctx.Done()
			return []byte("secret"), ctx.Err()
		}}
		_, err := reader.Read(t.Context(), "service", "account", false)
		usage := AsUsageError(err, "Claude")
		requireEqual(t, ErrorCodeKeychainReadTimeout, usage.Code)
		requireEqual(t, StatusNeedsConsent, usage.Status())
		requireEqual(t, string(ActionRefresh), usage.Action)
		requireEqual(t, true, explicitRecovery("claude", usage))
	})
	t.Run("Should bound captured secret output", func(t *testing.T) {
		t.Parallel()
		output := &claudeSecretBuffer{}
		_, err := output.Write(make([]byte, MaxResponseBytes+1))
		requireEqual(t, ErrorCodeParseError, AsUsageError(err, "Claude").Code)
		requireEqual(t, 0, len(output.data))
	})
}
