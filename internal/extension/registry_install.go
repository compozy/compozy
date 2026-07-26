package extensionpkg

import (
	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/store"
)

func (r *Registry) installWithConfig(manifest *Manifest, path string, checksum string, config installConfig) error {
	if err := r.checkReady("install extension"); err != nil {
		return err
	}
	sourceText, err := validateInstallConfig(manifest, checksum, &config)
	if err != nil {
		return err
	}

	resolvedManifest, manifestPath, actualChecksum, err := loadVerifiedInstallManifest(manifest, path, checksum)
	if err != nil {
		return err
	}

	info := registryInstallInfo(r, resolvedManifest, manifestPath, actualChecksum, config)
	return r.persistInstalledInfo(info, sourceText, config.replaceExisting)
}

func applyInstallOptions(config *installConfig, opts ...InstallOption) {
	if config == nil {
		return
	}

	for _, opt := range opts {
		if opt != nil {
			opt(config)
		}
	}
}

func validateInstallConfig(manifest *Manifest, checksum string, config *installConfig) (string, error) {
	if manifest == nil {
		return "", errors.New("extension: manifest is required")
	}
	if err := manifest.Validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(checksum) == "" {
		return "", errors.New("extension: checksum is required")
	}
	if config == nil {
		return "", errors.New("extension: install config is required")
	}

	sourceText := config.source.String()
	if sourceText == "" {
		return "", fmt.Errorf("extension: invalid source %d", config.source)
	}
	if config.source != SourceMarketplace {
		config.registrySlug = nil
		config.registryName = nil
		config.remoteVersion = nil
	}
	return sourceText, nil
}

func loadVerifiedInstallManifest(
	manifest *Manifest,
	path string,
	checksum string,
) (*Manifest, string, string, error) {
	artifactRoot, manifestPath, err := resolveInstallArtifact(path)
	if err != nil {
		return nil, "", "", err
	}

	trimmedChecksum := strings.ToLower(strings.TrimSpace(checksum))
	actualChecksum, err := ComputeDirectoryChecksum(artifactRoot)
	if err != nil {
		return nil, "", "", err
	}
	if actualChecksum != trimmedChecksum {
		return nil, "", "", &ExtensionChecksumMismatchError{
			ExpectedChecksum: trimmedChecksum,
			ActualChecksum:   actualChecksum,
		}
	}

	resolvedManifest, err := loadManifestAtPath(manifestPath)
	if err != nil {
		return nil, "", "", err
	}
	if strings.TrimSpace(manifest.Name) != strings.TrimSpace(resolvedManifest.Name) ||
		strings.TrimSpace(manifest.Version) != strings.TrimSpace(resolvedManifest.Version) {
		return nil, "", "", fmt.Errorf(
			"extension: manifest %q does not match provided identity %q@%q",
			manifestPath,
			strings.TrimSpace(manifest.Name),
			strings.TrimSpace(manifest.Version),
		)
	}

	return resolvedManifest, manifestPath, actualChecksum, nil
}

func registryInstallInfo(
	r *Registry,
	resolvedManifest *Manifest,
	manifestPath string,
	actualChecksum string,
	config installConfig,
) ExtensionInfo {
	installedAt := r.now().UTC()
	capabilities := normalizeCapabilitiesConfig(resolvedManifest.Capabilities)
	actions := normalizeActionsConfig(resolvedManifest.Actions)
	fallbackProvenance := ExtensionProvenance{
		Slug:             dereferenceOptionalString(config.registrySlug),
		InstalledFrom:    installedFromForSource(config.source),
		SourceURL:        manifestPath,
		ChecksumSHA256:   actualChecksum,
		ChecksumVerified: config.source != SourceMarketplace,
		RegistryTier:     registryTierForSource(config.source, dereferenceOptionalString(config.registryName)),
		Permissions:      extensionPermissions(resolvedManifest),
		InstalledAt:      installedAt,
		InstalledBy:      extensionTrustInstalledByOperator,
	}
	provenance := fallbackProvenance
	if config.provenance != nil {
		provenance = normalizeExtensionProvenance(*config.provenance, fallbackProvenance)
	}
	return ExtensionInfo{
		Name:          strings.TrimSpace(resolvedManifest.Name),
		Version:       strings.TrimSpace(resolvedManifest.Version),
		Source:        config.source,
		Enabled:       true,
		ManifestPath:  manifestPath,
		InstalledAt:   installedAt,
		Capabilities:  capabilities,
		Actions:       actions,
		Checksum:      actualChecksum,
		RegistrySlug:  config.registrySlug,
		RegistryName:  config.registryName,
		RemoteVersion: config.remoteVersion,
		Provenance:    provenance,
	}
}

func (r *Registry) persistInstalledInfo(info ExtensionInfo, sourceText string, replaceExisting bool) error {
	capabilitiesJSON, err := json.Marshal(info.Capabilities)
	if err != nil {
		return fmt.Errorf("extension: marshal capabilities for %q: %w", info.Name, err)
	}
	actionsJSON, err := json.Marshal(info.Actions)
	if err != nil {
		return fmt.Errorf("extension: marshal actions for %q: %w", info.Name, err)
	}
	provenanceJSON, err := json.Marshal(info.Provenance)
	if err != nil {
		return fmt.Errorf("extension: marshal provenance for %q: %w", info.Name, err)
	}

	query := `
		INSERT INTO extensions (
` + registryInsertColumns + `
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if replaceExisting {
		query += `
		ON CONFLICT(name) DO UPDATE SET
			version = excluded.version,
			source = excluded.source,
			manifest_path = excluded.manifest_path,
			installed_at = excluded.installed_at,
			capabilities = excluded.capabilities,
			actions = excluded.actions,
			checksum = excluded.checksum,
			registry_slug = excluded.registry_slug,
			registry_name = excluded.registry_name,
			remote_version = excluded.remote_version,
			provenance_json = excluded.provenance_json
		`
	}

	_, err = r.db.ExecContext(
		registryContext(),
		query,
		info.Name,
		info.Version,
		sourceText,
		info.Enabled,
		info.ManifestPath,
		store.FormatTimestamp(info.InstalledAt),
		string(capabilitiesJSON),
		string(actionsJSON),
		info.Checksum,
		nullableStringValue(info.RegistrySlug),
		nullableStringValue(info.RegistryName),
		nullableStringValue(info.RemoteVersion),
		string(provenanceJSON),
	)
	if err != nil {
		if replaceExisting {
			return fmt.Errorf("extension: persist %q: %w", info.Name, err)
		}
		return mapRegistryConstraintError(err, info.Name)
	}
	r.invalidateEnabledBundledNames()
	return nil
}
