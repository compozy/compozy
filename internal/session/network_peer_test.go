package session

import (
	"reflect"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestNetworkPeerCapabilitiesProjectsUnifiedFieldsAndDeepCopiesCatalogData(t *testing.T) {
	t.Parallel()

	agent := aghconfig.AgentDef{
		Name:   "coder",
		Prompt: "You write reliable code.",
		Capabilities: &aghconfig.CapabilityCatalog{
			Capabilities: []aghconfig.CapabilityDef{{
				ID:                " review-pr ",
				Summary:           " Review pull requests. ",
				Outcome:           " Actionable review feedback. ",
				Version:           " 1.0.0 ",
				ContextNeeded:     []string{" diff ", "", " acceptance criteria "},
				ArtifactsExpected: []string{" review summary "},
				ExecutionOutline:  []string{" inspect ", " comment "},
				Constraints:       []string{" cite exact files "},
				Examples:          []string{" auth regression review "},
				Requirements:      []string{" workspace-write ", "review-guidelines"},
			}},
		},
	}
	if err := agent.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	catalog := agent.Capabilities
	got := networkPeerCapabilities(catalog)
	want := []NetworkPeerCapability{{
		ID:                "review-pr",
		Summary:           "Review pull requests.",
		Outcome:           "Actionable review feedback.",
		Version:           "1.0.0",
		Digest:            catalog.Capabilities[0].Digest,
		ContextNeeded:     []string{"diff", "acceptance criteria"},
		ArtifactsExpected: []string{"review summary"},
		ExecutionOutline:  []string{"inspect", "comment"},
		Constraints:       []string{"cite exact files"},
		Examples:          []string{"auth regression review"},
		Requirements:      []string{"review-guidelines", "workspace-write"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("networkPeerCapabilities() = %#v, want %#v", got, want)
	}

	got[0].Summary = "mutated"
	got[0].Digest = "sha256:mutated"
	got[0].ContextNeeded[0] = "mutated context"
	got[0].Requirements[0] = "mutated requirement"

	capability := catalog.Capabilities[0]
	if got, want := capability.Summary, "Review pull requests."; got != want {
		t.Fatalf("catalog summary mutated = %q, want %q", got, want)
	}
	if got, want := capability.ContextNeeded, []string{"diff", "acceptance criteria"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("catalog context mutated = %#v, want %#v", got, want)
	}
	if got, want := capability.Requirements, []string{
		"review-guidelines",
		"workspace-write",
	}; !reflect.DeepEqual(
		got,
		want,
	) {
		t.Fatalf("catalog requirements mutated = %#v, want %#v", got, want)
	}
	if got := capability.Digest; got == "" || got == "sha256:mutated" {
		t.Fatalf("catalog digest mutated = %q, want stable computed digest", got)
	}
}
