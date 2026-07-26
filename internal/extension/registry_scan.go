package extensionpkg

import (
	"context"

	"database/sql"

	"encoding/json"

	"fmt"

	"strings"

	"github.com/compozy/agh/internal/store"
)

func sqliteTableExists(db *sql.DB, tableName string) (bool, error) {
	var count int
	err := db.QueryRowContext(
		registryContext(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		strings.TrimSpace(tableName),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("extension: query sqlite table %q: %w", tableName, err)
	}
	return count > 0, nil
}

func scanExtensionInfo(scanner interface{ Scan(dest ...any) error }) (*ExtensionInfo, error) {
	var (
		info            ExtensionInfo
		sourceText      string
		installedAtText string
		capabilitiesRaw string
		actionsRaw      string
		registrySlug    sql.NullString
		registryName    sql.NullString
		remoteVersion   sql.NullString
		provenanceRaw   string
	)

	if err := scanner.Scan(
		&info.Name,
		&info.Version,
		&sourceText,
		&info.Enabled,
		&info.ManifestPath,
		&installedAtText,
		&capabilitiesRaw,
		&actionsRaw,
		&info.Checksum,
		&registrySlug,
		&registryName,
		&remoteVersion,
		&provenanceRaw,
	); err != nil {
		return nil, err
	}

	source, err := parseExtensionSource(sourceText)
	if err != nil {
		return nil, err
	}
	info.Source = source

	info.InstalledAt, err = store.ParseTimestamp(installedAtText)
	if err != nil {
		return nil, fmt.Errorf("extension: parse installed_at for %q: %w", info.Name, err)
	}

	if err := decodeRegistryJSON(capabilitiesRaw, &info.Capabilities); err != nil {
		return nil, fmt.Errorf("extension: decode capabilities for %q: %w", info.Name, err)
	}
	if err := decodeRegistryJSON(actionsRaw, &info.Actions); err != nil {
		return nil, fmt.Errorf("extension: decode actions for %q: %w", info.Name, err)
	}

	info.Capabilities = normalizeCapabilitiesConfig(info.Capabilities)
	info.Actions = normalizeActionsConfig(info.Actions)
	info.RegistrySlug = nullableStringPointer(registrySlug)
	info.RegistryName = nullableStringPointer(registryName)
	info.RemoteVersion = nullableStringPointer(remoteVersion)
	if err := decodeRegistryJSON(provenanceRaw, &info.Provenance); err != nil {
		return nil, fmt.Errorf("extension: decode provenance for %q: %w", info.Name, err)
	}
	info.Provenance = normalizeExtensionProvenance(info.Provenance, ExtensionProvenance{
		Slug:             dereferenceOptionalString(info.RegistrySlug),
		InstalledFrom:    installedFromForSource(info.Source),
		SourceURL:        info.ManifestPath,
		ChecksumSHA256:   info.Checksum,
		ChecksumVerified: info.Source != SourceMarketplace,
		RegistryTier:     registryTierForSource(info.Source, dereferenceOptionalString(info.RegistryName)),
		Permissions:      extensionPermissionsFromParts(info.Capabilities, info.Actions),
		InstalledAt:      info.InstalledAt,
		InstalledBy:      extensionTrustInstalledByOperator,
		AllowUnverified:  false,
	})

	return &info, nil
}

func registryContext() context.Context {
	return context.Background()
}

func nullableStringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return optionalInstallString(value.String)
}

func nullableStringValue(value *string) any {
	if value == nil {
		return nil
	}
	return strings.TrimSpace(*value)
}

func optionalInstallString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func decodeRegistryJSON(raw string, target any) error {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		payload = "{}"
	}
	return json.Unmarshal([]byte(payload), target)
}
