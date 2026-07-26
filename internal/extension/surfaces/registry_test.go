package surfaces

import (
	"slices"
	"testing"

	"github.com/compozy/agh/internal/resources"
)

func TestLookupReturnsFirstWaveSurfaceMetadata(t *testing.T) {
	t.Parallel()

	surface, ok := Lookup(resources.ResourceKind("tool"))
	if !ok {
		t.Fatal("Lookup(tool) ok = false, want true")
	}
	if !surface.ExtensionPublish {
		t.Fatal("Lookup(tool).ExtensionPublish = false, want true")
	}
	if surface.ManifestFamily != FamilyTools {
		t.Fatalf("Lookup(tool).ManifestFamily = %q, want %q", surface.ManifestFamily, FamilyTools)
	}
	if !slices.Equal(surface.LegalScopes, []resources.ResourceScopeKind{
		resources.ResourceScopeKindGlobal,
		resources.ResourceScopeKindWorkspace,
	}) {
		t.Fatalf("Lookup(tool).LegalScopes = %#v, want global+workspace", surface.LegalScopes)
	}
}

func TestResolveManifestRequestRejectsIllegalFamilyBeforeHandshake(t *testing.T) {
	t.Parallel()

	_, err := ResolveManifestRequest([]string{string(FamilyBridgeInstances)}, resources.ResourceScopeKindGlobal)
	if err == nil {
		t.Fatal("ResolveManifestRequest() error = nil, want daemon-only family rejection")
	}
}

func TestResolveManifestRequestRejectsIllegalScopeBeforeHandshake(t *testing.T) {
	t.Parallel()

	_, err := ResolveManifestRequest([]string{string(FamilyTools)}, resources.ResourceScopeKind("session"))
	if err == nil {
		t.Fatal("ResolveManifestRequest() error = nil, want invalid scope rejection")
	}
}

func TestNormalizeAllowedKindsRejectsDaemonOnlyKinds(t *testing.T) {
	t.Parallel()

	_, err := NormalizeAllowedKinds([]resources.ResourceKind{
		resources.ResourceKind("tool"),
		resources.ResourceKind("bridge.instance"),
	})
	if err == nil {
		t.Fatal("NormalizeAllowedKinds() error = nil, want daemon-only rejection")
	}
}

func TestResolveManifestRequestExpandsGlobalScopeToGrantedScopeSet(t *testing.T) {
	t.Parallel()

	request, err := ResolveManifestRequest(
		[]string{string(FamilyTools), string(FamilyMCPServers), string(FamilyWindowLayouts)},
		resources.ResourceScopeKindGlobal,
	)
	if err != nil {
		t.Fatalf("ResolveManifestRequest() error = %v", err)
	}
	if !slices.Equal(request.Kinds, []resources.ResourceKind{
		resources.ResourceKind("mcp_server"),
		resources.ResourceKind("tool"),
		resources.ResourceKind("window_layout"),
	}) {
		t.Fatalf("ResolveManifestRequest().Kinds = %#v, want tool+mcp_server+window_layout", request.Kinds)
	}
	if !slices.Equal(request.Scopes, []resources.ResourceScopeKind{
		resources.ResourceScopeKindGlobal,
		resources.ResourceScopeKindWorkspace,
	}) {
		t.Fatalf("ResolveManifestRequest().Scopes = %#v, want global+workspace", request.Scopes)
	}
}

func TestAllAndPublishableKindsReturnClonedStaticRegistry(t *testing.T) {
	t.Parallel()

	all := All()
	if len(all) == 0 {
		t.Fatal("All() = empty, want first-wave registry")
	}
	all[0].LegalScopes = nil

	fetched, ok := Lookup(all[0].Kind)
	if !ok {
		t.Fatalf("Lookup(%q) ok = false, want true", all[0].Kind)
	}
	if len(fetched.LegalScopes) == 0 {
		t.Fatal("Lookup().LegalScopes mutated through All() clone")
	}

	publishable := PublishableKinds()
	if len(publishable) != 9 {
		t.Fatalf("PublishableKinds() len = %d, want 9", len(publishable))
	}
}

func TestResolveManifestRequestAllowsEmptyRequest(t *testing.T) {
	t.Parallel()

	request, err := ResolveManifestRequest(nil, "")
	if err != nil {
		t.Fatalf("ResolveManifestRequest(nil) error = %v", err)
	}
	if len(request.Kinds) != 0 || len(request.Scopes) != 0 || request.MaxScope != "" {
		t.Fatalf("ResolveManifestRequest(nil) = %#v, want zero value", request)
	}
}

func TestNormalizeAllowedKindsRejectsUnknownKinds(t *testing.T) {
	t.Parallel()

	_, err := NormalizeAllowedKinds([]resources.ResourceKind{resources.ResourceKind("unknown.kind")})
	if err == nil {
		t.Fatal("NormalizeAllowedKinds() error = nil, want unknown-kind rejection")
	}
}
