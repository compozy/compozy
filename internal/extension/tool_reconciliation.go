package extensionpkg

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

// ExtensionToolRuntimeState captures extension lifecycle state needed for manifest/runtime reconciliation.
type ExtensionToolRuntimeState struct {
	Enabled              bool
	Active               bool
	Healthy              bool
	ProvidedCapabilities []string
}

// ReconcileManifestToolRuntime reports whether a manifest-authoritative extension tool is executable.
func ReconcileManifestToolRuntime(
	manifest *ManifestToolDescriptor,
	runtime *toolspkg.ExtensionToolRuntimeDescriptor,
	state ExtensionToolRuntimeState,
) toolspkg.Availability {
	availability := toolspkg.Availability{
		Registered: true,
		Enabled:    state.Enabled,
		Authorized: true,
	}
	reasons := make([]toolspkg.ReasonCode, 0, 4)
	if !state.Enabled {
		reasons = appendToolReason(reasons, toolspkg.ReasonSourceDisabled)
	}
	if !state.Active {
		reasons = appendToolReason(reasons, toolspkg.ReasonExtensionInactive)
	}
	if !state.Healthy {
		reasons = appendToolReason(reasons, toolspkg.ReasonBackendUnhealthy)
	}
	if manifest == nil {
		reasons = appendToolReason(reasons, toolspkg.ReasonRuntimeDescriptorMissing)
		availability.ReasonCodes = reasons
		return availability
	}
	if !hasRequiredCapabilities(state.ProvidedCapabilities, manifest.Tool.Backend.RequiresCapabilities) {
		reasons = appendToolReason(reasons, toolspkg.ReasonExtensionCapabilityMissing)
	}
	reasons = appendRuntimeDescriptorReasons(reasons, manifest, runtime)
	if len(reasons) == 0 {
		availability.Available = true
		availability.Executable = true
	}
	availability.ReasonCodes = reasons
	return availability
}

func appendRuntimeDescriptorReasons(
	reasons []toolspkg.ReasonCode,
	manifest *ManifestToolDescriptor,
	runtime *toolspkg.ExtensionToolRuntimeDescriptor,
) []toolspkg.ReasonCode {
	if runtime == nil {
		return appendToolReason(reasons, toolspkg.ReasonRuntimeDescriptorMissing)
	}
	if strings.TrimSpace(runtime.Handler) == "" {
		reasons = appendToolReason(reasons, toolspkg.ReasonHandlerMissing)
	}
	if err := runtime.Validate(); err != nil {
		return appendToolReason(reasons, toolspkg.ReasonRuntimeDescriptorMismatch)
	}
	if !runtimeDescriptorMatches(manifest, *runtime) {
		reasons = appendToolReason(reasons, toolspkg.ReasonRuntimeDescriptorMismatch)
		reasons = appendToolReason(reasons, toolspkg.ReasonExtensionRuntimeMismatch)
	}
	return reasons
}

func runtimeDescriptorMatches(
	tool *ManifestToolDescriptor,
	runtime toolspkg.ExtensionToolRuntimeDescriptor,
) bool {
	manifest := tool.RuntimeDescriptor
	return manifest.ID == runtime.ID &&
		strings.TrimSpace(manifest.Handler) == strings.TrimSpace(runtime.Handler) &&
		strings.TrimSpace(manifest.FriendlyVerb) == strings.TrimSpace(runtime.FriendlyVerb) &&
		strings.TrimSpace(manifest.Preview) == strings.TrimSpace(runtime.Preview) &&
		extensionSchemaDigestMatches(tool.Tool.InputSchema, manifest.InputSchemaDigest, runtime.InputSchemaDigest) &&
		extensionSchemaDigestMatches(tool.Tool.OutputSchema, manifest.OutputSchemaDigest, runtime.OutputSchemaDigest) &&
		manifest.ReadOnly == runtime.ReadOnly &&
		manifest.Risk == runtime.Risk &&
		manifest.RequiresInteraction == runtime.RequiresInteraction &&
		commandSpecsEqual(manifest.Command, runtime.Command)
}

func extensionSchemaDigestMatches(schema json.RawMessage, identity, supplied string) bool {
	supplied = strings.TrimSpace(supplied)
	identity = strings.TrimSpace(identity)
	if identity == supplied {
		return true
	}
	if identity == "" || supplied == "" {
		return false
	}
	// Recompute against the authoritative schema before accepting the SDK's separator encoding.
	digest, err := toolspkg.TypeScriptSeparatorSchemaDigest(schema)
	return err == nil && supplied == digest
}

func commandSpecsEqual(left, right *toolspkg.ExtensionCommandSpec) bool {
	if left == nil || right == nil {
		return left == right
	}
	return strings.TrimSpace(left.Verb) == strings.TrimSpace(right.Verb) &&
		strings.TrimSpace(left.Summary) == strings.TrimSpace(right.Summary) &&
		strings.TrimSpace(left.Example) == strings.TrimSpace(right.Example) &&
		maps.Equal(left.Flags, right.Flags)
}

func hasRequiredCapabilities(provided []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	normalized := normalizeStrings(provided)
	for _, capability := range required {
		if !slices.Contains(normalized, strings.TrimSpace(capability)) {
			return false
		}
	}
	return true
}

func appendToolReason(reasons []toolspkg.ReasonCode, reason toolspkg.ReasonCode) []toolspkg.ReasonCode {
	if reason == "" || slices.Contains(reasons, reason) {
		return reasons
	}
	return append(reasons, reason)
}
