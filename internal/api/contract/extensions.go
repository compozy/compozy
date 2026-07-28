package contract

import "time"

// InstallExtensionRequest is the shared extension install request payload.
type InstallExtensionRequest struct {
	Path            string `json:"path,omitempty"`
	Checksum        string `json:"checksum,omitempty"`
	Slug            string `json:"slug,omitempty"`
	Version         string `json:"version,omitempty"`
	Source          string `json:"source,omitempty"`
	Asset           string `json:"asset,omitempty"`
	AllowUnverified bool   `json:"allow_unverified,omitempty"`
}

// UpdateExtensionRequest is the shared marketplace extension update payload.
type UpdateExtensionRequest struct {
	Version         string `json:"version,omitempty"`
	CheckOnly       bool   `json:"check_only,omitempty"`
	AllowUnverified bool   `json:"allow_unverified,omitempty"`
}

// ExtensionTrustReportPayload records the trust decision for an extension.
type ExtensionTrustReportPayload struct {
	Decision         string           `json:"decision"`
	RegistryTier     string           `json:"registry_tier"`
	ChecksumVerified bool             `json:"checksum_verified"`
	AllowUnverified  bool             `json:"allow_unverified"`
	Warnings         []DiagnosticItem `json:"warnings,omitempty"`
}

// ExtensionProvenancePayload contains the persisted source and trust record.
type ExtensionProvenancePayload struct {
	Slug                string                       `json:"slug,omitempty"`
	CatalogEntryID      string                       `json:"catalog_entry_id,omitempty"`
	InstalledFrom       string                       `json:"installed_from"`
	SourceURL           string                       `json:"source_url,omitempty"`
	ChecksumSHA256      string                       `json:"checksum_sha256"`
	ArchiveDigestSHA256 string                       `json:"archive_digest_sha256,omitempty"`
	ChecksumVerified    bool                         `json:"checksum_verified"`
	RegistryTier        string                       `json:"registry_tier"`
	Permissions         []string                     `json:"permissions,omitempty"`
	InstalledAt         time.Time                    `json:"installed_at"`
	InstalledBy         string                       `json:"installed_by"`
	AllowUnverified     bool                         `json:"allow_unverified"`
	Warnings            []DiagnosticItem             `json:"warnings,omitempty"`
	Trust               *ExtensionTrustReportPayload `json:"trust,omitempty"`
}

// ExtensionPayload is the shared extension response payload surfaced by CLI APIs.
type ExtensionPayload struct {
	Name          string                          `json:"name"`
	Version       string                          `json:"version"`
	Type          string                          `json:"type"`
	Source        string                          `json:"source"`
	Enabled       bool                            `json:"enabled"`
	State         string                          `json:"state"`
	Capabilities  []string                        `json:"capabilities,omitempty"`
	Actions       []string                        `json:"actions,omitempty"`
	RequiresEnv   []string                        `json:"requires_env,omitempty"`
	MissingEnv    []string                        `json:"missing_env,omitempty"`
	PID           int                             `json:"pid,omitempty"`
	UptimeSeconds int64                           `json:"uptime_seconds,omitempty"`
	Health        string                          `json:"health,omitempty"`
	HealthMessage string                          `json:"health_message,omitempty"`
	LastError     string                          `json:"last_error,omitempty"`
	DaemonRunning bool                            `json:"daemon_running"`
	Bundles       []ExtensionBundleSummaryPayload `json:"bundles,omitempty"`
	Provenance    *ExtensionProvenancePayload     `json:"provenance,omitempty"`
	Marketplace   *MarketplaceListingPayload      `json:"marketplace,omitempty"`
	Trust         *ExtensionTrustReportPayload    `json:"trust,omitempty"`
	Diagnostics   []DiagnosticItem                `json:"diagnostics,omitempty"`
}

// ExtensionBundleSummaryPayload describes an installed bundle exposed with extension status.
type ExtensionBundleSummaryPayload struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Profiles    []string `json:"profiles,omitempty"`
}
