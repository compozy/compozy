package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// restoreStub adds the restore and removed-listing capabilities to the shared extension stub.
type restoreStub struct {
	extensionServiceStub
	restoreFn func(context.Context, string, taskpkg.ActorContext) (contract.ExtensionRestorePayload, error)
	listFn    func(context.Context) ([]contract.ExtensionRemovedBuiltinPayload, error)
}

func (s restoreStub) RestoreBuiltin(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ExtensionRestorePayload, error) {
	return s.restoreFn(ctx, name, actor)
}

func (s restoreStub) ListRemovedBuiltins(ctx context.Context) ([]contract.ExtensionRemovedBuiltinPayload, error) {
	return s.listFn(ctx)
}

// restoreEngine mounts the restore and removed-listing routes the way the HTTP and UDS registrations do.
// The listing is static and the restore is a `:name` child, so both coexist with GET /extensions/:name.
func restoreEngine(service core.ExtensionService) *gin.Engine {
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
	engine := gin.New()
	engine.GET("/extensions/removed-builtins", handlers.ListRemovedBuiltinExtensions)
	engine.GET("/extensions/:name", handlers.ExtensionStatus)
	engine.POST("/extensions/:name/restore", handlers.RestoreExtension)
	return engine
}

func TestRestoreExtensionStatusCodes(t *testing.T) {
	t.Parallel()

	t.Run("Should return the restored payload with 200", func(t *testing.T) {
		t.Parallel()

		service := restoreStub{restoreFn: func(
			_ context.Context,
			name string,
			_ taskpkg.ActorContext,
		) (contract.ExtensionRestorePayload, error) {
			return contract.ExtensionRestorePayload{
				Name:    name,
				Status:  "restored",
				Source:  "bundled",
				Version: "0.7.3",
				Enabled: true,
				Path:    "/home/operator/.compozy/extensions/spec-cycle",
			}, nil
		}}
		response := performRequest(t, restoreEngine(service), http.MethodPost, "/extensions/spec-cycle/restore", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ExtensionRestorePayload
		decodeRestoreResponse(t, response, &payload)
		if payload.Name != "spec-cycle" || payload.Status != "restored" || payload.Source != "bundled" ||
			payload.Version != "0.7.3" || !payload.Enabled {
			t.Fatalf("restore payload = %#v", payload)
		}
	})

	t.Run("Should return 409 extension_builtin_not_removed when no tombstone exists", func(t *testing.T) {
		t.Parallel()

		service := restoreStub{restoreFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ExtensionRestorePayload, error) {
			return contract.ExtensionRestorePayload{}, errors.Join(extensionpkg.ErrExtensionBuiltinNotRemoved)
		}}
		assertRestoreError(t, service, http.StatusConflict, diagnosticcontract.CodeExtensionBuiltinNotRemoved)
	})

	t.Run("Should return 409 extension_exists with the remove action when a copy is installed", func(t *testing.T) {
		t.Parallel()

		service := restoreStub{restoreFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ExtensionRestorePayload, error) {
			return contract.ExtensionRestorePayload{}, extensionpkg.ErrExtensionExists
		}}
		response := assertRestoreError(t, service, http.StatusConflict, diagnosticcontract.CodeExtensionExists)
		var payload contract.ExtensionOperationErrorPayload
		decodeRestoreResponse(t, response, &payload)
		if payload.Diagnostic == nil || payload.Diagnostic.SuggestedCommand != "compozy extension remove spec-cycle" {
			t.Fatalf("diagnostic = %#v, want the remove action for spec-cycle", payload.Diagnostic)
		}
	})

	t.Run("Should return 500 extension_restore_failed even when the cause is a not-found error", func(t *testing.T) {
		t.Parallel()

		// The failure wraps a not-found cause; the restore kind must win so the status stays 500.
		service := restoreStub{restoreFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ExtensionRestorePayload, error) {
			return contract.ExtensionRestorePayload{}, errors.Join(
				extensionpkg.ErrExtensionRestoreFailed,
				&extensionpkg.ExtensionNotFoundError{Name: "spec-cycle"},
			)
		}}
		assertRestoreError(t, service, http.StatusInternalServerError, diagnosticcontract.CodeExtensionRestoreFailed)
	})

	t.Run("Should return 500 extension_restore_failed even when the cause requires inputs", func(t *testing.T) {
		t.Parallel()

		// The restore kind must outrank the acquisition kind, which would answer 422 for the inputs cause.
		service := restoreStub{restoreFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ExtensionRestorePayload, error) {
			return contract.ExtensionRestorePayload{}, errors.Join(
				extensionpkg.ErrExtensionRestoreFailed,
				extensionpkg.ErrExtensionInputsRequired,
			)
		}}
		assertRestoreError(t, service, http.StatusInternalServerError, diagnosticcontract.CodeExtensionRestoreFailed)
	})

	t.Run("Should return 404 for a name that is neither removed nor a known built-in", func(t *testing.T) {
		t.Parallel()

		service := restoreStub{restoreFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ExtensionRestorePayload, error) {
			return contract.ExtensionRestorePayload{}, &extensionpkg.ExtensionNotFoundError{Name: "missing"}
		}}
		response := performRequest(t, restoreEngine(service), http.MethodPost, "/extensions/missing/restore", nil)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("Should return 501 when the service cannot restore built-ins", func(t *testing.T) {
		t.Parallel()

		response := performRequest(
			t, restoreEngine(extensionServiceStub{}), http.MethodPost, "/extensions/spec-cycle/restore", nil,
		)
		if response.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want 501; body=%s", response.Code, response.Body.String())
		}
	})
}

func assertRestoreError(
	t *testing.T,
	service core.ExtensionService,
	wantStatus int,
	wantCode string,
) *httptest.ResponseRecorder {
	t.Helper()
	response := performRequest(t, restoreEngine(service), http.MethodPost, "/extensions/spec-cycle/restore", nil)
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, wantStatus, response.Body.String())
	}
	var payload contract.ExtensionOperationErrorPayload
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(error) error = %v", err)
	}
	if payload.Code != wantCode {
		t.Fatalf("code = %q, want %q; body=%s", payload.Code, wantCode, response.Body.String())
	}
	return response
}

func TestListRemovedBuiltinExtensions(t *testing.T) {
	t.Parallel()

	t.Run("Should render an empty listing as an empty array with 200", func(t *testing.T) {
		t.Parallel()

		service := restoreStub{listFn: func(context.Context) ([]contract.ExtensionRemovedBuiltinPayload, error) {
			return nil, nil
		}}
		response := performRequest(t, restoreEngine(service), http.MethodGet, "/extensions/removed-builtins", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		if body := strings.TrimSpace(response.Body.String()); body != `{"removed":[]}` {
			t.Fatalf("body = %s, want {\"removed\":[]}", body)
		}
	})

	t.Run("Should list removed built-ins with the DTO fields", func(t *testing.T) {
		t.Parallel()

		service := restoreStub{listFn: func(context.Context) ([]contract.ExtensionRemovedBuiltinPayload, error) {
			return []contract.ExtensionRemovedBuiltinPayload{{
				Name: "spec-cycle", RemovedAt: "2026-11-17T14:02:11Z", RemovedBy: "operator",
			}}, nil
		}}
		response := performRequest(t, restoreEngine(service), http.MethodGet, "/extensions/removed-builtins", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ExtensionRemovedBuiltinsResponse
		decodeRestoreResponse(t, response, &payload)
		if len(payload.Removed) != 1 || payload.Removed[0].Name != "spec-cycle" ||
			payload.Removed[0].RemovedAt != "2026-11-17T14:02:11Z" || payload.Removed[0].RemovedBy != "operator" {
			t.Fatalf("removed = %#v", payload.Removed)
		}
	})

	t.Run("Should return 501 when the service cannot list removed built-ins", func(t *testing.T) {
		t.Parallel()

		response := performRequest(
			t, restoreEngine(extensionServiceStub{}), http.MethodGet, "/extensions/removed-builtins", nil,
		)
		if response.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want 501; body=%s", response.Code, response.Body.String())
		}
	})
}

func TestExtensionStatusCodeForRestoreKinds(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{name: "not removed is a conflict", err: extensionpkg.ErrExtensionBuiltinNotRemoved, want: http.StatusConflict},
		{name: "existing copy is a conflict", err: extensionpkg.ErrExtensionExists, want: http.StatusConflict},
		{
			name: "restore failure is internal even with a not-found cause",
			err: errors.Join(
				extensionpkg.ErrExtensionRestoreFailed, &extensionpkg.ExtensionNotFoundError{Name: "x"},
			),
			want: http.StatusInternalServerError,
		},
	} {
		t.Run("Should map "+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := core.ExtensionStatusCode(tc.err); got != tc.want {
				t.Fatalf("ExtensionStatusCode() = %d, want %d", got, tc.want)
			}
		})
	}
}

func decodeRestoreResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", response.Body.String(), err)
	}
}
