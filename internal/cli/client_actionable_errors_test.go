package cli

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
)

func TestUnixSocketClientActionableDaemonErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should include daemon start guidance when socket is missing", func(t *testing.T) {
		t.Parallel()

		root, err := os.MkdirTemp("/tmp", "agh-sock-")
		if err != nil {
			t.Fatalf("os.MkdirTemp() error = %v", err)
		}
		t.Cleanup(func() {
			if removeErr := os.RemoveAll(root); removeErr != nil {
				t.Errorf("os.RemoveAll(%q) error = %v", root, removeErr)
			}
		})
		socketPath := filepath.Join(root, "agh.sock")
		client, err := NewClient(socketPath)
		if err != nil {
			t.Fatalf("NewClient() error = %v", err)
		}

		_, err = client.DaemonStatus(context.Background())
		if err == nil {
			t.Fatal("DaemonStatus() error = nil, want missing daemon socket failure")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("DaemonStatus() error = %v, want os.ErrNotExist in chain", err)
		}
		assertDaemonUnavailableDiagnostic(t, err, socketPath)
	})

	t.Run("Should include daemon start guidance when socket refuses connection", func(t *testing.T) {
		t.Parallel()

		socketPath := "/tmp/agh-stale.sock"
		client := &unixSocketClient{
			socketPath: socketPath,
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
					return nil, &url.Error{
						Op:  "Get",
						URL: baseURL + "/api/status",
						Err: syscall.ECONNREFUSED,
					}
				}),
			},
		}

		_, err := client.DaemonStatus(context.Background())
		if err == nil {
			t.Fatal("DaemonStatus() error = nil, want stale daemon socket failure")
		}
		if !errors.Is(err, syscall.ECONNREFUSED) {
			t.Fatalf("DaemonStatus() error = %v, want syscall.ECONNREFUSED in chain", err)
		}
		assertDaemonUnavailableDiagnostic(t, err, socketPath)
	})
}

func assertDaemonUnavailableDiagnostic(t *testing.T, err error, socketPath string) {
	t.Helper()

	var structured *StructuredError
	if !errors.As(err, &structured) {
		t.Fatalf("DaemonStatus() error = %T, want *StructuredError", err)
	}
	if structured.Item.Code != contract.CodeDaemonUnavailable {
		t.Fatalf("StructuredError.Code = %q, want %q", structured.Item.Code, contract.CodeDaemonUnavailable)
	}
	if structured.Item.SuggestedCommand != "agh daemon start" {
		t.Fatalf("StructuredError.SuggestedCommand = %q, want agh daemon start", structured.Item.SuggestedCommand)
	}
	if structured.Item.Evidence["socket_path"] != socketPath {
		t.Fatalf(
			"StructuredError.Evidence[socket_path] = %#v, want %q",
			structured.Item.Evidence["socket_path"],
			socketPath,
		)
	}
	if got := agentidentity.ExitCodeForError(err); got != agentidentity.ExitUnavailable {
		t.Fatalf("ExitCodeForError() = %d, want %d", got, agentidentity.ExitUnavailable)
	}
}
