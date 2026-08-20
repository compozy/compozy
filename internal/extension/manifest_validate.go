package extensionpkg

import (
	"strings"

	"github.com/compozy/compozy/internal/extension/surfaces"
)

func validateManifestIdentity(manifest *Manifest) error {
	if err := requireField(manifestNameKey, manifest.Name); err != nil {
		return err
	}
	if manifest.Format == FormatAgentPlugin && strings.TrimSpace(manifest.Version) == "" {
		return nil
	}
	if err := requireField(manifestVersionKey, manifest.Version); err != nil {
		return err
	}
	return validateSemanticVersionField(manifestVersionKey, manifest.Version)
}

func validateManifestRuntime(manifest *Manifest) error {
	if manifest.Format != FormatAgentPlugin {
		if err := validateNativeManifestCompatibility(manifest); err != nil {
			return err
		}
	}
	if err := validateEnvRequirements("requires_env", manifest.RequiresEnv); err != nil {
		return err
	}
	if err := manifest.NetworkParticipation.Validate(manifestFieldNetworkParticipation); err != nil {
		return err
	}
	return validateManifestEnvMaps(
		"subprocess", "extensions", manifest.Subprocess.Env, manifest.Subprocess.SecretEnv,
	)
}

func validateNativeManifestCompatibility(manifest *Manifest) error {
	if err := requireField(manifestMinCompozyVersionKey, manifest.MinCompozyVersion); err != nil {
		return err
	}
	if err := validateSemanticVersionField(manifestMinCompozyVersionKey, manifest.MinCompozyVersion); err != nil {
		return err
	}
	return validateDaemonCompatibility(manifest.MinCompozyVersion)
}

func validateManifestResources(manifest *Manifest) error {
	if err := validateManifestHookEnv(manifest.Resources.Hooks); err != nil {
		return err
	}
	if err := validateManifestMCPServerEnv(manifest.Resources.MCPServers); err != nil {
		return err
	}
	if err := validateManifestCommandResources(manifest); err != nil {
		return err
	}
	_, err := validateManifestCmdPalette(manifest)
	return err
}

func validateManifestCapabilities(manifest *Manifest) error {
	if err := validateDottedIdentifiers(
		manifestFieldCapabilitiesProvides, manifest.Capabilities.Provides, false,
	); err != nil {
		return err
	}
	if err := validateProvideCapabilities(manifest.Capabilities.Provides); err != nil {
		return err
	}
	if err := manifest.validateModelSourceCapability(); err != nil {
		return err
	}
	return manifest.validateBridgeAdapterCapability()
}

func validateManifestPermissions(manifest *Manifest) error {
	if err := validateSlashIdentifiers("permissions.requires", manifest.Permissions.Requires); err != nil {
		return err
	}
	return validatePermissionMethods(manifest.Permissions.Requires)
}

func validateManifestPublication(manifest *Manifest) error {
	if _, err := surfaces.ResolveManifestRequest(
		manifest.Resources.Publish.Families,
		manifest.Resources.Publish.MaxScope,
	); err != nil {
		return &ManifestValidationError{Field: manifestResourcesPublishPath, Message: err.Error()}
	}
	return nil
}
