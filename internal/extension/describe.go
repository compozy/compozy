package extensionpkg

import (
	"time"

	"github.com/compozy/agh/internal/api/contract"
)

const (
	describeActiveKey     = "active"
	describeDisabledKey   = "disabled"
	describeRegisteredKey = "registered"
	describeResourceKey   = "resource"
	describeSubprocessKey = "subprocess"
)

const (
	extensionStateEnabled    = "enabled"
	extensionStateError      = "error"
	extensionHealthUnknown   = hostAPIUnknownExtensionName
	extensionHealthHealthy   = "healthy"
	extensionHealthUnhealthy = "unhealthy"
)

// DescribeExtension projects one extension snapshot into the shared CLI/API payload.
func DescribeExtension(ext *Extension, daemonRunning bool, now time.Time) contract.ExtensionPayload {
	if ext == nil {
		return contract.ExtensionPayload{}
	}

	uptimeSeconds := int64(0)
	if ext.Status.Active && !ext.Status.LastStartedAt.IsZero() {
		uptimeSeconds = max(int64(now.Sub(ext.Status.LastStartedAt).Seconds()), 0)
	}

	requiresEnv := []string(nil)
	missingEnv := []string(nil)
	if ext.Manifest != nil {
		requiresEnv = append(requiresEnv, ext.Manifest.RequiresEnv...)
		if !ext.Status.MissingEnvChecked {
			missingEnv = ext.Manifest.MissingEnv(nil)
		}
	}
	if len(ext.Status.MissingEnv) > 0 {
		missingEnv = append([]string(nil), ext.Status.MissingEnv...)
	}

	return contract.ExtensionPayload{
		Name:          ext.Info.Name,
		Version:       ext.Info.Version,
		Type:          extensionType(ext.Manifest, ext.Info),
		Source:        ext.Info.Source.String(),
		Enabled:       ext.Info.Enabled,
		State:         extensionState(ext.Info, ext.Status, daemonRunning),
		Capabilities:  append([]string(nil), ext.Info.Capabilities.Provides...),
		Actions:       append([]string(nil), ext.Info.Actions.Requires...),
		RequiresEnv:   requiresEnv,
		MissingEnv:    missingEnv,
		PID:           ext.Status.PID,
		UptimeSeconds: uptimeSeconds,
		Health:        extensionHealth(ext.Manifest, ext.Info, ext.Status, daemonRunning),
		HealthMessage: ext.Status.HealthMessage,
		LastError:     ext.Status.LastError,
		DaemonRunning: daemonRunning,
		Bundles:       bundleSummaryPayloads(ext.Bundles),
		Provenance:    extensionProvenancePayload(ext.Info.Provenance),
		Trust:         extensionTrustPayload(ext.Info.Provenance),
		Diagnostics:   append([]contract.DiagnosticItem(nil), ext.Info.Provenance.Warnings...),
	}
}

func extensionType(manifest *Manifest, info ExtensionInfo) string {
	if requiresSubprocess(manifest) || len(info.Capabilities.Provides) > 0 || len(info.Actions.Requires) > 0 {
		return describeSubprocessKey
	}
	return describeResourceKey
}

func extensionState(info ExtensionInfo, status ExtensionStatus, daemonRunning bool) string {
	if !info.Enabled {
		return describeDisabledKey
	}
	if !daemonRunning {
		return extensionStateEnabled
	}
	if status.Active {
		return describeActiveKey
	}
	if status.LastError != "" {
		return extensionStateError
	}
	if status.Registered {
		return describeRegisteredKey
	}
	return extensionStateEnabled
}

func extensionHealth(manifest *Manifest, info ExtensionInfo, status ExtensionStatus, daemonRunning bool) string {
	if !daemonRunning {
		return extensionHealthUnknown
	}
	if status.Active {
		if status.Healthy ||
			(!requiresSubprocess(manifest) && len(info.Capabilities.Provides) == 0 && len(info.Actions.Requires) == 0) {
			return extensionHealthHealthy
		}
		return extensionHealthUnhealthy
	}
	if status.LastError != "" {
		return extensionHealthUnhealthy
	}
	if !requiresSubprocess(manifest) && len(info.Capabilities.Provides) == 0 && len(info.Actions.Requires) == 0 &&
		status.Registered {
		return extensionHealthHealthy
	}
	return extensionHealthUnknown
}

func bundleSummaryPayloads(values []BundleSpec) []contract.ExtensionBundleSummaryPayload {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]contract.ExtensionBundleSummaryPayload, 0, len(values))
	for _, value := range values {
		profiles := make([]string, 0, len(value.Profiles))
		for _, profile := range value.Profiles {
			profiles = append(profiles, profile.Name)
		}
		payloads = append(payloads, contract.ExtensionBundleSummaryPayload{
			Name:        value.Name,
			Description: value.Description,
			Profiles:    profiles,
		})
	}
	return payloads
}

func extensionProvenancePayload(
	value ExtensionProvenance,
) *contract.ExtensionProvenancePayload {
	if !hasExtensionProvenance(value) {
		return nil
	}
	return &contract.ExtensionProvenancePayload{
		Slug:                value.Slug,
		CatalogEntryID:      value.CatalogEntryID,
		InstalledFrom:       value.InstalledFrom,
		SourceURL:           value.SourceURL,
		ChecksumSHA256:      value.ChecksumSHA256,
		ArchiveDigestSHA256: value.ArchiveDigestSHA256,
		ChecksumVerified:    value.ChecksumVerified,
		RegistryTier:        value.RegistryTier,
		Permissions:         append([]string(nil), value.Permissions...),
		InstalledAt:         value.InstalledAt,
		InstalledBy:         value.InstalledBy,
		AllowUnverified:     value.AllowUnverified,
		Warnings:            append([]contract.DiagnosticItem(nil), value.Warnings...),
		Trust:               extensionTrustPayload(value),
	}
}

func extensionTrustPayload(value ExtensionProvenance) *contract.ExtensionTrustReportPayload {
	if !hasExtensionProvenance(value) {
		return nil
	}
	return &contract.ExtensionTrustReportPayload{
		Decision:         extensionTrustDecision(value),
		RegistryTier:     value.RegistryTier,
		ChecksumVerified: value.ChecksumVerified,
		AllowUnverified:  value.AllowUnverified,
		Warnings:         append([]contract.DiagnosticItem(nil), value.Warnings...),
	}
}

func hasExtensionProvenance(value ExtensionProvenance) bool {
	return value.Slug != "" ||
		value.CatalogEntryID != "" ||
		value.InstalledFrom != "" ||
		value.SourceURL != "" ||
		value.ChecksumSHA256 != "" ||
		value.ArchiveDigestSHA256 != "" ||
		value.RegistryTier != "" ||
		!value.InstalledAt.IsZero() ||
		value.InstalledBy != "" ||
		len(value.Permissions) > 0 ||
		len(value.Warnings) > 0
}
