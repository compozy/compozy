//go:build windows

package cli

import (
	"crypto/sha256"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
)

// Invariant: Windows control authenticates over a same-user local pipe and rejects remote discovery.
func TestAppControlWindows(t *testing.T) {
	t.Parallel()
	t.Run("Should exchange authenticated CLI requests over a real named pipe", func(t *testing.T) {
		t.Parallel()
		directory := appControlWindowsTestDirectory(t)
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
	t.Run("Should close a different owner's pipe before sending request bytes", func(t *testing.T) {
		t.Parallel()
		directory := appControlWindowsTestDirectory(t)
		address := fmt.Sprintf(`\\.\pipe\compozyos-%x`, sha256.Sum256([]byte(directory)))
		listener, err := winio.ListenPipe(address, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		})
		path := filepath.Join(directory, "app.sock")
		if err := os.WriteFile(path, []byte(address), 0o600); err != nil {
			t.Fatal(err)
		}
		served := make(chan error, 1)
		go func() {
			connection, err := listener.Accept()
			if err != nil {
				served <- err
				return
			}
			bytes, readErr := io.ReadAll(connection)
			closeErr := connection.Close()
			if len(bytes) != 0 {
				readErr = errors.Join(readErr, errors.New("untrusted pipe received request bytes"))
			}
			served <- errors.Join(readErr, closeErr)
		}()
		connection, err := dialAppControlWithProcessInspector(
			t.Context(),
			path,
			func(pid uint32) (string, string, error) {
				if pid != uint32(os.Getpid()) {
					return "", "", fmt.Errorf("unexpected pipe server process: %d", pid)
				}
				return "S-1-0-0", "", nil
			},
		)
		if connection != nil {
			if closeErr := connection.Close(); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatal("untrusted pipe remained connected")
		}
		if err == nil || !strings.Contains(err.Error(), "pipe server belongs to another user") {
			t.Fatalf("dialAppControlWithProcessInspector() = %v, want owner rejection", err)
		}
		if err := <-served; err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should reject remote and unrelated pipe addresses before transmitting credentials", func(t *testing.T) {
		t.Parallel()
		remoteAddress := `\\remote\pipe\compozyos-` + strings.Repeat("a", 64)
		for _, address := range []string{remoteAddress, `\\.\pipe\another-app`, "127.0.0.1:1234"} {
			path := filepath.Join(appControlWindowsTestDirectory(t), "app.sock")
			if err := os.WriteFile(path, []byte(address), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			if connection, err := dialAppControl(t.Context(), path); err == nil {
				if closeErr := connection.Close(); closeErr != nil {
					t.Errorf("Close() error = %v", closeErr)
				}
				t.Fatal("dialAppControl() accepted an invalid local pipe address")
			} else if !strings.Contains(err.Error(), "invalid local pipe address") {
				t.Fatalf("dialAppControl() rejected discovery for the wrong reason: %v", err)
			}
		}
	})
}

func appControlWindowsTestDirectory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	operator, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	security, err := windows.SecurityDescriptorFromString(
		fmt.Sprintf("D:P(A;OICI;FA;;;%s)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)", operator.User.Sid.String()),
	)
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := security.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(directory, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	return directory
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
