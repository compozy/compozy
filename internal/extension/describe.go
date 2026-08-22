package extensionpkg

import (
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
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
	state := extensionState(ext.Info, ext.Status, daemonRunning)
	lastError := ext.Status.LastError
	bootDiagnostic := devBootFailureDiagnostic(ext.Status.FailureCode)
	if bootDiagnostic != "" {
		lastError = bootDiagnostic
	}
	originPath := ""
	if ext.DevLink != nil && state != extensionStateError && bootDiagnostic == "" {
		originPath = ext.DevLink.OriginPath
	}

	return contract.ExtensionPayload{
		Name:                     ext.Info.Name,
		Profile:                  "default",
		WorkspaceID:              ext.Status.WorkspaceID,
		Version:                  ext.Info.Version,
		Type:                     extensionType(ext.Manifest, ext.Info),
		Format:                   string(normalizeExtensionFormat(ext.Info.Format)),
		Source:                   ext.Info.Source.String(),
		Enabled:                  ext.Info.Enabled,
		State:                    state,
		Capabilities:             append([]string(nil), ext.Info.Capabilities.Provides...),
		Permissions:              append([]string(nil), ext.Info.Permissions.Requires...),
		RequiresEnv:              requiresEnv,
		MissingEnv:               missingEnv,
		NetworkRequirementDigest: ext.Info.NetworkRequirementDigest,
		NetworkConfirmationRequired: strings.TrimSpace(ext.Info.NetworkRequirementDigest) != "" &&
			(strings.TrimSpace(ext.Info.NetworkConfirmedBy) == "" || ext.Info.NetworkConfirmedAt.IsZero()),
		PID:                 ext.Status.PID,
		UptimeSeconds:       uptimeSeconds,
		Health:              extensionHealth(ext.Manifest, ext.Info, ext.Status, daemonRunning),
		HealthMessage:       ext.Status.HealthMessage,
		LastError:           lastError,
		FailureCode:         ext.Status.FailureCode,
		ConsecutiveFailures: ext.Status.ConsecutiveFailures,
		RestartBackoffMS:    ext.Status.RestartBackoff.Milliseconds(),
		GenerationHash:      ext.Status.GenerationHash,
		Dev:                 ext.DevLink != nil || ext.Status.WorkspaceID != "",
		OverridesPublished:  ext.OverridesPublished,
		OriginPath:          originPath,
		RemoteVersion:       dereferenceOptionalString(ext.Info.RemoteVersion),
		DigestMatched:       ext.Info.Provenance.DigestMatched,
		DaemonRunning:       daemonRunning,
		Provenance:          extensionProvenancePayload(ext.Info.Provenance),
		Trust:               extensionTrustPayload(ext.Info.Provenance),
		Diagnostics:         extensionProjectionDiagnostics(ext.Info),
	}
}

func extensionProjectionDiagnostics(info ExtensionInfo) []contract.DiagnosticItem {
	diagnostics := make([]contract.DiagnosticItem, 0, len(info.IngestDiagnostics)+len(info.Provenance.Warnings))
	diagnostics = append(diagnostics, info.IngestDiagnostics...)
	diagnostics = append(diagnostics, info.Provenance.Warnings...)
	return diagnostics
}

func extensionType(manifest *Manifest, info ExtensionInfo) string {
	if requiresSubprocess(manifest) || len(info.Capabilities.Provides) > 0 || len(info.Permissions.Requires) > 0 {
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
			(!requiresSubprocess(manifest) && len(info.Capabilities.Provides) == 0 && len(info.Permissions.Requires) == 0) {
			return extensionHealthHealthy
		}
		return extensionHealthUnhealthy
	}
	if status.LastError != "" {
		return extensionHealthUnhealthy
	}
	if !requiresSubprocess(manifest) && len(info.Capabilities.Provides) == 0 && len(info.Permissions.Requires) == 0 &&
		status.Registered {
		return extensionHealthHealthy
	}
	return extensionHealthUnknown
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
		DigestMatched:       value.DigestMatched,
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
