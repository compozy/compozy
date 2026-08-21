package extensionpkg

import (
	"errors"
	"slices"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/resources"
)

func TestDeriveConsentAreas(t *testing.T) {
	t.Parallel()

	t.Run("Should derive unique consent areas from permissions", func(t *testing.T) {
		t.Parallel()

		got, err := DeriveConsentAreas([]string{"sessions/list", "memory/store", "sessions/list"})
		if err != nil {
			t.Fatalf("DeriveConsentAreas() error = %v", err)
		}
		want := []ConsentArea{
			{Area: "memory", Access: "write"},
			{Area: "sessions", Access: "read"},
		}
		if !slices.Equal(got, want) {
			t.Fatalf("DeriveConsentAreas() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject an unknown permission", func(t *testing.T) {
		t.Parallel()

		_, err := DeriveConsentAreas([]string{"nope/x"})
		if err == nil {
			t.Fatal("DeriveConsentAreas() error = nil, want membership error")
		}
		if !strings.Contains(err.Error(), "nope/x") || !strings.Contains(err.Error(), "unknown Host API permission") {
			t.Fatalf("DeriveConsentAreas() error = %v, want unknown permission", err)
		}
	})

	t.Run("Should map every generated Host API permission to a consent area", func(t *testing.T) {
		t.Parallel()

		for _, contract := range extensioncontract.PermissionContracts() {
			if contract.Area == "" || contract.Access == "" || contract.Capability == "" {
				t.Fatalf("permission contract %q = %#v, want non-empty consent area", contract.Method, contract)
			}
		}
	})
}

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

	denied, deniedMatched := errors.AsType[*ErrCapabilityDenied](err)
	if !deniedMatched {
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

func TestCapabilityCheckerCheckHostAPIShouldEnforcePermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		permissions  []string
		method       string
		wantRequired []string
		wantGranted  []string
		wantErr      bool
	}{
		{
			name:        "Should allow a declared read permission",
			permissions: []string{"sessions/list"},
			method:      "sessions/list",
		},
		{
			name:        "Should allow a declared write permission",
			permissions: []string{"bridges/instances/report_state"},
			method:      "bridges/instances/report_state",
		},
		{
			name:        "Should allow a declared execution permission",
			permissions: []string{"sandbox/exec"},
			method:      "sandbox/exec",
		},
		{
			name:         "Should reject an undeclared write permission",
			permissions:  []string{"bridges/instances/get"},
			method:       "bridges/instances/report_state",
			wantRequired: []string{"bridges/instances/report_state"},
			wantGranted:  []string{"bridges/instances/get"},
			wantErr:      true,
		},
		{
			name:         "Should reject an undeclared read permission",
			method:       "sessions/list",
			wantRequired: []string{"sessions/list"},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := newTestCapabilityChecker("ext", SourceUser, tt.permissions)
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

			denied, deniedMatched := errors.AsType[*ErrCapabilityDenied](err)
			if !deniedMatched {
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
			)

			for _, capability := range []string{"memory.write", "session.write"} {
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
		method     string
		capability string
	}{
		{name: "session write", method: "sessions/create", capability: "session.write"},
		{name: "memory write", method: "memory/store", capability: "memory.write"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := newTestCapabilityChecker("ext", SourceMarketplace, []string{tt.method})
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
		[]string{"memory/recall", "logs/list", "observe/health", "sessions/list", "skills/list"},
	)

	for _, capability := range []string{"memory.read", "logs.read", "observe.read", "session.read"} {
		if err := checker.Check("ext", capability); err != nil {
			t.Fatalf("Check(%q) error = %v, want nil", capability, err)
		}
	}
	for _, method := range []string{"memory/recall", "logs/list", "observe/health", "sessions/list", "skills/list"} {
		if err := checker.CheckHostAPI("ext", method); err != nil {
			t.Fatalf("CheckHostAPI(%q) error = %v, want nil", method, err)
		}
	}
}

func TestCapabilityCheckerRegisterShouldApplyMarketplaceTierCeiling(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	checker.Register("ext", SourceMarketplace, &Manifest{
		Permissions: PermissionsConfig{
			Requires: []string{
				"logs/list",
				"memory/recall",
				"memory/store",
				"sessions/create",
				"sessions/list",
				"skills/list",
			},
		},
	})

	grant := checker.grants["ext"]
	if !slices.Equal(grant.permissions, []string{"logs/list", "memory/recall", "sessions/list", "skills/list"}) {
		t.Fatalf(
			"grant.permissions = %v, want %v",
			grant.permissions,
			[]string{"logs/list", "memory/recall", "sessions/list", "skills/list"},
		)
	}
	if !slices.Equal(
		grant.security,
		[]string{"logs.read", "memory.read", "session.read", "skills.read"},
	) {
		t.Fatalf(
			"grant.security = %v, want %v",
			grant.security,
			[]string{"logs.read", "memory.read", "session.read", "skills.read"},
		)
	}
}

func TestCapabilityCheckerResolveShouldApplyOperatorResourcePolicy(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	checker.SetResourcePolicy(compozyconfig.ExtensionsResourcesConfig{
		AllowedKinds: []resources.ResourceKind{resources.ResourceKind("tool")},
		MaxScope:     resources.ResourceScopeKindWorkspace,
	})

	grant, err := checker.Resolve(SourceUser, &Manifest{
		Resources: ResourcesConfig{
			Publish: ResourceGrantRequest{
				Families: []string{"tools", "mcp_servers"},
				MaxScope: resources.ResourceScopeKindUser,
			},
		},
	}, resources.ResourceScopeKindUser)
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
				MaxScope: resources.ResourceScopeKindUser,
			},
		},
	}, resources.ResourceScopeKindUser)
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
				MaxScope: resources.ResourceScopeKindUser,
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
				MaxScope: resources.ResourceScopeKindUser,
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
				MaxScope: resources.ResourceScopeKindUser,
			},
		},
	}, resources.ResourceScopeKindUser)
	if err == nil {
		t.Fatal("RegisterForSession() error = nil, want invalid manifest request")
	}
}

func TestCapabilityCheckerNilResolveReturnsEmptyGrant(t *testing.T) {
	t.Parallel()

	var checker *CapabilityChecker
	grant, err := checker.Resolve(SourceUser, nil, resources.ResourceScopeKindUser)
	if err != nil {
		t.Fatalf("Resolve(nil) error = %v, want nil", err)
	}
	if len(grant.Permissions) != 0 ||
		len(grant.Security) != 0 ||
		len(grant.ResourceKinds) != 0 ||
		len(grant.ResourceScopes) != 0 {
		t.Fatalf("Resolve(nil) = %#v, want zero value", grant)
	}
}

func TestSourceTierResourceHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		source    ExtensionSource
		wantScope resources.ResourceScopeKind
	}{
		{
			name:      "Should grant bundled sources a global ceiling",
			source:    SourceBundled,
			wantScope: resources.ResourceScopeKindUser,
		},
		{
			name:      "Should grant user sources a global ceiling",
			source:    SourceUser,
			wantScope: resources.ResourceScopeKindUser,
		},
		{
			name:      "Should grant workspace sources a workspace ceiling",
			source:    SourceWorkspace,
			wantScope: resources.ResourceScopeKindWorkspace,
		},
		{
			name:      "Should grant marketplace sources a global read ceiling",
			source:    SourceMarketplace,
			wantScope: resources.ResourceScopeKindUser,
		},
		{name: "Should reject an unknown source ceiling", source: ExtensionSource(99), wantScope: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := sourceTierMaxScope(tt.source); got != tt.wantScope {
				t.Fatalf("sourceTierMaxScope(%v) = %q, want %q", tt.source, got, tt.wantScope)
			}
		})
	}

	t.Run("Should project scopes through each valid ceiling", func(t *testing.T) {
		t.Parallel()

		if !slices.Equal(scopesThrough(resources.ResourceScopeKindUser), []resources.ResourceScopeKind{
			resources.ResourceScopeKindUser,
			resources.ResourceScopeKindWorkspace,
		}) {
			t.Fatalf(
				"scopesThrough(global) = %#v, want global+workspace",
				scopesThrough(resources.ResourceScopeKindUser),
			)
		}
		if !slices.Equal(scopesThrough(resources.ResourceScopeKindWorkspace), []resources.ResourceScopeKind{
			resources.ResourceScopeKindWorkspace,
		}) {
			t.Fatalf(
				"scopesThrough(workspace) = %#v, want [workspace]",
				scopesThrough(resources.ResourceScopeKindWorkspace),
			)
		}
		if got := scopesThrough(resources.ResourceScopeKind("invalid")); got != nil {
			t.Fatalf("scopesThrough(invalid) = %#v, want nil", got)
		}
	})

	t.Run("Should rank workspace before global and unknown scopes", func(t *testing.T) {
		t.Parallel()

		if got, want := scopeRank(resources.ResourceScopeKindWorkspace), 0; got != want {
			t.Fatalf("scopeRank(workspace) = %d, want %d", got, want)
		}
		if got, want := scopeRank(resources.ResourceScopeKindUser), 1; got != want {
			t.Fatalf("scopeRank(global) = %d, want %d", got, want)
		}
		if got, want := scopeRank(resources.ResourceScopeKind("")), 2; got != want {
			t.Fatalf("scopeRank(unknown) = %d, want %d", got, want)
		}
	})
}

func TestCapabilityHelperPoliciesAndCeilings(t *testing.T) {
	t.Parallel()

	t.Run("Should enforce wildcard grant ceilings", func(t *testing.T) {
		t.Parallel()

		if !capabilityGranted([]string{"network.*"}, "network.http") {
			t.Fatalf("capabilityGranted() = false, want true for wildcard superset")
		}
		if capabilityGranted([]string{"network.http"}, "network.*") {
			t.Fatalf("capabilityGranted() = true, want false when request exceeds ceiling")
		}
	})

	t.Run("Should restrict marketplace permissions while preserving the global read ceiling", func(t *testing.T) {
		t.Parallel()

		marketplace := sourcePolicy(SourceMarketplace)
		if marketplace.allowAllPermissions {
			t.Fatalf("marketplace policy = %#v, want narrowed permissions", marketplace)
		}
		if marketplace.maxResourceScope != resources.ResourceScopeKindUser {
			t.Fatalf("marketplace maxResourceScope = %q, want global", marketplace.maxResourceScope)
		}
		if len(marketplace.allowedConsent) == 0 {
			t.Fatalf("marketplace policy = %#v, want populated ceilings", marketplace)
		}
	})

	t.Run("Should grant bundled extensions their full global ceiling", func(t *testing.T) {
		t.Parallel()

		bundled := sourcePolicy(SourceBundled)
		if !bundled.allowAllPermissions {
			t.Fatalf("bundled policy = %#v, want full permission grants", bundled)
		}
		if bundled.maxResourceScope != resources.ResourceScopeKindUser {
			t.Fatalf("bundled maxResourceScope = %q, want global", bundled.maxResourceScope)
		}
	})

	t.Run("Should validate narrowed scope ceilings", func(t *testing.T) {
		t.Parallel()

		if got, err := narrowScopeCeiling("", "", "", ""); err != nil || got != "" {
			t.Fatalf("narrowScopeCeiling(empty) = (%q, %v), want empty nil", got, err)
		}
		if _, err := narrowScopeCeiling(resources.ResourceScopeKind("invalid"), "", "", ""); err == nil {
			t.Fatalf("narrowScopeCeiling(invalid) error = nil, want validation error")
		}
	})
}

func newTestCapabilityChecker(
	extName string,
	source ExtensionSource,
	permissions []string,
	_ ...[]string,
) *CapabilityChecker {
	checker := &CapabilityChecker{}
	checker.Register(extName, source, &Manifest{
		Permissions: PermissionsConfig{
			Requires: permissions,
		},
	})
	return checker
}
