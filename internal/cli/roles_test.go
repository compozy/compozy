package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
)

func TestRolesCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should render the role roster as the public JSON contract", func(t *testing.T) {
		t.Parallel()

		provider := "anthropic"
		model := "claude-opus-4-1"
		roles := []RoleRecord{
			{Role: "auto_title", Enabled: true, ResolutionMode: contract.RoleResolutionModeInherit,
				FallbackChain: []contract.RoleFallbackStatus{}, Provenance: map[string]string{},
				Diagnostics: []contract.RoleDiagnostic{}},
			{Role: "checkpoint_summary", Enabled: true, ResolutionMode: contract.RoleResolutionModeBuiltin,
				FallbackChain: []contract.RoleFallbackStatus{}, Provenance: map[string]string{},
				Diagnostics: []contract.RoleDiagnostic{}},
			{Role: "coordinator", Enabled: true, ResolutionMode: contract.RoleResolutionModeBuiltin,
				Provider: &provider, Model: &model,
				FallbackChain: []contract.RoleFallbackStatus{{
					Provider: "backup", Model: "backup-model", ReasoningEffort: "low",
				}},
				Provenance: map[string]string{"provider": "global", "model": "workspace"},
				Diagnostics: []contract.RoleDiagnostic{{
					Code: "role_agent_not_found", Message: "configured agent is unavailable", Agent: "missing",
				}}},
			{Role: "dream", Enabled: true, ResolutionMode: contract.RoleResolutionModeBuiltin,
				FallbackChain: []contract.RoleFallbackStatus{}, Provenance: map[string]string{},
				Diagnostics: []contract.RoleDiagnostic{}},
			{Role: "memory_controller", Enabled: true, ResolutionMode: contract.RoleResolutionModeInherit,
				FallbackChain: []contract.RoleFallbackStatus{}, Provenance: map[string]string{},
				Diagnostics: []contract.RoleDiagnostic{}},
			{Role: "memory_extractor", Enabled: true, ResolutionMode: contract.RoleResolutionModeInherit,
				FallbackChain: []contract.RoleFallbackStatus{}, Provenance: map[string]string{},
				Diagnostics: []contract.RoleDiagnostic{}},
		}
		deps := newTestDeps(t, &stubClient{
			listRolesFn: func(_ context.Context, query RoleQuery) ([]RoleRecord, error) {
				if query.Workspace != "workspace-a" {
					t.Fatalf("ListRoles() workspace = %q, want workspace-a", query.Workspace)
				}
				return roles, nil
			},
		})

		stdout, stderr, err := executeRootCommand(
			t,
			deps,
			"roles", "list", "--workspace", "workspace-a", "-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(roles list) error = %v; stderr=%s", err, stderr)
		}
		var got []RoleRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(roles list) error = %v; stdout=%s", err, stdout)
		}
		if !reflect.DeepEqual(got, roles) {
			t.Fatalf("roles list = %#v, want %#v", got, roles)
		}
	})

	t.Run("Should show one workspace-scoped role as JSON", func(t *testing.T) {
		t.Parallel()

		agent := "catalog-agent"
		provider := "anthropic"
		model := "claude-sonnet"
		role := RoleRecord{
			Role:           "dream",
			Enabled:        true,
			ResolutionMode: contract.RoleResolutionModeCatalog,
			Agent:          &agent,
			Provider:       &provider,
			Model:          &model,
			FallbackChain: []contract.RoleFallbackStatus{{
				Provider: "backup", Model: "fallback-model", ReasoningEffort: "medium",
			}},
			Provenance: map[string]string{"agent": "workspace", "provider": "global"},
			Diagnostics: []contract.RoleDiagnostic{{
				Code: "role_warning", Message: "fallback configured", Agent: agent,
			}},
		}
		deps := newTestDeps(t, &stubClient{
			getRoleFn: func(_ context.Context, name string, query RoleQuery) (RoleRecord, error) {
				if name != "dream" || query.Workspace != "workspace-b" {
					t.Fatalf("GetRole() role/workspace = %q/%q, want dream/workspace-b", name, query.Workspace)
				}
				return role, nil
			},
		})

		stdout, stderr, err := executeRootCommand(
			t,
			deps,
			"roles", "show", "dream", "--workspace", "workspace-b", "-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(roles show) error = %v; stderr=%s", err, stderr)
		}
		var got RoleRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(roles show) error = %v; stdout=%s", err, stdout)
		}
		if !reflect.DeepEqual(got, role) {
			t.Fatalf("roles show = %#v, want %#v", got, role)
		}
	})

	t.Run("Should render role_unknown as a structured nonzero failure", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			getRoleFn: func(context.Context, string, RoleQuery) (RoleRecord, error) {
				return RoleRecord{}, &daemonAPIError{
					statusCode: http.StatusNotFound,
					payload: contract.ErrorPayload{
						Error: "role is not supported",
						Diagnostic: &contract.DiagnosticItem{
							Code:    contract.CodeRoleUnknown,
							Message: "role is not supported",
						},
					},
				}
			},
		})

		exitCode, stdout, stderr := executeRootCommandWithExit(
			t,
			deps,
			"roles", "show", "judge", "-o", "json",
		)
		if exitCode == 0 || stdout != "" {
			t.Fatalf("roles show unknown exit/stdout = %d/%q, want nonzero/empty", exitCode, stdout)
		}
		var payload contract.ErrorPayload
		if err := json.Unmarshal([]byte(stderr), &payload); err != nil {
			t.Fatalf("json.Unmarshal(role error) error = %v; stderr=%s", err, stderr)
		}
		if payload.Diagnostic == nil || payload.Diagnostic.Code != contract.CodeRoleUnknown {
			t.Fatalf("roles show unknown payload = %#v, want role_unknown", payload)
		}
	})
}
