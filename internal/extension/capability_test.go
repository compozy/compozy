package extensionpkg

import (
	"errors"
	"slices"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
)

func TestCapabilityCheckerCheckShouldAllowGrantedCapability(t *testing.T) {
	t.Parallel()

	checker := newTestCapabilityChecker(
		"ext",
		SourceUser,
		[]string{"sessions/list"},
		[]string{"session.read"},
	)

	if err := checker.Check("ext", "session.read"); err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
}

func TestCapabilityCheckerCheckShouldReturnCapabilityDenied(t *testing.T) {
	t.Parallel()

	checker := newTestCapabilityChecker(
		"ext",
		SourceUser,
		[]string{"sessions/list"},
		[]string{"session.read"},
	)

	err := checker.Check("ext", "session.write")
	if err == nil {
		t.Fatal("Check() error = nil, want capability denied")
	}

	var denied *ErrCapabilityDenied
	if !errors.As(err, &denied) {
		t.Fatalf("Check() error type = %T, want *ErrCapabilityDenied", err)
	}
	if denied.Code() != CapabilityDeniedCode {
		t.Fatalf("Code() = %d, want %d", denied.Code(), CapabilityDeniedCode)
	}
	if denied.Data.Method != "session.write" {
		t.Fatalf("Data.Method = %q, want %q", denied.Data.Method, "session.write")
	}
	if !slices.Equal(denied.Data.Required, []string{"session.write"}) {
		t.Fatalf("Data.Required = %v, want %v", denied.Data.Required, []string{"session.write"})
	}
	if !slices.Equal(denied.Data.Granted, []string{"session.read"}) {
		t.Fatalf("Data.Granted = %v, want %v", denied.Data.Granted, []string{"session.read"})
	}
}

func TestCapabilityCheckerCheckHostAPIShouldEnforceDualGates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		actions      []string
		security     []string
		method       string
		wantRequired []string
		wantGranted  []string
		wantErr      bool
	}{
		{
			name:     "succeeds when action and security are granted",
			actions:  []string{"sessions/list"},
			security: []string{"session.read"},
			method:   "sessions/list",
		},
		{
			name:     "allows bridge list method with matching grant",
			actions:  []string{"bridges/instances/list"},
			security: []string{"bridge.read"},
			method:   "bridges/instances/list",
		},
		{
			name:     "allows bridge read method with matching grant",
			actions:  []string{"bridges/instances/get"},
			security: []string{"bridge.read"},
			method:   "bridges/instances/get",
		},
		{
			name:    "allows sandbox list with action grant only",
			actions: []string{"sandbox/list"},
			method:  "sandbox/list",
		},
		{
			name:    "allows sandbox info with action grant only",
			actions: []string{"sandbox/info"},
			method:  "sandbox/info",
		},
		{
			name:     "allows sandbox exec with action and exec capability",
			actions:  []string{"sandbox/exec"},
			security: []string{"sandbox.exec"},
			method:   "sandbox/exec",
		},
		{
			name:         "rejects sandbox exec without exec capability",
			actions:      []string{"sandbox/exec"},
			security:     []string{"session.read"},
			method:       "sandbox/exec",
			wantRequired: []string{"sandbox.exec"},
			wantGranted:  []string{"session.read"},
			wantErr:      true,
		},
		{
			name:     "ShouldAllowBridgeStateReportWithWriteGrant",
			actions:  []string{"bridges/instances/report_state"},
			security: []string{"bridge.write"},
			method:   "bridges/instances/report_state",
		},
		{
			name:         "ShouldRejectBridgeStateReportWithoutActionGrant",
			actions:      []string{"bridges/instances/get"},
			security:     []string{"bridge.write"},
			method:       "bridges/instances/report_state",
			wantRequired: []string{"bridges/instances/report_state"},
			wantGranted:  []string{"bridges/instances/get"},
			wantErr:      true,
		},
		{
			name:         "ShouldRejectBridgeStateReportWithoutWriteGrant",
			actions:      []string{"bridges/instances/report_state"},
			security:     []string{"bridge.read"},
			method:       "bridges/instances/report_state",
			wantRequired: []string{"bridge.write"},
			wantGranted:  []string{"bridge.read"},
			wantErr:      true,
		},
		{
			name:         "fails when action grant is missing",
			actions:      nil,
			security:     []string{"session.read"},
			method:       "sessions/list",
			wantRequired: []string{"sessions/list"},
			wantGranted:  nil,
			wantErr:      true,
		},
		{
			name:         "fails when security grant is missing",
			actions:      []string{"sessions/list"},
			security:     []string{"observe.read"},
			method:       "sessions/list",
			wantRequired: []string{"session.read"},
			wantGranted:  []string{"observe.read"},
			wantErr:      true,
		},
		{
			name:     "allows logs list with logs read capability",
			actions:  []string{"logs/list"},
			security: []string{"logs.read"},
			method:   "logs/list",
		},
		{
			name:         "rejects logs list with observe read only",
			actions:      []string{"logs/list"},
			security:     []string{"observe.read"},
			method:       "logs/list",
			wantRequired: []string{"logs.read"},
			wantGranted:  []string{"observe.read"},
			wantErr:      true,
		},
		{
			name:     "automation read requires action and automation.read capability",
			actions:  []string{"automation/jobs"},
			security: []string{"automation.read"},
			method:   "automation/jobs",
		},
		{
			name:     "automation write requires action and automation.write capability",
			actions:  []string{"automation/jobs/create"},
			security: []string{"automation.write"},
			method:   "automation/jobs/create",
		},
		{
			name:         "fails for bridge write method without bridge security grant",
			actions:      []string{"bridges/messages/ingest"},
			security:     []string{"bridge.read"},
			method:       "bridges/messages/ingest",
			wantRequired: []string{"bridge.write"},
			wantGranted:  []string{"bridge.read"},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := newTestCapabilityChecker("ext", SourceUser, tt.actions, tt.security)
			err := checker.CheckHostAPI("ext", tt.method)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("CheckHostAPI() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatal("CheckHostAPI() error = nil, want capability denied")
			}

			var denied *ErrCapabilityDenied
			if !errors.As(err, &denied) {
				t.Fatalf("CheckHostAPI() error type = %T, want *ErrCapabilityDenied", err)
			}
			if denied.Data.Method != tt.method {
				t.Fatalf("Data.Method = %q, want %q", denied.Data.Method, tt.method)
			}
			if !slices.Equal(denied.Data.Required, tt.wantRequired) {
				t.Fatalf("Data.Required = %v, want %v", denied.Data.Required, tt.wantRequired)
			}
			if !slices.Equal(denied.Data.Granted, tt.wantGranted) {
				t.Fatalf("Data.Granted = %v, want %v", denied.Data.Granted, tt.wantGranted)
			}
		})
	}
}

func TestCapabilityCheckerAutomationMethodsMapToExpectedCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method     string
		capability string
	}{
		{method: "automation/jobs", capability: "automation.read"},
		{method: "automation/jobs/get", capability: "automation.read"},
		{method: "automation/jobs/create", capability: "automation.write"},
		{method: "automation/jobs/update", capability: "automation.write"},
		{method: "automation/jobs/delete", capability: "automation.write"},
		{method: "automation/jobs/trigger", capability: "automation.write"},
		{method: "automation/jobs/runs", capability: "automation.read"},
		{method: "automation/triggers", capability: "automation.read"},
		{method: "automation/triggers/get", capability: "automation.read"},
		{method: "automation/triggers/create", capability: "automation.write"},
		{method: "automation/triggers/update", capability: "automation.write"},
		{method: "automation/triggers/delete", capability: "automation.write"},
		{method: "automation/triggers/runs", capability: "automation.read"},
		{method: "automation/triggers/fire", capability: "automation.write"},
		{method: "automation/runs", capability: "automation.read"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()

			checker := newTestCapabilityChecker("ext", SourceUser, []string{tt.method}, []string{tt.capability})
			if err := checker.CheckHostAPI("ext", tt.method); err != nil {
				t.Fatalf("CheckHostAPI(%q) error = %v, want nil", tt.method, err)
			}
		})
	}
}

func TestCapabilityCheckerNetworkMethodsShouldMapToExpectedCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method     string
		capability string
	}{
		{method: "network/status", capability: "network.read"},
		{method: "network/usage", capability: "network.read"},
		{method: "network/channels", capability: "network.read"},
		{method: "network/peers", capability: "network.read"},
		{method: "network/threads", capability: "network.read"},
		{method: "network/thread/get", capability: "network.read"},
		{method: "network/thread/messages", capability: "network.read"},
		{method: "network/directs", capability: "network.read"},
		{method: "network/direct/resolve", capability: "network.write"},
		{method: "network/direct/messages", capability: "network.read"},
		{method: "network/work/get", capability: "network.read"},
		{method: "network/send", capability: "network.write"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()

			checker := newTestCapabilityChecker("ext", SourceUser, []string{tt.method}, []string{tt.capability})
			if err := checker.CheckHostAPI("ext", tt.method); err != nil {
				t.Fatalf("CheckHostAPI(%q) error = %v, want nil", tt.method, err)
			}
		})
	}
}

func TestCapabilityCheckerRegisterShouldGrantRequestedCapabilitiesForTrustedSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source ExtensionSource
	}{
		{name: "bundled", source: SourceBundled},
		{name: "user", source: SourceUser},
		{name: "workspace", source: SourceWorkspace},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := newTestCapabilityChecker(
				"ext",
				tt.source,
				[]string{"memory/store", "sessions/create"},
				[]string{"agent.pre_start", "memory.write", "permission.request", "session.write"},
			)

			for _, capability := range []string{"agent.pre_start", "memory.write", "permission.request", "session.write"} {
				if err := checker.Check("ext", capability); err != nil {
					t.Fatalf("Check(%q) error = %v, want nil", capability, err)
				}
			}
			for _, method := range []string{"memory/store", "sessions/create"} {
				if err := checker.CheckHostAPI("ext", method); err != nil {
					t.Fatalf("CheckHostAPI(%q) error = %v, want nil", method, err)
				}
			}
		})
	}
}

func TestCapabilityCheckerMarketplaceShouldDenyRestrictedCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		capability string
	}{
		{name: "permission family", capability: "permission.request"},
		{name: "session write", capability: "session.write"},
		{name: "memory write", capability: "memory.write"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := newTestCapabilityChecker("ext", SourceMarketplace, nil, []string{tt.capability})
			err := checker.Check("ext", tt.capability)
			if err == nil {
				t.Fatalf("Check(%q) error = nil, want capability denied", tt.capability)
			}
			var denied *ErrCapabilityDenied
			if !errors.As(err, &denied) {
				t.Fatalf("Check(%q) error type = %T, want *ErrCapabilityDenied", tt.capability, err)
			}
		})
	}
}

func TestCapabilityCheckerMarketplaceShouldAllowDefaultReadCapabilities(t *testing.T) {
	t.Parallel()

	checker := newTestCapabilityChecker(
		"ext",
		SourceMarketplace,
		[]string{"memory/recall", "logs/list", "sessions/list"},
		[]string{"*"},
	)

	for _, capability := range []string{"memory.read", "logs.read", "observe.read", "session.read"} {
		if err := checker.Check("ext", capability); err != nil {
			t.Fatalf("Check(%q) error = %v, want nil", capability, err)
		}
	}
	for _, method := range []string{"memory/recall", "logs/list", "sessions/list"} {
		if err := checker.CheckHostAPI("ext", method); err != nil {
			t.Fatalf("CheckHostAPI(%q) error = %v, want nil", method, err)
		}
	}
}

func TestCapabilityCheckerRegisterShouldApplyMarketplaceTierCeiling(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	checker.Register("ext", SourceMarketplace, &Manifest{
		Actions: ActionsConfig{
			Requires: []string{
				"logs/list",
				"memory/recall",
				"memory/store",
				"sessions/create",
				"sessions/list",
				"skills/list",
			},
		},
		Security: SecurityConfig{
			Capabilities: []string{"*"},
		},
	})

	grant := checker.grants["ext"]
	if !slices.Equal(grant.actions, []string{"logs/list", "memory/recall", "sessions/list", "skills/list"}) {
		t.Fatalf(
			"grant.actions = %v, want %v",
			grant.actions,
			[]string{"logs/list", "memory/recall", "sessions/list", "skills/list"},
		)
	}
	if !slices.Equal(
		grant.security,
		[]string{"logs.read", "memory.read", "observe.read", "session.read", "skills.read", "tool.read"},
	) {
		t.Fatalf(
			"grant.security = %v, want %v",
			grant.security,
			[]string{"logs.read", "memory.read", "observe.read", "session.read", "skills.read", "tool.read"},
		)
	}
}

func TestCapabilityCheckerCheckShouldHonorGlobalWildcardGrant(t *testing.T) {
	t.Parallel()

	checker := newTestCapabilityChecker("ext", SourceUser, nil, []string{"*"})
	for _, capability := range []string{"agent.pre_start", "permission.request", "session.write"} {
		if err := checker.Check("ext", capability); err != nil {
			t.Fatalf("Check(%q) error = %v, want nil", capability, err)
		}
	}
}

func TestCapabilityCheckerCheckShouldHonorFamilyWildcardGrant(t *testing.T) {
	t.Parallel()

	checker := newTestCapabilityChecker("ext", SourceUser, nil, []string{"session.*"})
	for _, capability := range []string{"session.read", "session.write"} {
		if err := checker.Check("ext", capability); err != nil {
			t.Fatalf("Check(%q) error = %v, want nil", capability, err)
		}
	}

	if err := checker.Check("ext", "memory.read"); err == nil {
		t.Fatal("Check(memory.read) error = nil, want capability denied")
	}
}

func TestCapabilityCheckerResolveShouldApplyOperatorResourcePolicy(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	checker.SetResourcePolicy(aghconfig.ExtensionsResourcesConfig{
		AllowedKinds: []resources.ResourceKind{resources.ResourceKind("tool")},
		MaxScope:     resources.ResourceScopeKindWorkspace,
	})

	grant, err := checker.Resolve(SourceUser, &Manifest{
		Resources: ResourcesConfig{
			Publish: ResourceGrantRequest{
				Families: []string{"tools", "mcp_servers"},
				MaxScope: resources.ResourceScopeKindGlobal,
			},
		},
	}, resources.ResourceScopeKindGlobal)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !slices.Equal(grant.ResourceKinds, []resources.ResourceKind{resources.ResourceKind("tool")}) {
		t.Fatalf("Resolve().ResourceKinds = %#v, want [tool]", grant.ResourceKinds)
	}
	if !slices.Equal(grant.ResourceScopes, []resources.ResourceScopeKind{resources.ResourceScopeKindWorkspace}) {
		t.Fatalf("Resolve().ResourceScopes = %#v, want [workspace]", grant.ResourceScopes)
	}
}

func TestCapabilityCheckerResolveShouldApplySourceTierScopeCeiling(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	grant, err := checker.Resolve(SourceWorkspace, &Manifest{
		Resources: ResourcesConfig{
			Publish: ResourceGrantRequest{
				Families: []string{"tools"},
				MaxScope: resources.ResourceScopeKindGlobal,
			},
		},
	}, resources.ResourceScopeKindGlobal)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !slices.Equal(grant.ResourceScopes, []resources.ResourceScopeKind{resources.ResourceScopeKindWorkspace}) {
		t.Fatalf("Resolve().ResourceScopes = %#v, want [workspace]", grant.ResourceScopes)
	}
}

func TestCapabilityCheckerResolveShouldApplySessionModeScopeNarrowing(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	grant, err := checker.Resolve(SourceUser, &Manifest{
		Resources: ResourcesConfig{
			Publish: ResourceGrantRequest{
				Families: []string{"tools"},
				MaxScope: resources.ResourceScopeKindGlobal,
			},
		},
	}, resources.ResourceScopeKindWorkspace)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !slices.Equal(grant.ResourceScopes, []resources.ResourceScopeKind{resources.ResourceScopeKindWorkspace}) {
		t.Fatalf("Resolve().ResourceScopes = %#v, want [workspace]", grant.ResourceScopes)
	}
}

func TestCapabilityCheckerRegisterForSessionStoresGrantSnapshot(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	grant, err := checker.RegisterForSession("ext", SourceUser, &Manifest{
		Resources: ResourcesConfig{
			Publish: ResourceGrantRequest{
				Families: []string{"tools"},
				MaxScope: resources.ResourceScopeKindGlobal,
			},
		},
	}, resources.ResourceScopeKindWorkspace)
	if err != nil {
		t.Fatalf("RegisterForSession() error = %v", err)
	}
	if !slices.Equal(grant.ResourceScopes, []resources.ResourceScopeKind{resources.ResourceScopeKindWorkspace}) {
		t.Fatalf("RegisterForSession().ResourceScopes = %#v, want [workspace]", grant.ResourceScopes)
	}

	stored := checker.Grant("ext")
	if !slices.Equal(stored.ResourceKinds, []resources.ResourceKind{resources.ResourceKind("tool")}) {
		t.Fatalf("Grant().ResourceKinds = %#v, want [tool]", stored.ResourceKinds)
	}

	grant.ResourceKinds[0] = resources.ResourceKind("mutated")
	if got := checker.Grant("ext").ResourceKinds[0]; got != resources.ResourceKind("tool") {
		t.Fatalf("Grant() leaked caller mutation, got %q", got)
	}
}

func TestCapabilityCheckerRegisterForSessionRejectsInvalidManifestResourceRequest(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	_, err := checker.RegisterForSession("ext", SourceUser, &Manifest{
		Resources: ResourcesConfig{
			Publish: ResourceGrantRequest{
				Families: []string{"bridge_instances"},
				MaxScope: resources.ResourceScopeKindGlobal,
			},
		},
	}, resources.ResourceScopeKindGlobal)
	if err == nil {
		t.Fatal("RegisterForSession() error = nil, want invalid manifest request")
	}
}

func TestCapabilityCheckerNilResolveReturnsEmptyGrant(t *testing.T) {
	t.Parallel()

	var checker *CapabilityChecker
	grant, err := checker.Resolve(SourceUser, nil, resources.ResourceScopeKindGlobal)
	if err != nil {
		t.Fatalf("Resolve(nil) error = %v, want nil", err)
	}
	if len(grant.Actions) != 0 ||
		len(grant.Security) != 0 ||
		len(grant.ResourceKinds) != 0 ||
		len(grant.ResourceScopes) != 0 {
		t.Fatalf("Resolve(nil) = %#v, want zero value", grant)
	}
}

func TestSourceTierResourceHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		source    ExtensionSource
		wantScope resources.ResourceScopeKind
	}{
		{source: SourceBundled, wantScope: resources.ResourceScopeKindGlobal},
		{source: SourceUser, wantScope: resources.ResourceScopeKindGlobal},
		{source: SourceWorkspace, wantScope: resources.ResourceScopeKindWorkspace},
		{source: SourceMarketplace, wantScope: resources.ResourceScopeKindWorkspace},
		{source: ExtensionSource(99), wantScope: ""},
	}

	for _, tt := range tests {
		if got := sourceTierMaxScope(tt.source); got != tt.wantScope {
			t.Fatalf("sourceTierMaxScope(%v) = %q, want %q", tt.source, got, tt.wantScope)
		}
	}
	if !slices.Equal(scopesThrough(resources.ResourceScopeKindGlobal), []resources.ResourceScopeKind{
		resources.ResourceScopeKindGlobal,
		resources.ResourceScopeKindWorkspace,
	}) {
		t.Fatalf("scopesThrough(global) = %#v, want global+workspace", scopesThrough(resources.ResourceScopeKindGlobal))
	}
	if !slices.Equal(scopesThrough(resources.ResourceScopeKindWorkspace), []resources.ResourceScopeKind{
		resources.ResourceScopeKindWorkspace,
	}) {
		t.Fatalf(
			"scopesThrough(workspace) = %#v, want [workspace]",
			scopesThrough(resources.ResourceScopeKindWorkspace),
		)
	}
	if got, want := scopeRank(resources.ResourceScopeKindWorkspace), 0; got != want {
		t.Fatalf("scopeRank(workspace) = %d, want %d", got, want)
	}
	if got, want := scopeRank(resources.ResourceScopeKindGlobal), 1; got != want {
		t.Fatalf("scopeRank(global) = %d, want %d", got, want)
	}
	if got, want := scopeRank(resources.ResourceScopeKind("")), 2; got != want {
		t.Fatalf("scopeRank(unknown) = %d, want %d", got, want)
	}
	if got := scopesThrough(resources.ResourceScopeKind("invalid")); got != nil {
		t.Fatalf("scopesThrough(invalid) = %#v, want nil", got)
	}
}

func TestCapabilityHelperPoliciesAndCeilings(t *testing.T) {
	t.Parallel()

	if !ceilingAllowsRequestedGrant([]string{"network.*"}, "network.http") {
		t.Fatalf("ceilingAllowsRequestedGrant() = false, want true for wildcard superset")
	}
	if ceilingAllowsRequestedGrant([]string{"network.http"}, "network.*") {
		t.Fatalf("ceilingAllowsRequestedGrant() = true, want false when request exceeds ceiling")
	}

	marketplace := sourcePolicy(SourceMarketplace)
	if marketplace.allowAllActions || marketplace.allowAllSecurity {
		t.Fatalf("marketplace policy = %#v, want narrowed actions and security", marketplace)
	}
	if marketplace.maxResourceScope != resources.ResourceScopeKindWorkspace {
		t.Fatalf("marketplace maxResourceScope = %q, want workspace", marketplace.maxResourceScope)
	}
	if len(marketplace.allowedActions) == 0 || len(marketplace.allowedSecurity) == 0 {
		t.Fatalf("marketplace policy = %#v, want populated ceilings", marketplace)
	}

	bundled := sourcePolicy(SourceBundled)
	if !bundled.allowAllActions || !bundled.allowAllSecurity {
		t.Fatalf("bundled policy = %#v, want full action and security grants", bundled)
	}
	if bundled.maxResourceScope != resources.ResourceScopeKindGlobal {
		t.Fatalf("bundled maxResourceScope = %q, want global", bundled.maxResourceScope)
	}

	if got, err := narrowScopeCeiling("", "", "", ""); err != nil || got != "" {
		t.Fatalf("narrowScopeCeiling(empty) = (%q, %v), want empty nil", got, err)
	}
	if _, err := narrowScopeCeiling(resources.ResourceScopeKind("invalid"), "", "", ""); err == nil {
		t.Fatalf("narrowScopeCeiling(invalid) error = nil, want validation error")
	}
}

func newTestCapabilityChecker(
	extName string,
	source ExtensionSource,
	actions []string,
	security []string,
) *CapabilityChecker {
	checker := &CapabilityChecker{}
	checker.Register(extName, source, &Manifest{
		Actions: ActionsConfig{
			Requires: actions,
		},
		Security: SecurityConfig{
			Capabilities: security,
		},
	})
	return checker
}
