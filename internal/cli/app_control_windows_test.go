//go:build windows

package cli

import (
	"crypto/sha256"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/tailscale/go-winio"
)

// Invariant: Windows control authenticates over a same-user local pipe and rejects remote discovery.
func TestAppControlWindows(t *testing.T) {
	t.Parallel()
	t.Run("Should exchange authenticated CLI requests over a real named pipe", func(t *testing.T) {
		t.Parallel()
		directory := t.TempDir()
		address := fmt.Sprintf(`\\.\pipe\compozyos-%x`, sha256.Sum256([]byte(directory)))
		listener, err := winio.ListenPipe(address, nil)
		if err != nil {
			t.Fatalf("ListenPipe() error = %v", err)
		}
		t.Cleanup(func() {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("Close(pipe) error = %v", err)
			}
		})
		for name, contents := range map[string]string{"app.sock": address, "app.token": "current-token"} {
			if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o600); err != nil {
				t.Fatalf("WriteFile(%s) error = %v", name, err)
			}
		}
		served := make(chan error, 1)
		go func() {
			served <- exchangeWindowsAppControl(listener)
		}()
		result, err := callAppControl(t.Context(), filepath.Join(directory, "app.sock"), "diagnose", nil)
		if err != nil {
			t.Fatalf("callAppControl() error = %v", err)
		}
		fields, ok := result.(map[string]any)
		if !ok || fields["authenticated"] != true {
			t.Fatalf("callAppControl() = %#v, want authenticated response", result)
		}
		if err := <-served; err != nil {
			t.Fatalf("pipe server error = %v", err)
		}
	})
	t.Run("Should reject remote and unrelated pipe addresses before transmitting credentials", func(t *testing.T) {
		t.Parallel()
		for _, address := range []string{`\\remote\pipe\compozyos-foo`, `\\.\pipe\another-app`, "127.0.0.1:1234"} {
			path := filepath.Join(t.TempDir(), "app.sock")
			if err := os.WriteFile(path, []byte(address), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			if connection, err := dialAppControl(t.Context(), path); err == nil {
				if closeErr := connection.Close(); closeErr != nil {
					t.Errorf("Close() error = %v", closeErr)
				}
				t.Fatal("dialAppControl() accepted an invalid local pipe address")
			}
		}
	})
}

func exchangeWindowsAppControl(listener net.Listener) (err error) {
	connection, err := listener.Accept()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, connection.Close()) }()
	var request appControlRequest
	data, err := jsontext.NewDecoder(connection).ReadValue()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return err
	}
	if request.Token != "current-token" || request.Method != "diagnose" {
		return errors.New("unexpected control request")
	}
	return json.MarshalWrite(connection, map[string]any{
		"schema_version": appControlSchemaVersion, "id": request.ID,
		"result": map[string]bool{"authenticated": true},
	})
}
