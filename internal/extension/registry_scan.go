package extensionpkg

import (
	"context"

	"database/sql"

	"encoding/json"

	"fmt"

	"strings"

	"github.com/compozy/compozy/internal/store"
)

func scanExtensionInfo(scanner interface{ Scan(dest ...any) error }) (*ExtensionInfo, error) {
	var row extensionInfoRow
	if err := row.scan(scanner); err != nil {
		return nil, err
	}
	if err := row.decode(); err != nil {
		return nil, err
	}
	return &row.info, nil
}

type extensionInfoRow struct {
	info               ExtensionInfo
	sourceText         string
	installedAtText    string
	providesRaw        string
	permissionsRaw     string
	registrySlug       sql.NullString
	registryName       sql.NullString
	remoteVersion      sql.NullString
	provenanceRaw      string
	networkConfirmedBy sql.NullString
	networkConfirmedAt sql.NullString
}

func (r *extensionInfoRow) scan(scanner interface{ Scan(dest ...any) error }) error {
	return scanner.Scan(
		&r.info.Name,
		&r.info.Version,
		&r.sourceText,
		&r.info.Enabled,
		&r.info.ManifestPath,
		&r.installedAtText,
		&r.providesRaw,
		&r.permissionsRaw,
		&r.info.Checksum,
		&r.registrySlug,
		&r.registryName,
		&r.remoteVersion,
		&r.provenanceRaw,
		&r.info.NetworkRequirementDigest,
		&r.networkConfirmedBy,
		&r.networkConfirmedAt,
	)
}

func (r *extensionInfoRow) decode() error {
	source, err := parseExtensionSource(r.sourceText)
	if err != nil {
		return err
	}
	r.info.Source = source

	r.info.InstalledAt, err = store.ParseTimestamp(r.installedAtText)
	if err != nil {
		return fmt.Errorf("extension: parse installed_at for %q: %w", r.info.Name, err)
	}

	if err := decodeRegistryJSONArray(r.providesRaw, &r.info.Capabilities.Provides); err != nil {
		return fmt.Errorf("extension: decode capabilities for %q: %w", r.info.Name, err)
	}
	if err := decodeRegistryJSONArray(r.permissionsRaw, &r.info.Permissions.Requires); err != nil {
		return fmt.Errorf("extension: decode permissions for %q: %w", r.info.Name, err)
	}

	r.info.Capabilities = normalizeCapabilitiesConfig(r.info.Capabilities)
	r.info.Permissions = normalizePermissionsConfig(r.info.Permissions)
	r.info.RegistrySlug = nullableStringPointer(r.registrySlug)
	r.info.RegistryName = nullableStringPointer(r.registryName)
	r.info.RemoteVersion = nullableStringPointer(r.remoteVersion)
	if err := decodeRegistryJSON(r.provenanceRaw, &r.info.Provenance); err != nil {
		return fmt.Errorf("extension: decode provenance for %q: %w", r.info.Name, err)
	}
	fallbackProvenance := extensionInstallProvenance(
		r.info.Source,
		r.info.ManifestPath,
		r.info.Checksum,
		extensionPermissionsFromParts(r.info.Capabilities, r.info.Permissions),
		r.info.InstalledAt,
	)
	fallbackProvenance.Slug = dereferenceOptionalString(r.info.RegistrySlug)
	r.info.Provenance = normalizeExtensionProvenance(r.info.Provenance, fallbackProvenance)
	r.info.NetworkRequirementDigest = strings.TrimSpace(r.info.NetworkRequirementDigest)
	r.info.NetworkConfirmedBy = strings.TrimSpace(r.networkConfirmedBy.String)
	if r.networkConfirmedAt.Valid {
		r.info.NetworkConfirmedAt, err = store.ParseTimestamp(r.networkConfirmedAt.String)
		if err != nil {
			return fmt.Errorf("extension: parse network_confirmed_at for %q: %w", r.info.Name, err)
		}
	}
	return nil
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

func decodeRegistryJSONArray(raw string, target any) error {
	payload := strings.TrimSpace(raw)
	if payload == "" {
		payload = "[]"
	}
	return json.Unmarshal([]byte(payload), target)
}
