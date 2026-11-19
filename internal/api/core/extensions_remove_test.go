package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// removeStub overrides the removal methods of the shared extension stub so each case controls them.
// Without RemovePermanently it models a service that lacks the permanent-removal capability.
type removeStub struct {
	extensionServiceStub
	removeFn    func(context.Context, string, taskpkg.ActorContext) (contract.ManagedExtensionRemovePayload, error)
	permanentFn func(context.Context, string, taskpkg.ActorContext) (contract.ManagedExtensionRemovePayload, error)
}

func (s removeStub) Remove(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if s.removeFn != nil {
		return s.removeFn(ctx, name, actor)
	}
	return contract.ManagedExtensionRemovePayload{}, nil
}

func (s removeStub) RemovePermanently(
	ctx context.Context,
	name string,
	actor taskpkg.ActorContext,
) (contract.ManagedExtensionRemovePayload, error) {
	if s.permanentFn != nil {
		return s.permanentFn(ctx, name, actor)
	}
	return contract.ManagedExtensionRemovePayload{}, nil
}

// removeEngine mounts the DELETE route the same way the HTTP and UDS registrations do.
func removeEngine(service core.ExtensionService) *gin.Engine {
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{Extensions: service})
	engine := gin.New()
	engine.DELETE("/extensions/:name", handlers.RemoveExtension)
	return engine
}

func TestRemoveExtensionNonPermanentKeepsPayloadWithoutTombstone(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should keep the existing payload fields and report permanent false with an empty tombstone time",
		func(t *testing.T) {
			t.Parallel()

			permanentCalls := 0
			service := removeStub{
				removeFn: func(
					_ context.Context,
					name string,
					_ taskpkg.ActorContext,
				) (contract.ManagedExtensionRemovePayload, error) {
					return contract.ManagedExtensionRemovePayload{
						Name:   name,
						Path:   "/home/operator/.compozy/extensions/" + name,
						Status: "removed",
					}, nil
				},
				permanentFn: func(
					context.Context,
					string,
					taskpkg.ActorContext,
				) (contract.ManagedExtensionRemovePayload, error) {
					permanentCalls++
					return contract.ManagedExtensionRemovePayload{}, nil
				},
			}
			response := performRequest(t, removeEngine(service), http.MethodDelete, "/extensions/spec-cycle", nil)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
			}
			var raw struct {
				Extension map[string]any `json:"extension"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
				t.Fatalf("json.Unmarshal(remove) error = %v", err)
			}
			if raw.Extension["name"] != "spec-cycle" || raw.Extension["status"] != "removed" ||
				raw.Extension["path"] != "/home/operator/.compozy/extensions/spec-cycle" {
				t.Fatalf("remove payload = %#v, want the existing fields unchanged", raw.Extension)
			}
			if permanent, ok := raw.Extension["permanent"].(bool); !ok || permanent {
				t.Fatalf("permanent = %#v, want false", raw.Extension["permanent"])
			}
			if tombstonedAt, ok := raw.Extension["tombstoned_at"].(string); !ok || tombstonedAt != "" {
				t.Fatalf("tombstoned_at = %#v, want empty string", raw.Extension["tombstoned_at"])
			}
			if permanentCalls != 0 {
				t.Fatalf("RemovePermanently calls = %d, want 0 for a non-permanent remove", permanentCalls)
			}
		},
	)
}

func TestRemoveExtensionPermanentStatusCodes(t *testing.T) {
	t.Parallel()

	t.Run("Should dispatch permanent removal and return the tombstone time", func(t *testing.T) {
		t.Parallel()

		removeCalls := 0
		service := removeStub{
			removeFn: func(
				context.Context,
				string,
				taskpkg.ActorContext,
			) (contract.ManagedExtensionRemovePayload, error) {
				removeCalls++
				return contract.ManagedExtensionRemovePayload{}, nil
			},
			permanentFn: func(
				_ context.Context,
				name string,
				_ taskpkg.ActorContext,
			) (contract.ManagedExtensionRemovePayload, error) {
				return contract.ManagedExtensionRemovePayload{
					Name:         name,
					Status:       "removed",
					Permanent:    true,
					TombstonedAt: "2026-11-17T14:02:11Z",
				}, nil
			},
		}
		response := performRequest(
			t, removeEngine(service), http.MethodDelete, "/extensions/spec-cycle?permanent=true", nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ExtensionRemoveResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(remove) error = %v", err)
		}
		if !payload.Extension.Permanent || payload.Extension.TombstonedAt != "2026-11-17T14:02:11Z" {
			t.Fatalf("permanent payload = %#v", payload.Extension)
		}
		if removeCalls != 0 {
			t.Fatalf("Remove calls = %d, want 0 for permanent removal", removeCalls)
		}
	})

	t.Run("Should return 422 extension_not_builtin for a non-built-in row", func(t *testing.T) {
		t.Parallel()

		service := removeStub{permanentFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ManagedExtensionRemovePayload, error) {
			return contract.ManagedExtensionRemovePayload{}, &extensionpkg.ExtensionNotBuiltinError{
				Name:   "dev-cycle",
				Source: "marketplace",
			}
		}}
		response := performRequest(
			t, removeEngine(service), http.MethodDelete, "/extensions/dev-cycle?permanent=true", nil,
		)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ExtensionOperationErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(error) error = %v", err)
		}
		if payload.Code != diagnosticcontract.CodeExtensionNotBuiltin {
			t.Fatalf("code = %q, want %q", payload.Code, diagnosticcontract.CodeExtensionNotBuiltin)
		}
	})

	t.Run("Should return 404 for an unknown name", func(t *testing.T) {
		t.Parallel()

		service := removeStub{permanentFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ManagedExtensionRemovePayload, error) {
			return contract.ManagedExtensionRemovePayload{}, &extensionpkg.ExtensionNotFoundError{Name: "missing"}
		}}
		response := performRequest(
			t, removeEngine(service), http.MethodDelete, "/extensions/missing?permanent=true", nil,
		)
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("Should refuse permanent removal combined with a workspace or profile scope", func(t *testing.T) {
		t.Parallel()

		calls := 0
		service := removeStub{permanentFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ManagedExtensionRemovePayload, error) {
			calls++
			return contract.ManagedExtensionRemovePayload{}, nil
		}}
		engine := removeEngine(service)
		for _, path := range []string{
			"/extensions/spec-cycle?permanent=true&workspace=ws-1",
			"/extensions/spec-cycle?permanent=true&profile=work",
		} {
			response := performRequest(t, engine, http.MethodDelete, path, nil)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d, want 400; body=%s", path, response.Code, response.Body.String())
			}
		}
		if calls != 0 {
			t.Fatalf("RemovePermanently calls = %d, want 0 for scoped permanent removal", calls)
		}
	})

	t.Run("Should refuse a permanent flag that is not a boolean", func(t *testing.T) {
		t.Parallel()

		calls := 0
		service := removeStub{permanentFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ManagedExtensionRemovePayload, error) {
			calls++
			return contract.ManagedExtensionRemovePayload{}, nil
		}}
		response := performRequest(
			t, removeEngine(service), http.MethodDelete, "/extensions/spec-cycle?permanent=maybe", nil,
		)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body=%s", response.Code, response.Body.String())
		}
		if calls != 0 {
			t.Fatalf("RemovePermanently calls = %d, want 0 for an invalid flag", calls)
		}
	})

	t.Run("Should return 501 when the service lacks the permanent removal capability", func(t *testing.T) {
		t.Parallel()

		// The plain stub has no RemovePermanently method, so it models a service without the capability.
		service := extensionServiceStub{}
		response := performRequest(
			t, removeEngine(service), http.MethodDelete, "/extensions/spec-cycle?permanent=true", nil,
		)
		if response.Code != http.StatusNotImplemented {
			t.Fatalf("status = %d, want 501; body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("Should return 403 when a non-operator actor is denied permanent removal", func(t *testing.T) {
		t.Parallel()

		service := removeStub{permanentFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ManagedExtensionRemovePayload, error) {
			return contract.ManagedExtensionRemovePayload{}, taskpkg.ErrPermissionDenied
		}}
		response := performRequest(
			t, removeEngine(service), http.MethodDelete, "/extensions/spec-cycle?permanent=true", nil,
		)
		if response.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body=%s", response.Code, response.Body.String())
		}
	})

	t.Run("Should map an unexpected service failure to 500", func(t *testing.T) {
		t.Parallel()

		service := removeStub{permanentFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ManagedExtensionRemovePayload, error) {
			return contract.ManagedExtensionRemovePayload{}, errors.New("disk unavailable")
		}}
		response := performRequest(
			t, removeEngine(service), http.MethodDelete, "/extensions/spec-cycle?permanent=true", nil,
		)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500; body=%s", response.Code, response.Body.String())
		}
	})
}

// Invariant: a removal whose compensation failed answers 500 extension_removal_compensation_failed. The suggested
// command is restore when a built-in's tombstone remains, and doctor otherwise (Safety Invariant 1).
func TestRemoveExtensionCompensationFailureCode(t *testing.T) {
	t.Parallel()

	t.Run("Should suggest restore when a permanent removal kept its tombstone", func(t *testing.T) {
		t.Parallel()

		service := removeStub{permanentFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ManagedExtensionRemovePayload, error) {
			return contract.ManagedExtensionRemovePayload{}, errors.Join(
				extensionpkg.ErrExtensionBuiltinRemovalCompensationFailed,
				extensionpkg.ErrExtensionRemovalCompensationFailed,
			)
		}}
		payload := assertRemoveCompensationFailure(t, removeEngine(service), "/extensions/spec-cycle?permanent=true")
		if payload.Diagnostic == nil || payload.Diagnostic.SuggestedCommand != "compozy extension restore spec-cycle" {
			t.Fatalf("diagnostic = %#v, want the restore command for spec-cycle", payload.Diagnostic)
		}
	})

	t.Run("Should suggest doctor when a non-permanent removal could not be compensated", func(t *testing.T) {
		t.Parallel()

		service := removeStub{removeFn: func(
			context.Context,
			string,
			taskpkg.ActorContext,
		) (contract.ManagedExtensionRemovePayload, error) {
			return contract.ManagedExtensionRemovePayload{}, extensionpkg.ErrExtensionRemovalCompensationFailed
		}}
		payload := assertRemoveCompensationFailure(t, removeEngine(service), "/extensions/spec-cycle")
		if payload.Diagnostic == nil || payload.Diagnostic.SuggestedCommand != "compozy doctor" {
			t.Fatalf("diagnostic = %#v, want the doctor command", payload.Diagnostic)
		}
	})
}

func assertRemoveCompensationFailure(
	t *testing.T,
	engine *gin.Engine,
	target string,
) contract.ExtensionOperationErrorPayload {
	t.Helper()
	response := performRequest(t, engine, http.MethodDelete, target, nil)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", response.Code, response.Body.String())
	}
	var payload contract.ExtensionOperationErrorPayload
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(error) error = %v", err)
	}
	if payload.Code != diagnosticcontract.CodeExtensionRemovalCompensationFailed {
		t.Fatalf("code = %q, want %q; body=%s", payload.Code, diagnosticcontract.CodeExtensionRemovalCompensationFailed,
			response.Body.String())
	}
	return payload
}
