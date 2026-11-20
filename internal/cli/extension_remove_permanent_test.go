package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
)

const permanentRemoveGoldenHuman = "Removed spec-cycle permanently. It will not return on restart or update.\n" +
	"Restore it with: compozy extension restore spec-cycle"

func TestExtensionRemovePermanentCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should print the two-line permanent message and forward the global permanent removal", func(t *testing.T) {
		t.Parallel()

		var permanentName string
		removeCalls := 0
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			removeExtensionFn: func(context.Context, string) (ManagedExtensionRemoveRecord, error) {
				removeCalls++
				return ManagedExtensionRemoveRecord{}, nil
			},
			removePermanentExtensionFn: func(_ context.Context, name string) (ManagedExtensionRemoveRecord, error) {
				permanentName = name
				return ManagedExtensionRemoveRecord{
					Name:         name,
					Path:         "/tmp/spec-cycle",
					Status:       "removed",
					Permanent:    true,
					TombstonedAt: "2026-11-17T14:02:11Z",
				}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "extension", "remove", "spec-cycle", "--permanent")
		if err != nil {
			t.Fatalf("extension remove --permanent error = %v", err)
		}
		if permanentName != "spec-cycle" {
			t.Fatalf("RemovePermanentExtension name = %q, want spec-cycle", permanentName)
		}
		if removeCalls != 0 {
			t.Fatalf("RemoveExtension calls = %d, want 0 for permanent removal", removeCalls)
		}
		if !strings.Contains(stdout, permanentRemoveGoldenHuman) {
			t.Fatalf("stdout = %q, want the two-line permanent message %q", stdout, permanentRemoveGoldenHuman)
		}
	})

	t.Run("Should report permanent and tombstoned_at in the JSON output", func(t *testing.T) {
		t.Parallel()

		deps, _ := newExtensionLocalDeps(t, &stubClient{
			removePermanentExtensionFn: func(_ context.Context, name string) (ManagedExtensionRemoveRecord, error) {
				return ManagedExtensionRemoveRecord{
					Name:         name,
					Path:         "/tmp/spec-cycle",
					Status:       "removed",
					Permanent:    true,
					TombstonedAt: "2026-11-17T14:02:11Z",
				}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(
			t, deps, "extension", "remove", "spec-cycle", "--permanent", "--global", "-o", "json",
		)
		if err != nil {
			t.Fatalf("extension remove --permanent -o json error = %v", err)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(remove) error = %v; stdout=%s", err, stdout)
		}
		if payload["permanent"] != true || payload["tombstoned_at"] != "2026-11-17T14:02:11Z" {
			t.Fatalf("permanent JSON = %#v, want permanent true and tombstoned_at set", payload)
		}
	})

	t.Run(
		"Should keep the non-permanent JSON fields and add permanent false with an empty tombstone time",
		func(t *testing.T) {
			t.Parallel()

			deps, _ := newExtensionLocalDeps(t, &stubClient{
				removeExtensionFn: func(_ context.Context, name string) (ManagedExtensionRemoveRecord, error) {
					return ManagedExtensionRemoveRecord{Name: name, Path: "/tmp/remove-ext", Status: "removed"}, nil
				},
			})
			markExtensionDaemonRunning(&deps)

			stdout, _, err := executeRootCommand(t, deps, "extension", "remove", "remove-ext", "--global", "-o", "json")
			if err != nil {
				t.Fatalf("extension remove -o json error = %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("json.Unmarshal(remove) error = %v; stdout=%s", err, stdout)
			}
			if payload["name"] != "remove-ext" || payload["status"] != "removed" ||
				payload["path"] != "/tmp/remove-ext" {
				t.Fatalf("non-permanent JSON = %#v, want the existing fields unchanged", payload)
			}
			if payload["permanent"] != false || payload["tombstoned_at"] != "" {
				t.Fatalf("non-permanent JSON = %#v, want permanent false and empty tombstoned_at", payload)
			}
		},
	)

	t.Run(
		"Should refuse --permanent with --workspace as invalid usage before contacting the daemon",
		func(t *testing.T) {
			t.Parallel()

			calls := 0
			deps, _ := newExtensionLocalDeps(t, &stubClient{
				removePermanentExtensionFn: func(context.Context, string) (ManagedExtensionRemoveRecord, error) {
					calls++
					return ManagedExtensionRemoveRecord{}, nil
				},
			})
			markExtensionDaemonRunning(&deps)

			_, _, err := executeRootCommand(
				t, deps, "extension", "remove", "spec-cycle", "--permanent", "--workspace", "ws-1",
			)
			if err == nil {
				t.Fatal("extension remove --permanent --workspace error = nil, want invalid usage")
			}
			if got := cliExitCodeForError(err); got != 2 {
				t.Fatalf("exit code = %d, want 2 for invalid usage", got)
			}
			if calls != 0 {
				t.Fatalf("RemovePermanentExtension calls = %d, want 0 for invalid usage", calls)
			}
		},
	)

	t.Run("Should exit 1 when the daemon refuses permanent removal", func(t *testing.T) {
		t.Parallel()

		refusals := []*extensionOperationAPIError{
			{
				statusCode: http.StatusUnprocessableEntity,
				status:     "422 Unprocessable Entity",
				payload: contract.ExtensionOperationErrorPayload{
					Error: "dev-cycle is not a built-in",
					Code:  diagnosticcontract.CodeExtensionNotBuiltin,
				},
			},
			{
				statusCode: http.StatusNotFound,
				status:     "404 Not Found",
				payload:    contract.ExtensionOperationErrorPayload{Error: "spec-cycle is not installed"},
			},
			{
				statusCode: http.StatusConflict,
				status:     "409 Conflict",
				payload:    contract.ExtensionOperationErrorPayload{Error: "concurrent mutation"},
			},
		}
		for _, refusal := range refusals {
			deps, _ := newExtensionLocalDeps(t, &stubClient{
				removePermanentExtensionFn: func(context.Context, string) (ManagedExtensionRemoveRecord, error) {
					return ManagedExtensionRemoveRecord{}, refusal
				},
			})
			markExtensionDaemonRunning(&deps)

			_, _, err := executeRootCommand(t, deps, "extension", "remove", "spec-cycle", "--permanent")
			if err == nil {
				t.Fatalf("extension remove --permanent error = nil for %v, want a refusal", refusal)
			}
			if got := cliExitCodeForError(err); got != 1 {
				t.Fatalf("exit code = %d for %v, want 1", got, refusal)
			}
			apiErr, ok := errors.AsType[*extensionOperationAPIError](err)
			if !ok || apiErr.payload.Code != refusal.payload.Code {
				t.Fatalf("refusal diagnostic = %v, want code %q from the daemon", err, refusal.payload.Code)
			}
			if !strings.Contains(err.Error(), refusal.payload.Error) {
				t.Fatalf("refusal message = %q, want the daemon message %q", err.Error(), refusal.payload.Error)
			}
		}
	})

	t.Run("Should send no profile selection with permanent removal when --profile is set", func(t *testing.T) {
		t.Parallel()

		var profileSeen string
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			resolveWorkspaceRefs: true,
			listProfilesFn: func(context.Context) ([]contract.Profile, error) {
				return []contract.Profile{
					{Name: configDefaultKey, State: authoredContextActiveKey},
					{Name: "team", State: authoredContextActiveKey},
				}, nil
			},
			removePermanentExtensionFn: func(ctx context.Context, name string) (ManagedExtensionRemoveRecord, error) {
				profileSeen = profileQueryValues(ctx, nil).Get(profileFlagName)
				return ManagedExtensionRemoveRecord{Name: name, Status: "removed", Permanent: true}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		_, _, err := executeRootCommand(
			t, deps, "extension", "remove", "spec-cycle", "--permanent", "--profile", "team",
		)
		if err != nil {
			t.Fatalf("extension remove --permanent --profile error = %v", err)
		}
		// The daemon answers a profile query on permanent removal with 400, so the request must carry none.
		if profileSeen != "" {
			t.Fatalf("permanent removal request profile = %q, want none", profileSeen)
		}
	})

	t.Run("Should exit 0 when permanent removal succeeds", func(t *testing.T) {
		t.Parallel()

		deps, _ := newExtensionLocalDeps(t, &stubClient{
			removePermanentExtensionFn: func(_ context.Context, name string) (ManagedExtensionRemoveRecord, error) {
				return ManagedExtensionRemoveRecord{Name: name, Status: "removed", Permanent: true}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		_, _, err := executeRootCommand(t, deps, "extension", "remove", "spec-cycle", "--permanent")
		if got := cliExitCodeForError(err); err != nil || got != 0 {
			t.Fatalf("extension remove --permanent err = %v, exit = %d; want success", err, got)
		}
	})
}

func TestExtensionRemovePermanentNeverFallsBackToNonPermanentRoute(t *testing.T) {
	t.Parallel()

	t.Run("Should not degrade a failed permanent removal to the plain route", func(t *testing.T) {
		t.Parallel()

		// Invariant: --permanent never degrades to the non-permanent DELETE. When the permanent call fails,
		// the command fails with exit 1 and the plain removal is not attempted.
		removeCalls := 0
		deps, _ := newExtensionLocalDeps(t, &stubClient{
			removeExtensionFn: func(context.Context, string) (ManagedExtensionRemoveRecord, error) {
				removeCalls++
				return ManagedExtensionRemoveRecord{Name: "spec-cycle", Status: "removed"}, nil
			},
			removePermanentExtensionFn: func(context.Context, string) (ManagedExtensionRemoveRecord, error) {
				return ManagedExtensionRemoveRecord{}, errors.New("daemon refused")
			},
		})
		markExtensionDaemonRunning(&deps)
		_, _, err := executeRootCommand(t, deps, "extension", "remove", "spec-cycle", "--permanent")
		if err == nil || !strings.Contains(err.Error(), "daemon refused") {
			t.Fatalf("extension remove --permanent error = %v, want the daemon refusal", err)
		}
		if got := cliExitCodeForError(err); got != 1 {
			t.Fatalf("exit code = %d, want 1", got)
		}
		if removeCalls != 0 {
			t.Fatalf("RemoveExtension calls = %d, want 0 after a failed permanent removal", removeCalls)
		}
	})
}
