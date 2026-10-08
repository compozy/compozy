package daemon

import (
	"errors"
	"reflect"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestRoleStatusProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve workspace role overrides and live opt-in changes in role status", func(t *testing.T) {
		t.Parallel()
		cfg := roleResolverConfig()
		cfg.Roles.AutoTitle.Enabled = false
		scoped := cfg
		scoped.Roles.AutoTitle.Enabled = true
		resolver := newRoleResolver(
			&cfg,
			roleWorkspaceResolverStub{configs: map[string]compozyconfig.Config{"ws-title": scoped}},
			nil,
		)
		for _, workspace := range []string{"", "ws-title"} {
			role, err := resolver.RoleStatus(t.Context(), workspace, string(compozyconfig.RoleAutoTitle))
			if err != nil || role.Enabled != (workspace != "") {
				t.Fatalf("workspace=%q role=%#v err=%v", workspace, role, err)
			}
		}
		cfg.Roles.AutoTitle.Enabled = true
		role, err := resolver.RoleStatus(t.Context(), "", string(compozyconfig.RoleAutoTitle))
		if err != nil || !role.Enabled {
			t.Fatalf("live role=%#v err=%v", role, err)
		}
	})
	t.Run("Should preserve the profile role context in role status", func(t *testing.T) {
		t.Parallel()
		cfg := roleResolverConfig()
		scoped := loopActionBinderWorkspace(t, nil)
		scoped.ProfileID = "profile-engineering"
		scoped.Config.Roles.AutoTitle.Enabled = true
		resolver := newRoleResolver(&cfg, &loopPolicyProfileWorkspaceResolver{scoped: scoped}, nil)
		resolver.profileNames = loopProfileNameResolverStub{"profile-engineering": "engineering"}
		ctx := withRoleInvocationCorrelation(t.Context(), roleInvocationCorrelation{ProfileID: "profile-engineering"})
		role, err := resolver.RoleStatus(ctx, "ws-loop", string(compozyconfig.RoleAutoTitle))
		if err != nil || !role.Enabled {
			t.Fatalf("profile role=%#v err=%v", role, err)
		}
		ctx = withRoleInvocationCorrelation(ctx, roleInvocationCorrelation{ProfileID: "profile-missing"})
		if _, err := resolver.RoleStatus(ctx, "ws-loop", string(compozyconfig.RoleAutoTitle)); err == nil {
			t.Fatal("missing profile must remain an error")
		}
	})

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
			"coordinator",
		}
		if !reflect.DeepEqual(roles, want) {
			t.Fatalf("RoleStatuses() roles = %#v, want %#v", roles, want)
		}
		if statuses[0].ResolutionMode != contract.RoleResolutionModeInherit || statuses[0].Agent != nil {
			t.Fatalf("auto_title status = %#v, want invocation-time inheritance", statuses[0])
		}
	})

	t.Run("Should honor workspace auto-title opt-out in role status", func(t *testing.T) {
		t.Parallel()
		global := roleResolverConfig()
		global.Roles.AutoTitle.Enabled = true
		workspace := global
		disabled := workspace
		disabled.Roles.AutoTitle.Enabled = false
		disabled.RoleSources = compozyconfig.CloneRoleFieldSources(global.RoleSources)
		disabled.RoleSources[compozyconfig.RoleAutoTitle][compozyconfig.RoleFieldEnabled] = compozyconfig.RoleFieldSourceWorkspace
		resolver := newRoleResolver(&global, roleWorkspaceResolverStub{configs: map[string]compozyconfig.Config{
			"ws-disabled": disabled,
			"ws-overlay":  workspace,
		}}, nil)
		for _, target := range []string{"ws-disabled", "ws-overlay", ""} {
			status, err := resolver.RoleStatus(t.Context(), target, string(compozyconfig.RoleAutoTitle))
			want := target != "ws-disabled"
			if err != nil || status.Enabled != want {
				t.Fatalf("workspace=%q status=%#v error=%v, want enabled=%t", target, status, err, want)
			}
			if target == "ws-disabled" &&
				status.Provenance[compozyconfig.RoleFieldEnabled] != compozyconfig.RoleFieldSourceWorkspace {
				t.Fatalf("workspace role switch provenance=%#v", status.Provenance)
			}
		}
	})
	for _, tc := range []struct {
		name    string
		enabled bool
	}{
		{name: "Should honor profile auto-title opt-in in role status", enabled: true},
		{name: "Should honor profile auto-title opt-out in role status"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			global := roleResolverConfig()
			scoped := loopActionBinderWorkspace(t, nil)
			scoped.ProfileID = "profile-engineering"
			scoped.Config.Roles.AutoTitle.Enabled = tc.enabled
			resolver := newRoleResolver(&global, &loopPolicyProfileWorkspaceResolver{scoped: scoped}, nil)
			resolver.profileNames = loopProfileNameResolverStub{"profile-engineering": "engineering"}
			ctx := withRoleInvocationCorrelation(
				t.Context(),
				roleInvocationCorrelation{ProfileID: "profile-engineering"},
			)
			status, err := resolver.RoleStatus(ctx, "ws-loop", string(compozyconfig.RoleAutoTitle))
			if err != nil || status.Enabled != tc.enabled {
				t.Fatalf("profile status=%#v error=%v, want enabled=%t", status, err, tc.enabled)
			}
		})
	}

	t.Run("Should report a missing catalog agent without failing projection", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		cfg.Roles.AutoTitle.Agent = "missing-curator"
		status, err := newRoleResolver(&cfg, nil, roleAgentResolverStub{}).RoleStatus(
			t.Context(),
			"",
			string(compozyconfig.RoleAutoTitle),
		)
		if err != nil {
			t.Fatalf("RoleStatus(title) error = %v", err)
		}
		if len(status.Diagnostics) != 1 || status.Diagnostics[0].Code != contract.CodeRoleAgentNotFound ||
			status.Diagnostics[0].Agent != "missing-curator" {
			t.Fatalf("RoleStatus(title).Diagnostics = %#v", status.Diagnostics)
		}
		if status.Agent == nil || *status.Agent != "missing-curator" ||
			status.ResolutionMode != contract.RoleResolutionModeCatalog {
			t.Fatalf("RoleStatus(title) = %#v, want missing catalog projection", status)
		}
	})

	t.Run("Should project route commands with their fingerprint", func(t *testing.T) { // UT-018, IT-001
		t.Parallel()

		const command = "CLAUDE_CONFIG_DIR=/Users/ada/.claude-work claude --acp"
		cfg := roleResolverConfig()
		cfg.Roles.AutoTitle.FallbackChain = []compozyconfig.RoleFallback{
			{Provider: "claude", Model: "haiku-4-5", Command: command},
			{Provider: "codex", Model: "gpt-5.6-terra", ReasoningEffort: "high"},
		}
		status, err := newRoleResolver(&cfg, nil, roleAgentResolverStub{}).RoleStatus(
			t.Context(),
			"",
			string(compozyconfig.RoleAutoTitle),
		)
		if err != nil {
			t.Fatalf("RoleStatus(auto_title) error = %v", err)
		}
		chain := status.FallbackChain
		if len(chain) != 2 || chain[0].Command != command ||
			chain[0].CommandFingerprint != compozyconfig.CommandFingerprint(command) ||
			chain[1].Command != "" || chain[1].CommandFingerprint != "" {
			t.Fatalf("RoleStatus(auto_title).FallbackChain = %#v, want command projected with fingerprint", chain)
		}
	})

	t.Run("Should preserve global and workspace field provenance", func(t *testing.T) {
		t.Parallel()

		global := roleResolverConfig()
		global.Roles.AutoTitle.Model = "global-model"
		global.RoleSources[compozyconfig.RoleAutoTitle][compozyconfig.RoleFieldModel] = compozyconfig.RoleFieldSourceGlobal
		resolver := newRoleResolver(&global, nil, nil)
		globalStatus, err := resolver.RoleStatus(t.Context(), "", string(compozyconfig.RoleAutoTitle))
		if err != nil {
			t.Fatalf("RoleStatus(global title) error = %v", err)
		}
		if got := globalStatus.Provenance[compozyconfig.RoleFieldModel]; got != compozyconfig.RoleFieldSourceGlobal {
			t.Fatalf("RoleStatus(global title) model source = %q, want global", got)
		}

		workspace := global
		workspace.RoleSources = compozyconfig.CloneRoleFieldSources(global.RoleSources)
		workspace.Roles.AutoTitle.Model = "workspace-model"
		workspace.RoleSources[compozyconfig.RoleAutoTitle][compozyconfig.RoleFieldModel] = compozyconfig.RoleFieldSourceWorkspace
		resolver = newRoleResolver(&global, roleWorkspaceResolverStub{configs: map[string]compozyconfig.Config{
			"ws-role-status": workspace,
		}}, nil)
		workspaceStatus, err := resolver.RoleStatus(
			t.Context(),
			"ws-role-status",
			string(compozyconfig.RoleAutoTitle),
		)
		if err != nil {
			t.Fatalf("RoleStatus(workspace title) error = %v", err)
		}
		if got := workspaceStatus.Provenance[compozyconfig.RoleFieldModel]; got != compozyconfig.RoleFieldSourceWorkspace {
			t.Fatalf("RoleStatus(workspace title) model source = %q, want workspace", got)
		}
	})

	t.Run("Should return the stable unknown role code", func(t *testing.T) {
		t.Parallel()

		cfg := roleResolverConfig()
		_, err := newRoleResolver(&cfg, nil, nil).RoleStatus(t.Context(), "", "judge")
		resolutionErr, resolutionErrMatched := errors.AsType[*RoleResolutionError](err)
		if !resolutionErrMatched || resolutionErr.DiagnosticCode() != contract.CodeRoleUnknown {
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
		{name: compozyconfig.RoleFieldEnabled, present: true},
		{name: compozyconfig.RoleFieldFallbacks, present: true},
		{name: daemonAgentField, present: status.Agent != nil},
		{name: compozyconfig.RoleFieldProvider, present: status.Provider != nil},
		{name: compozyconfig.RoleFieldModel, present: status.Model != nil},
		{name: compozyconfig.RoleFieldReasoning, present: status.ReasoningEffort != nil},
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
		if exists && source != compozyconfig.RoleFieldSourceDefault {
			t.Fatalf(
				"RoleStatus(%s).Provenance[%s] = %q, want %q: %#v",
				status.Role,
				field.name,
				source,
				compozyconfig.RoleFieldSourceDefault,
				status.Provenance,
			)
		}
	}
}
