package daemon

import (
	"errors"
	"reflect"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
)

func TestRoleStatusProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should return the closed roster sorted with truthful provenance", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		statuses, err := newRoleResolver(&cfg, nil, nil).RoleStatuses(t.Context(), "")
		if err != nil {
			t.Fatalf("RoleStatuses() error = %v", err)
		}
		roles := make([]string, 0, len(statuses))
		for _, status := range statuses {
			roles = append(roles, status.Role)
			assertRoleStatusProvenance(t, status)
		}
		want := []string{
			"auto_title",
			"checkpoint_summary",
			"coordinator",
			"dream",
			"memory_controller",
			"memory_extractor",
		}
		if !reflect.DeepEqual(roles, want) {
			t.Fatalf("RoleStatuses() roles = %#v, want %#v", roles, want)
		}
		if statuses[0].ResolutionMode != contract.RoleResolutionModeInherit || statuses[0].Agent != nil {
			t.Fatalf("auto_title status = %#v, want invocation-time inheritance", statuses[0])
		}
	})

	t.Run("Should report a missing catalog agent without failing projection", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		cfg.Roles.Dream.Agent = "missing-curator"
		status, err := newRoleResolver(&cfg, nil, roleAgentResolverStub{}).RoleStatus(
			t.Context(),
			"",
			string(aghconfig.RoleDream),
		)
		if err != nil {
			t.Fatalf("RoleStatus(dream) error = %v", err)
		}
		if len(status.Diagnostics) != 1 || status.Diagnostics[0].Code != contract.CodeRoleAgentNotFound ||
			status.Diagnostics[0].Agent != "missing-curator" {
			t.Fatalf("RoleStatus(dream).Diagnostics = %#v", status.Diagnostics)
		}
		if status.Agent == nil || *status.Agent != "missing-curator" ||
			status.ResolutionMode != contract.RoleResolutionModeCatalog {
			t.Fatalf("RoleStatus(dream) = %#v, want missing catalog projection", status)
		}
	})

	t.Run("Should preserve global and workspace field provenance", func(t *testing.T) {
		t.Parallel()

		global := roleResolverConfig()
		global.Roles.Dream.Model = "global-model"
		global.RoleSources[aghconfig.RoleDream][aghconfig.RoleFieldModel] = aghconfig.RoleFieldSourceGlobal
		resolver := newRoleResolver(&global, nil, nil)
		globalStatus, err := resolver.RoleStatus(t.Context(), "", string(aghconfig.RoleDream))
		if err != nil {
			t.Fatalf("RoleStatus(global dream) error = %v", err)
		}
		if got := globalStatus.Provenance[aghconfig.RoleFieldModel]; got != aghconfig.RoleFieldSourceGlobal {
			t.Fatalf("RoleStatus(global dream) model source = %q, want global", got)
		}

		workspace := global
		workspace.RoleSources = aghconfig.CloneRoleFieldSources(global.RoleSources)
		workspace.Roles.Dream.Model = "workspace-model"
		workspace.RoleSources[aghconfig.RoleDream][aghconfig.RoleFieldModel] = aghconfig.RoleFieldSourceWorkspace
		resolver = newRoleResolver(&global, roleWorkspaceResolverStub{configs: map[string]aghconfig.Config{
			"ws-role-status": workspace,
		}}, nil)
		workspaceStatus, err := resolver.RoleStatus(
			t.Context(),
			"ws-role-status",
			string(aghconfig.RoleDream),
		)
		if err != nil {
			t.Fatalf("RoleStatus(workspace dream) error = %v", err)
		}
		if got := workspaceStatus.Provenance[aghconfig.RoleFieldModel]; got != aghconfig.RoleFieldSourceWorkspace {
			t.Fatalf("RoleStatus(workspace dream) model source = %q, want workspace", got)
		}
	})

	t.Run("Should return the stable unknown role code", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		_, err := newRoleResolver(&cfg, nil, nil).RoleStatus(t.Context(), "", "judge")
		var resolutionErr *RoleResolutionError
		if !errors.As(err, &resolutionErr) || resolutionErr.DiagnosticCode() != contract.CodeRoleUnknown {
			t.Fatalf("RoleStatus(judge) error = %v, want role_unknown", err)
		}
	})
}

func assertRoleStatusProvenance(t *testing.T, status contract.RoleStatus) {
	t.Helper()
	for _, field := range []struct {
		name    string
		present bool
	}{
		{name: aghconfig.RoleFieldEnabled, present: true},
		{name: aghconfig.RoleFieldFallbacks, present: true},
		{name: daemonAgentField, present: status.Agent != nil},
		{name: aghconfig.RoleFieldProvider, present: status.Provider != nil},
		{name: aghconfig.RoleFieldModel, present: status.Model != nil},
		{name: aghconfig.RoleFieldReasoning, present: status.ReasoningEffort != nil},
		{name: roleFieldTimeout, present: status.Timeout != nil},
	} {
		source, exists := status.Provenance[field.name]
		if exists != field.present {
			t.Fatalf(
				"RoleStatus(%s).Provenance[%s] exists = %t, want %t: %#v",
				status.Role,
				field.name,
				exists,
				field.present,
				status.Provenance,
			)
		}
		if exists && source != aghconfig.RoleFieldSourceDefault {
			t.Fatalf(
				"RoleStatus(%s).Provenance[%s] = %q, want %q: %#v",
				status.Role,
				field.name,
				source,
				aghconfig.RoleFieldSourceDefault,
				status.Provenance,
			)
		}
	}
}
