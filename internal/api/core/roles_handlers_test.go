package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func TestRoleStatusHandlers(t *testing.T) {
	t.Parallel()

	t.Run("Should list the provider projection for the requested workspace", func(t *testing.T) {
		t.Parallel()

		provider := &rolesStatusProviderStub{
			statuses: []contract.RoleStatus{
				{
					Role:          "auto_title",
					FallbackChain: []contract.RoleFallbackStatus{},
					Diagnostics:   []contract.RoleDiagnostic{},
				},
				{
					Role:          "checkpoint_summary",
					FallbackChain: []contract.RoleFallbackStatus{},
					Diagnostics:   []contract.RoleDiagnostic{},
				},
				{
					Role:          "coordinator",
					FallbackChain: []contract.RoleFallbackStatus{},
					Diagnostics:   []contract.RoleDiagnostic{},
				},
				{
					Role:          "dream",
					FallbackChain: []contract.RoleFallbackStatus{},
					Diagnostics:   []contract.RoleDiagnostic{},
				},
				{
					Role:          "memory_controller",
					FallbackChain: []contract.RoleFallbackStatus{},
					Diagnostics:   []contract.RoleDiagnostic{},
				},
				{
					Role:          "memory_extractor",
					FallbackChain: []contract.RoleFallbackStatus{},
					Diagnostics:   []contract.RoleDiagnostic{},
				},
			},
		}
		engine := roleStatusHandlerEngine(provider)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/roles?workspace=ws-a", http.NoBody)
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("GET /api/roles status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var payload contract.RolesResponse
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(RolesResponse) error = %v", err)
		}
		wantRoles := []string{
			"auto_title",
			"checkpoint_summary",
			"coordinator",
			"dream",
			"memory_controller",
			"memory_extractor",
		}
		if len(payload.Roles) != len(wantRoles) || provider.workspace != "ws-a" {
			t.Fatalf("roles/workspace = %d/%q, want %d/ws-a", len(payload.Roles), provider.workspace, len(wantRoles))
		}
		for index, wantRole := range wantRoles {
			if payload.Roles[index].Role != wantRole {
				t.Fatalf("roles[%d].Role = %q, want %q", index, payload.Roles[index].Role, wantRole)
			}
		}
	})

	t.Run("Should return role_unknown for an unsupported role", func(t *testing.T) {
		t.Parallel()

		provider := &rolesStatusProviderStub{statusErr: roleStatusDiagnosticError{code: contract.CodeRoleUnknown}}
		engine := roleStatusHandlerEngine(provider)
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/roles/judge", http.NoBody)
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("GET /api/roles/judge status = %d, want 404; body=%s", response.Code, response.Body.String())
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(ErrorPayload) error = %v", err)
		}
		if payload.Diagnostic == nil || payload.Diagnostic.Code != contract.CodeRoleUnknown {
			t.Fatalf("GET /api/roles/judge payload = %#v, want role_unknown", payload)
		}
		if provider.role != "judge" {
			t.Fatalf("RoleStatus role = %q, want judge", provider.role)
		}
	})
}

func roleStatusHandlerEngine(provider RolesStatusProvider) *gin.Engine {
	handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "test", Roles: provider})
	engine := gin.New()
	engine.GET("/api/roles", handlers.ListRoles)
	engine.GET("/api/roles/:role", handlers.GetRole)
	return engine
}

type rolesStatusProviderStub struct {
	statuses  []contract.RoleStatus
	status    contract.RoleStatus
	statusErr error
	workspace string
	role      string
}

func (s *rolesStatusProviderStub) RoleStatuses(
	_ context.Context,
	workspaceID string,
) ([]contract.RoleStatus, error) {
	s.workspace = workspaceID
	return append([]contract.RoleStatus(nil), s.statuses...), s.statusErr
}

func (s *rolesStatusProviderStub) RoleStatus(
	_ context.Context,
	workspaceID string,
	role string,
) (contract.RoleStatus, error) {
	s.workspace = workspaceID
	s.role = role
	return s.status, s.statusErr
}

type roleStatusDiagnosticError struct {
	code string
}

func (e roleStatusDiagnosticError) Error() string {
	return e.code
}

func (e roleStatusDiagnosticError) DiagnosticCode() string {
	return e.code
}
