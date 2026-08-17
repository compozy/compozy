package extensionpkg

import (
	"context"
	"fmt"
	"os"
)

func buildResourceOnlyBundle(ctx context.Context, req BuildRequest) (*BuildResult, error) {
	manifest, err := LoadManifest(req.SourceDir)
	if err != nil {
		return nil, fmt.Errorf(
			"extension: unsupported source; expected package.json or go.mod, or a native extension.toml "+
				"with static resources: %w",
			err,
		)
	}
	if err := validateResourceOnlyBuildRequest(req, manifest); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("extension: create build output %q: %w", req.OutputDir, err)
	}
	if err := prepareBuildOutput(req.OutputDir); err != nil {
		return nil, err
	}
	return publishBuildGeneration(ctx, req.OutputDir, req.SourceDir, manifest)
}

func validateResourceOnlyBuildRequest(req BuildRequest, manifest *Manifest) error {
	if manifest == nil {
		return &ManifestValidationError{Field: manifestResourcesKey, Message: "manifest is required"}
	}
	if manifest.Format != FormatCompozy {
		return &ManifestValidationError{
			Field:   "format",
			Message: "resource-only authoring requires a native Compozy manifest",
		}
	}
	if len(req.BuildCmd) > 0 {
		return &ManifestValidationError{
			Field:   "build_command",
			Message: "a build command requires package.json or go.mod",
		}
	}
	if resourceOnlyManifestRequiresToolchain(manifest) {
		return &ManifestValidationError{
			Field:   manifestSubprocessKey,
			Message: "executable extension behavior requires package.json or go.mod",
		}
	}
	if !hasStaticResourcePaths(manifest.Resources) {
		return &ManifestValidationError{
			Field: manifestResourcesKey,
			Message: "resource-only extensions must declare at least one skill, loop, agent, " +
				"automation, or layout path",
		}
	}
	return nil
}

func resourceOnlyManifestRequiresToolchain(manifest *Manifest) bool {
	if requiresSubprocess(manifest) {
		return true
	}
	resources := manifest.Resources
	return len(resources.Hooks) > 0 || len(resources.Tools) > 0 ||
		len(resources.CommandGroups) > 0 || len(resources.MCPServers) > 0 ||
		manifest.Bridge.Platform != "" || manifest.Bridge.DisplayName != "" ||
		len(manifest.Bridge.SecretSlots) > 0 || manifest.Bridge.ConfigSchema != nil
}

func hasStaticResourcePaths(resources ResourcesConfig) bool {
	for _, group := range staticResourcePathGroups(resources) {
		if len(group.paths) > 0 {
			return true
		}
	}
	return false
}
