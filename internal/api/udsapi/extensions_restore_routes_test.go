package udsapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	udsRemovedBuiltinsBody = `{"removed":[{"name":"spec-cycle","removed_at":"2026-11-17T14:02:11Z",` +
		`"removed_by":"operator"}]}`
	udsRestoreBody = `{"name":"spec-cycle","status":"restored","source":"bundled","version":"0.7.3",` +
		`"enabled":true,"path":"/home/operator/.compozy/extensions/spec-cycle"}`
)

// UT-011: the static removed-builtins path resolves to its listing through the real UDS registration,
// and the restore POST is a `:name` child that leaves GET /extensions/:name intact.
func TestExtensionRestoreRouteOrdering(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should resolve GET removed-builtins to the listing and never to the name status handler",
		func(t *testing.T) {
			t.Parallel()

			homePaths := newTestHomePaths(t)
			handlers := newTestHandlersWithExtensions(t, &stubSessionManager{}, stubObserver{}, stubExtensionService{
				StatusFn: func(context.Context, string) (contract.ExtensionPayload, error) {
					t.Error("GET /api/extensions/removed-builtins reached the :name status handler")
					return contract.ExtensionPayload{}, nil
				},
				ListRemovedFn: func(context.Context) ([]contract.ExtensionRemovedBuiltinPayload, error) {
					return []contract.ExtensionRemovedBuiltinPayload{{
						Name: "spec-cycle", RemovedAt: "2026-11-17T14:02:11Z", RemovedBy: "operator",
					}}, nil
				},
			}, homePaths)
			engine := newTestRouter(t, handlers)

			recorder := performRequest(t, engine, http.MethodGet, "/api/extensions/removed-builtins", nil)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
			}
			if got := strings.TrimSpace(recorder.Body.String()); got != udsRemovedBuiltinsBody {
				t.Fatalf("body = %s, want %s", got, udsRemovedBuiltinsBody)
			}
		},
	)

	t.Run("Should render an empty removed listing as an empty array", func(t *testing.T) {
		t.Parallel()

		handlers := newTestHandlersWithExtensions(
			t, &stubSessionManager{}, stubObserver{}, stubExtensionService{}, newTestHomePaths(t),
		)
		engine := newTestRouter(t, handlers)

		recorder := performRequest(t, engine, http.MethodGet, "/api/extensions/removed-builtins", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		if got := strings.TrimSpace(recorder.Body.String()); got != `{"removed":[]}` {
			t.Fatalf("body = %s, want {\"removed\":[]}", got)
		}
	})

	t.Run("Should route POST restore to the restore handler with the path name", func(t *testing.T) {
		t.Parallel()

		var gotName string
		var gotActor taskpkg.ActorContext
		handlers := newTestHandlersWithExtensions(t, &stubSessionManager{}, stubObserver{}, stubExtensionService{
			RestoreFn: func(
				_ context.Context,
				name string,
				actor taskpkg.ActorContext,
			) (contract.ExtensionRestorePayload, error) {
				gotName, gotActor = name, actor
				return contract.ExtensionRestorePayload{
					Name:    name,
					Status:  "restored",
					Source:  "bundled",
					Version: "0.7.3",
					Enabled: true,
					Path:    "/home/operator/.compozy/extensions/spec-cycle",
				}, nil
			},
		}, newTestHomePaths(t))
		engine := newTestRouter(t, handlers)

		recorder := performRequest(t, engine, http.MethodPost, "/api/extensions/spec-cycle/restore", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		if gotName != "spec-cycle" {
			t.Fatalf("RestoreBuiltin name = %q, want spec-cycle", gotName)
		}
		if gotActor.Actor.Kind == "" {
			t.Fatal("RestoreBuiltin received an empty actor")
		}
		if got := strings.TrimSpace(recorder.Body.String()); got != udsRestoreBody {
			t.Fatalf("body = %s, want %s", got, udsRestoreBody)
		}
	})

	t.Run("Should keep GET /api/extensions/:name on the status handler for ordinary names", func(t *testing.T) {
		t.Parallel()

		var gotName string
		handlers := newTestHandlersWithExtensions(t, &stubSessionManager{}, stubObserver{}, stubExtensionService{
			StatusFn: func(_ context.Context, name string) (contract.ExtensionPayload, error) {
				gotName = name
				return contract.ExtensionPayload{Name: name, State: "active"}, nil
			},
		}, newTestHomePaths(t))
		engine := newTestRouter(t, handlers)

		recorder := performRequest(t, engine, http.MethodGet, "/api/extensions/spec-cycle", nil)
		if recorder.Code != http.StatusOK || gotName != "spec-cycle" {
			t.Fatalf("status = %d name = %q, want 200 spec-cycle; body=%s",
				recorder.Code, gotName, recorder.Body.String())
		}
	})
}
