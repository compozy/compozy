package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
)

const restoreGoldenHuman = "Restored spec-cycle 0.7.3 (bundled, enabled)."

func TestExtensionRestoreCommand(t *testing.T) {
	t.Parallel()

	restored := ExtensionRestoreRecord{
		Name:    "spec-cycle",
		Status:  "restored",
		Source:  "bundled",
		Version: "0.7.3",
		Enabled: true,
		Path:    "/home/operator/.compozy/extensions/spec-cycle",
	}

	t.Run("Should print the golden restore line and forward the name to the daemon", func(t *testing.T) {
		t.Parallel()

		var gotName string
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			restoreExtensionBuiltinFn: func(_ context.Context, name string) (ExtensionRestoreRecord, error) {
				gotName = name
				return restored, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "extension", "restore", "spec-cycle", "--global")
		if err != nil {
			t.Fatalf("extension restore error = %v", err)
		}
		if gotName != "spec-cycle" {
			t.Fatalf("RestoreExtensionBuiltin name = %q, want spec-cycle", gotName)
		}
		if !strings.Contains(stdout, restoreGoldenHuman) {
			t.Fatalf("stdout = %q, want %q", stdout, restoreGoldenHuman)
		}
	})

	t.Run("Should print the restore payload as the JSON object with the DX fields", func(t *testing.T) {
		t.Parallel()

		deps, _ := newExtensionLocalDeps(t, &stubClient{
			restoreExtensionBuiltinFn: func(context.Context, string) (ExtensionRestoreRecord, error) {
				return restored, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "extension", "restore", "spec-cycle", "-o", "json")
		if err != nil {
			t.Fatalf("extension restore -o json error = %v", err)
		}
		want := `{"name":"spec-cycle","status":"restored","source":"bundled","version":"0.7.3",` +
			`"enabled":true,"path":"/home/operator/.compozy/extensions/spec-cycle"}`
		requireCompactJSON(t, stdout, want)
	})

	t.Run("Should exit 1 with the daemon conflict code when the built-in is not removed", func(t *testing.T) {
		t.Parallel()

		deps, _ := newExtensionLocalDeps(t, &stubClient{
			restoreExtensionBuiltinFn: func(context.Context, string) (ExtensionRestoreRecord, error) {
				return ExtensionRestoreRecord{}, &extensionOperationAPIError{
					statusCode: http.StatusConflict,
					status:     "409 Conflict",
					payload: contract.ExtensionOperationErrorPayload{
						Error: "spec-cycle is not permanently removed",
						Code:  diagnosticcontract.CodeExtensionBuiltinNotRemoved,
					},
				}
			},
		})
		markExtensionDaemonRunning(&deps)

		_, _, err := executeRootCommand(t, deps, "extension", "restore", "spec-cycle", "--global")
		if err == nil {
			t.Fatal("extension restore error = nil, want the daemon conflict")
		}
		if got := cliExitCodeForError(err); got != 1 {
			t.Fatalf("exit code = %d, want 1", got)
		}
		apiErr, ok := errors.AsType[*extensionOperationAPIError](err)
		if !ok || apiErr.payload.Code != diagnosticcontract.CodeExtensionBuiltinNotRemoved {
			t.Fatalf(
				"restore conflict = %v, want diagnostic code %q",
				err,
				diagnosticcontract.CodeExtensionBuiltinNotRemoved,
			)
		}
	})

	t.Run("Should exit 2 for a missing name before contacting the daemon", func(t *testing.T) {
		t.Parallel()

		calls := 0
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			restoreExtensionBuiltinFn: func(context.Context, string) (ExtensionRestoreRecord, error) {
				calls++
				return ExtensionRestoreRecord{}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		_, _, err := executeRootCommand(t, deps, "extension", "restore")
		if err == nil {
			t.Fatal("extension restore without a name error = nil, want invalid usage")
		}
		if got := cliExitCodeForError(err); got != 2 {
			t.Fatalf("exit code = %d, want 2 for invalid usage", got)
		}
		if !strings.Contains(err.Error(), "accepts 1 arg(s), received 0") {
			t.Fatalf("usage error = %q, want the one-argument requirement", err)
		}
		if calls != 0 {
			t.Fatalf("RestoreExtensionBuiltin calls = %d, want 0 for invalid usage", calls)
		}
	})
}

// UT-015: removed-list JSON matches the DTO shape in _dx.md, and an empty list renders {"removed":[]}.
func TestExtensionRemovedCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should render an empty listing as {\"removed\":[]} with exit 0", func(t *testing.T) {
		t.Parallel()

		deps, _ := newExtensionLocalDeps(t, &stubClient{
			listRemovedBuiltinsFn: func(context.Context) ([]ExtensionRemovedBuiltinRecord, error) {
				return nil, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "extension", "removed", "-o", "json")
		if got := cliExitCodeForError(err); err != nil || got != 0 {
			t.Fatalf("extension removed -o json err = %v, exit = %d; want success", err, got)
		}
		requireCompactJSON(t, stdout, `{"removed":[]}`)
	})

	t.Run("Should render each removed built-in with the DTO fields in JSON", func(t *testing.T) {
		t.Parallel()

		deps, _ := newExtensionLocalDeps(t, &stubClient{
			listRemovedBuiltinsFn: func(context.Context) ([]ExtensionRemovedBuiltinRecord, error) {
				return []ExtensionRemovedBuiltinRecord{{
					Name: "spec-cycle", RemovedAt: "2026-11-17T14:02:11Z", RemovedBy: "operator",
				}}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "extension", "removed", "-o", "json")
		if err != nil {
			t.Fatalf("extension removed -o json error = %v", err)
		}
		want := `{"removed":[{"name":"spec-cycle","removed_at":"2026-11-17T14:02:11Z","removed_by":"operator"}]}`
		requireCompactJSON(t, stdout, want)
	})

	t.Run("Should render the removed listing as a table in human output", func(t *testing.T) {
		t.Parallel()

		deps, _ := newExtensionLocalDeps(t, &stubClient{
			listRemovedBuiltinsFn: func(context.Context) ([]ExtensionRemovedBuiltinRecord, error) {
				return []ExtensionRemovedBuiltinRecord{{
					Name: "spec-cycle", RemovedAt: "2026-11-17T14:02:11Z", RemovedBy: "operator",
				}}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "extension", "removed")
		if err != nil {
			t.Fatalf("extension removed error = %v", err)
		}
		for _, want := range []string{"NAME", "REMOVED_AT", "REMOVED_BY", "spec-cycle", "2026-11-17T14:02:11Z", "operator"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("human output = %q, missing %q", stdout, want)
			}
		}
	})
}

// requireCompactJSON compares the CLI's indented JSON with the golden shape after compacting both sides.
func requireCompactJSON(t *testing.T, got string, want string) {
	t.Helper()
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(strings.TrimSpace(got))); err != nil {
		t.Fatalf("output is not JSON: %v; stdout=%s", err, got)
	}
	if compact.String() != want {
		t.Fatalf("JSON = %s, want %s", compact.String(), want)
	}
}
