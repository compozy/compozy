package extensionpkg

import (
	"encoding/json"
	"slices"
	"testing"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestReconcileManifestToolRuntimeReportsAvailabilityReasons(t *testing.T) {
	t.Parallel()

	fixture := unicodeSeparatorSchemaFixture(t)
	manifest := mustManifestToolDescriptor(t, fixture.Schema)
	matching := manifest.RuntimeDescriptor
	if matching.InputSchemaDigest != fixture.IdentitySHA256 || matching.OutputSchemaDigest != fixture.IdentitySHA256 {
		t.Fatalf(
			"manifest digests = (%q, %q), want %q",
			matching.InputSchemaDigest,
			matching.OutputSchemaDigest,
			fixture.IdentitySHA256,
		)
	}
	sdk := matching
	sdk.InputSchemaDigest = fixture.SHA256
	sdk.OutputSchemaDigest = fixture.SHA256
	missingHandler := matching
	missingHandler.Handler = ""
	mismatchedDigest := matching
	mismatchedDigest.InputSchemaDigest = "bad-digest"
	healthyState := ExtensionToolRuntimeState{
		Enabled:              true,
		Active:               true,
		Healthy:              true,
		ProvidedCapabilities: []string{"tool.provider"},
	}

	tests := []struct {
		name       string
		state      ExtensionToolRuntimeState
		runtime    *toolspkg.ExtensionToolRuntimeDescriptor
		executable bool
		reasons    []toolspkg.ReasonCode
	}{
		{
			name:       "Should Mark Matching Runtime Descriptor Executable",
			state:      healthyState,
			runtime:    &matching,
			executable: true,
		},
		{
			name:       "Should reconcile separator digests from the TypeScript SDK",
			state:      healthyState,
			runtime:    &sdk,
			executable: true,
		},
		{
			name: "Should Report Disabled Extension",
			state: ExtensionToolRuntimeState{
				Enabled:              false,
				Active:               true,
				Healthy:              true,
				ProvidedCapabilities: []string{"tool.provider"},
			},
			runtime: &matching,
			reasons: []toolspkg.ReasonCode{toolspkg.ReasonSourceDisabled},
		},
		{
			name: "Should Report Inactive Extension",
			state: ExtensionToolRuntimeState{
				Enabled:              true,
				Active:               false,
				Healthy:              true,
				ProvidedCapabilities: []string{"tool.provider"},
			},
			runtime: &matching,
			reasons: []toolspkg.ReasonCode{toolspkg.ReasonExtensionInactive},
		},
		{
			name: "Should Report Unhealthy Extension",
			state: ExtensionToolRuntimeState{
				Enabled:              true,
				Active:               true,
				Healthy:              false,
				ProvidedCapabilities: []string{"tool.provider"},
			},
			runtime: &matching,
			reasons: []toolspkg.ReasonCode{toolspkg.ReasonBackendUnhealthy},
		},
		{
			name:    "Should Report Missing Tool Provider Capability",
			state:   ExtensionToolRuntimeState{Enabled: true, Active: true, Healthy: true},
			runtime: &matching,
			reasons: []toolspkg.ReasonCode{toolspkg.ReasonExtensionCapabilityMissing},
		},
		{
			name:    "Should Report Missing Runtime Descriptor",
			state:   healthyState,
			runtime: nil,
			reasons: []toolspkg.ReasonCode{toolspkg.ReasonRuntimeDescriptorMissing},
		},
		{
			name:    "Should Report Missing Handler",
			state:   healthyState,
			runtime: &missingHandler,
			reasons: []toolspkg.ReasonCode{
				toolspkg.ReasonHandlerMissing,
				toolspkg.ReasonRuntimeDescriptorMismatch,
			},
		},
		{
			name:    "Should Report Runtime Descriptor Mismatch",
			state:   healthyState,
			runtime: &mismatchedDigest,
			reasons: []toolspkg.ReasonCode{
				toolspkg.ReasonRuntimeDescriptorMismatch,
				toolspkg.ReasonExtensionRuntimeMismatch,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ReconcileManifestToolRuntime(&manifest, tt.runtime, tt.state)
			if err := got.Validate(); err != nil {
				t.Fatalf("Availability.Validate() error = %v", err)
			}
			if got.Executable != tt.executable {
				t.Fatalf("Availability.Executable = %v, want %v", got.Executable, tt.executable)
			}
			if got.Available != tt.executable {
				t.Fatalf("Availability.Available = %v, want %v", got.Available, tt.executable)
			}
			if !slices.Equal(got.ReasonCodes, tt.reasons) {
				t.Fatalf("Availability.ReasonCodes = %#v, want %#v", got.ReasonCodes, tt.reasons)
			}
		})
	}
}

func mustManifestToolDescriptor(t *testing.T, schema json.RawMessage) ManifestToolDescriptor {
	t.Helper()

	descriptors, err := ResolveManifestToolDescriptors(&Manifest{
		Name: "linear",
		Resources: ResourcesConfig{
			Tools: map[string]ToolConfig{
				"lookup": {
					Description:  "Search workspace",
					Backend:      ToolBackendConfig{Kind: "extension_host", Handler: "lookup"},
					InputSchema:  schema,
					OutputSchema: schema,
					ReadOnly:     true,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	if got, want := len(descriptors), 1; got != want {
		t.Fatalf("len(ResolveManifestToolDescriptors()) = %d, want %d", got, want)
	}
	return descriptors[0]
}
