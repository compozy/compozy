package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// The HTTP and UDS suites assert these same literals, so both transports publish identical bytes.
const (
	httpRemovedBuiltinsBody = `{"removed":[{"name":"spec-cycle","removed_at":"2026-11-17T14:02:11Z",` +
		`"removed_by":"operator"}]}`
	httpRestoreBody = `{"name":"spec-cycle","status":"restored","source":"bundled","version":"0.7.3",` +
		`"enabled":true,"path":"/home/operator/.compozy/extensions/spec-cycle"}`
)

// UT-011 over HTTP: the static removed-builtins path is the listing, and the restore POST is a `:name` child.
func TestExtensionRestoreRoutesHTTP(t *testing.T) {
	t.Parallel()

	operator, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator", "workspace-a", taskpkg.OriginKindHTTP, "extensions.restore",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}

	newEngine := func(service stubExtensionService) *gin.Engine {
		handlers := newTestHandlersWithSettingsAndExtensions(
			t, "127.0.0.1", nil, nil, service, newTestHomePaths(t),
		)
		handlers.TaskActorContextResolver = func(_ *gin.Context, _ string) (taskpkg.ActorContext, error) {
			return operator, nil
		}
		return newTestRouter(t, handlers)
	}

	t.Run("Should serve GET removed-builtins from the listing and never the :name status handler", func(t *testing.T) {
		t.Parallel()

		engine := newEngine(stubExtensionService{
			StatusFn: func(context.Context, string) (contract.ExtensionPayload, error) {
				t.Error("GET /api/extensions/removed-builtins reached the :name status handler")
				return contract.ExtensionPayload{}, nil
			},
			ListRemovedFn: func(context.Context) ([]contract.ExtensionRemovedBuiltinPayload, error) {
				return []contract.ExtensionRemovedBuiltinPayload{{
					Name: "spec-cycle", RemovedAt: "2026-11-17T14:02:11Z", RemovedBy: "operator",
				}}, nil
			},
		})
		recorder := performRequest(t, engine, http.MethodGet, "/api/extensions/removed-builtins", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		if got := strings.TrimSpace(recorder.Body.String()); got != httpRemovedBuiltinsBody {
			t.Fatalf("body = %s, want %s", got, httpRemovedBuiltinsBody)
		}
	})

	t.Run("Should render an empty removed listing as an empty array", func(t *testing.T) {
		t.Parallel()

		engine := newEngine(stubExtensionService{})
		recorder := performRequest(t, engine, http.MethodGet, "/api/extensions/removed-builtins", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		if got := strings.TrimSpace(recorder.Body.String()); got != `{"removed":[]}` {
			t.Fatalf("body = %s, want {\"removed\":[]}", got)
		}
	})

	t.Run("Should restore through the privileged POST route with the same bytes as UDS", func(t *testing.T) {
		t.Parallel()

		var gotName string
		engine := newEngine(stubExtensionService{
			RestoreFn: func(
				_ context.Context,
				name string,
				_ taskpkg.ActorContext,
			) (contract.ExtensionRestorePayload, error) {
				gotName = name
				return contract.ExtensionRestorePayload{
					Name:    name,
					Status:  "restored",
					Source:  "bundled",
					Version: "0.7.3",
					Enabled: true,
					Path:    "/home/operator/.compozy/extensions/spec-cycle",
				}, nil
			},
		})
		recorder := performRequest(t, engine, http.MethodPost, "/api/extensions/spec-cycle/restore", nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		if gotName != "spec-cycle" {
			t.Fatalf("RestoreBuiltin name = %q, want spec-cycle", gotName)
		}
		if got := strings.TrimSpace(recorder.Body.String()); got != httpRestoreBody {
			t.Fatalf("body = %s, want %s", got, httpRestoreBody)
		}
	})

	t.Run("Should refuse restore from a non-loopback bound host like other privileged writes", func(t *testing.T) {
		t.Parallel()

		handlers := newTestHandlersWithSettingsAndExtensions(
			t, "10.0.0.7", nil, nil, stubExtensionService{}, newTestHomePaths(t),
		)
		engine := gin.New()
		engine.Use(gin.Recovery())
		RegisterRoutes(engine, handlers)
		recorder := performRequest(t, engine, http.MethodPost, "/api/extensions/spec-cycle/restore", nil)
		// The route is registered, so the guard answers before the handler; 404 would mean it is missing.
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403 from the loopback guard; body=%s", recorder.Code, recorder.Body.String())
		}
	})
}
