package extensionpkg

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"strings"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
)

const (
	manifestFieldCapabilitiesProvides = "capabilities.provides"
	manifestFieldNetworkParticipation = "network_participation"
)

func (r *Registry) installWithConfig(manifest *Manifest, path string, checksum string, config installConfig) error {
	if err := r.checkReady("install extension"); err != nil {
		return err
	}
	_, err := validateInstallConfig(manifest, checksum, &config)
	if err != nil {
		return err
	}

	resolvedManifest, manifestPath, actualChecksum, err := loadVerifiedInstallManifest(manifest, path, checksum)
	if err != nil {
		return err
	}
	sourceText, err := validateInstallConfig(resolvedManifest, actualChecksum, &config)
	if err != nil {
		return err
	}

	info, err := registryInstallInfo(r, resolvedManifest, manifestPath, actualChecksum, config)
	if err != nil {
		return err
	}
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
	if config.source != SourceBundled && providesCapability(
		manifest.Capabilities.Provides,
		extensionprotocol.CapabilityProvideBridgeAdapter,
	) {
		return "", &ManifestValidationError{
			Field:   manifestFieldCapabilitiesProvides,
			Value:   extensionprotocol.CapabilityProvideBridgeAdapter,
			Message: "external bridge authoring is a planned follow-up",
		}
	}
	if providesCapability(
		manifest.Capabilities.Provides,
		extensionprotocol.CapabilityProvideConnectivityProvider,
	) {
		if config.source == SourceWorkspace {
			return "", &ManifestValidationError{
				Field:   manifestFieldCapabilitiesProvides,
				Value:   extensionprotocol.CapabilityProvideConnectivityProvider,
				Message: "connectivity providers must be installed globally",
			}
		}
		digest, digestErr := NetworkParticipationRequirementDigest(manifest.NetworkParticipation)
		if digestErr != nil {
			return "", digestErr
		}
		if digest == "" {
			return "", &ManifestValidationError{
				Field:   manifestFieldNetworkParticipation,
				Message: "connectivity providers must declare required live network participation",
			}
		}
		normalized := manifest.NetworkParticipation.Normalize()
		privateScope := gateway.ProviderChannelScope(gateway.TierPrivate)
		publicScope := gateway.ProviderChannelScope(gateway.TierPublic)
		if normalized == nil || (!slices.Contains(normalized.ChannelScopes, privateScope) &&
			!slices.Contains(normalized.ChannelScopes, publicScope)) {
			return "", &ManifestValidationError{
				Field: "network_participation.channel_scopes",
				Message: fmt.Sprintf(
					"connectivity providers must declare %q or %q",
					privateScope,
					publicScope,
				),
			}
		}
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
) (ExtensionInfo, error) {
	installedAt := r.now().UTC()
	if config.installedAt != nil {
		installedAt = config.installedAt.UTC()
	}
	capabilities := normalizeCapabilitiesConfig(resolvedManifest.Capabilities)
	permissions := normalizePermissionsConfig(resolvedManifest.Permissions)
	fallbackProvenance := extensionInstallProvenance(
		config.source,
		manifestPath,
		actualChecksum,
		extensionPermissions(resolvedManifest),
		installedAt,
	)
	fallbackProvenance.Slug = dereferenceOptionalString(config.registrySlug)
	provenance := fallbackProvenance
	if config.provenance != nil {
		provenance = normalizeExtensionProvenance(*config.provenance, fallbackProvenance)
	}
	networkDigest, err := NetworkParticipationRequirementDigest(resolvedManifest.NetworkParticipation)
	if err != nil {
		return ExtensionInfo{}, err
	}
	return ExtensionInfo{
		Name:                     strings.TrimSpace(resolvedManifest.Name),
		Version:                  strings.TrimSpace(resolvedManifest.Version),
		Source:                   config.source,
		Enabled:                  config.enabled,
		ManifestPath:             manifestPath,
		Format:                   normalizeExtensionFormat(resolvedManifest.Format),
		IngestDiagnostics:        cloneDiagnosticItems(resolvedManifest.IngestDiagnostics),
		InstalledAt:              installedAt,
		Capabilities:             capabilities,
		Permissions:              permissions,
		Checksum:                 actualChecksum,
		RegistrySlug:             config.registrySlug,
		RegistryName:             config.registryName,
		RemoteVersion:            config.remoteVersion,
		Provenance:               provenance,
		NetworkRequirementDigest: networkDigest,
	}, nil
}

func (r *Registry) persistInstalledInfo(
	info ExtensionInfo,
	sourceText string,
	replaceExisting bool,
) (resultErr error) {
	encoded, err := marshalInstalledInfoFields(info)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO extensions (
` + registryInsertColumns + `
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	if replaceExisting {
		query += `
		ON CONFLICT(name) DO UPDATE SET
			version = excluded.version,
			source = excluded.source,
			manifest_path = excluded.manifest_path,
			format = excluded.format,
			ingest_diagnostics_json = excluded.ingest_diagnostics_json,
			installed_at = excluded.installed_at,
			provides_json = excluded.provides_json,
			permissions_json = excluded.permissions_json,
			checksum = excluded.checksum,
			registry_slug = excluded.registry_slug,
			registry_name = excluded.registry_name,
			remote_version = excluded.remote_version,
			provenance_json = excluded.provenance_json
			,network_requirement_digest = excluded.network_requirement_digest
			,network_confirmed_by = CASE
				WHEN extensions.network_requirement_digest = excluded.network_requirement_digest
				THEN extensions.network_confirmed_by
				ELSE NULL
			END
			,network_confirmed_at = CASE
				WHEN extensions.network_requirement_digest = excluded.network_requirement_digest
				THEN extensions.network_confirmed_at
				ELSE NULL
			END
		`
	}

	tx, err := r.db.BeginTx(registryContext(), nil)
	if err != nil {
		return fmt.Errorf("extension: begin persist %q: %w", info.Name, err)
	}
	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, fmt.Errorf("extension: roll back persist: %w", rollbackErr))
		}
	}()

	var existed bool
	if err := tx.QueryRowContext(
		registryContext(),
		`SELECT EXISTS(SELECT 1 FROM extensions WHERE name = ?)`,
		info.Name,
	).Scan(&existed); err != nil {
		return fmt.Errorf("extension: check existing install %q: %w", info.Name, err)
	}

	_, err = tx.ExecContext(
		registryContext(),
		query,
		info.Name,
		info.Version,
		sourceText,
		info.ManifestPath,
		string(normalizeExtensionFormat(info.Format)),
		string(encoded.diagnostics),
		store.FormatTimestamp(info.InstalledAt),
		string(encoded.capabilities),
		string(encoded.permissions),
		info.Checksum,
		nullableStringValue(info.RegistrySlug),
		nullableStringValue(info.RegistryName),
		nullableStringValue(info.RemoteVersion),
		string(encoded.provenance),
		strings.TrimSpace(info.NetworkRequirementDigest),
		nullableRegistryString(info.NetworkConfirmedBy),
		nullableRegistryTime(info.NetworkConfirmedAt),
	)
	if err != nil {
		if replaceExisting {
			return fmt.Errorf("extension: persist %q: %w", info.Name, err)
		}
		return mapRegistryConstraintError(err, info.Name)
	}
	if !existed && !info.Enabled {
		if _, err := tx.ExecContext(
			registryContext(),
			`INSERT INTO extension_profile_enablement (extension_name, profile_id, enabled) VALUES (?, ?, 0)`,
			info.Name,
			store.DefaultProfileID,
		); err != nil {
			return fmt.Errorf("extension: persist default-profile enablement for %q: %w", info.Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("extension: commit persist %q: %w", info.Name, err)
	}
	r.invalidateEnabledBundledNames()
	return nil
}

type installedInfoJSONFields struct {
	capabilities []byte
	permissions  []byte
	diagnostics  []byte
	provenance   []byte
}

func marshalInstalledInfoFields(info ExtensionInfo) (installedInfoJSONFields, error) {
	values := installedInfoJSONFields{}
	fields := []struct {
		name  string
		value any
		dest  *[]byte
	}{
		{name: manifestCapabilitiesKey, value: info.Capabilities.Provides, dest: &values.capabilities},
		{name: "permissions", value: info.Permissions.Requires, dest: &values.permissions},
		{name: "ingest diagnostics", value: info.IngestDiagnostics, dest: &values.diagnostics},
		{name: installedInfoProvenanceField, value: info.Provenance, dest: &values.provenance},
	}
	for _, field := range fields {
		encoded, err := json.Marshal(field.value)
		if err != nil {
			return installedInfoJSONFields{}, fmt.Errorf(
				"extension: marshal %s for %q: %w", field.name, info.Name, err,
			)
		}
		*field.dest = encoded
	}
	return values, nil
}

func normalizeExtensionFormat(format ExtensionFormat) ExtensionFormat {
	if format == FormatAgentPlugin {
		return FormatAgentPlugin
	}
	return FormatCompozy
}
