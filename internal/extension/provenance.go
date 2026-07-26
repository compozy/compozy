package extensionpkg

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	contract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
)

const (
	ExtensionInstalledFromMarketplace = "marketplace_registry"
	ExtensionInstalledFromLocalPath   = "local_path"
	ExtensionInstalledFromGitURL      = "git_url"

	ExtensionRegistryTierOfficial   = "official"
	ExtensionRegistryTierCommunity  = "community"
	ExtensionRegistryTierUnverified = "unverified"

	ExtensionTrustDecisionVerified          = "verified"
	ExtensionTrustDecisionAllowedUnverified = "allowed_unverified"
	ExtensionTrustDecisionBlocked           = "blocked"

	extensionTrustInstalledByOperator = "operator"
	extensionTrustEvidenceSourceKey   = "source"
	extensionTrustEvidenceVersionKey  = "version"
	extensionTrustGitHubSource        = "github"
)

var ErrExtensionChecksumUnverified = errors.New("extension: checksum is unverified")

// ExtensionProvenance records one installed extension's source and trust state.
type ExtensionProvenance struct {
	Slug                string                    `json:"slug,omitempty"`
	CatalogEntryID      string                    `json:"catalog_entry_id,omitempty"`
	InstalledFrom       string                    `json:"installed_from"`
	SourceURL           string                    `json:"source_url,omitempty"`
	ChecksumSHA256      string                    `json:"checksum_sha256"`
	ArchiveDigestSHA256 string                    `json:"archive_digest_sha256,omitempty"`
	ChecksumVerified    bool                      `json:"checksum_verified"`
	RegistryTier        string                    `json:"registry_tier"`
	Permissions         []string                  `json:"permissions,omitempty"`
	InstalledAt         time.Time                 `json:"installed_at"`
	InstalledBy         string                    `json:"installed_by"`
	AllowUnverified     bool                      `json:"allow_unverified"`
	Warnings            []contract.DiagnosticItem `json:"warnings,omitempty"`
}

// ExtensionTrustError carries the canonical diagnostic for a denied extension
// trust decision.
type ExtensionTrustError struct {
	Slug   string
	Source string
	Item   contract.DiagnosticItem
}

func (e *ExtensionTrustError) Error() string {
	slug := strings.TrimSpace(e.Slug)
	if slug == "" {
		slug = managerExtensionKey
	}
	return fmt.Sprintf("%s: %s", ErrExtensionChecksumUnverified, slug)
}

func (e *ExtensionTrustError) Unwrap() error {
	return ErrExtensionChecksumUnverified
}

func (e *ExtensionTrustError) DiagnosticItem() contract.DiagnosticItem {
	if e == nil {
		return diagnostics.EmptyItem()
	}
	return e.Item
}

func newExtensionChecksumUnverifiedError(slug string, source string) *ExtensionTrustError {
	item := extensionChecksumUnverifiedDiagnostic(slug, source, false)
	return &ExtensionTrustError{Slug: strings.TrimSpace(slug), Source: strings.TrimSpace(source), Item: item}
}

// NewExtensionChecksumUnverifiedError returns the canonical trust-gate error.
func NewExtensionChecksumUnverifiedError(slug string, source string) *ExtensionTrustError {
	return newExtensionChecksumUnverifiedError(slug, source)
}

// LocalPathProvenance records an explicit trust decision for a local install.
func LocalPathProvenance(
	manifest *Manifest,
	sourcePath string,
	checksum string,
	installedAt time.Time,
	allowUnverified bool,
) ExtensionProvenance {
	provenance := ExtensionProvenance{
		InstalledFrom:    ExtensionInstalledFromLocalPath,
		SourceURL:        strings.TrimSpace(sourcePath),
		ChecksumSHA256:   strings.TrimSpace(checksum),
		ChecksumVerified: false,
		RegistryTier:     ExtensionRegistryTierUnverified,
		InstalledAt:      installedAt.UTC(),
		InstalledBy:      extensionTrustInstalledByOperator,
		AllowUnverified:  allowUnverified,
	}
	if manifest != nil {
		provenance.Permissions = extensionPermissions(manifest)
		provenance.Warnings = []contract.DiagnosticItem{
			extensionChecksumUnverifiedDiagnostic(manifest.Name, sourcePath, allowUnverified),
		}
	}
	return provenance
}

func extensionChecksumUnverifiedDiagnostic(
	slug string,
	source string,
	allowed bool,
) contract.DiagnosticItem {
	title := "Extension checksum is unverified"
	message := "The marketplace did not provide a verifiable checksum for this extension."
	if allowed {
		message = "The extension was installed after an explicit allow_unverified trust decision."
	}
	return diagnostics.NewItem(
		"extension.checksum_unverified",
		contract.CodeExtensionChecksumUnverified,
		contract.CategoryExtension,
		title,
		message,
		contract.SeverityWarn,
		contract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"slug":                          strings.TrimSpace(slug),
			extensionTrustEvidenceSourceKey: strings.TrimSpace(source),
			"allow_unverified":              allowed,
		}),
	)
}

func normalizeExtensionProvenance(value ExtensionProvenance, fallback ExtensionProvenance) ExtensionProvenance {
	if strings.TrimSpace(value.InstalledFrom) == "" {
		value.InstalledFrom = fallback.InstalledFrom
	}
	if strings.TrimSpace(value.SourceURL) == "" {
		value.SourceURL = fallback.SourceURL
	}
	if strings.TrimSpace(value.CatalogEntryID) == "" {
		value.CatalogEntryID = fallback.CatalogEntryID
	}
	if strings.TrimSpace(value.ChecksumSHA256) == "" {
		value.ChecksumSHA256 = fallback.ChecksumSHA256
	}
	if strings.TrimSpace(value.ArchiveDigestSHA256) == "" {
		value.ArchiveDigestSHA256 = fallback.ArchiveDigestSHA256
	}
	if strings.TrimSpace(value.RegistryTier) == "" {
		value.RegistryTier = fallback.RegistryTier
	}
	if strings.TrimSpace(value.InstalledBy) == "" {
		value.InstalledBy = fallback.InstalledBy
	}
	if value.InstalledAt.IsZero() {
		value.InstalledAt = fallback.InstalledAt
	}
	if len(value.Permissions) == 0 {
		value.Permissions = append([]string(nil), fallback.Permissions...)
	}
	if len(value.Warnings) > 0 {
		value.Warnings = append([]contract.DiagnosticItem(nil), value.Warnings...)
	}
	value.Slug = strings.TrimSpace(value.Slug)
	value.CatalogEntryID = strings.TrimSpace(value.CatalogEntryID)
	value.InstalledFrom = strings.TrimSpace(value.InstalledFrom)
	value.SourceURL = strings.TrimSpace(value.SourceURL)
	value.ChecksumSHA256 = strings.TrimSpace(value.ChecksumSHA256)
	value.ArchiveDigestSHA256 = strings.TrimSpace(value.ArchiveDigestSHA256)
	value.RegistryTier = strings.TrimSpace(value.RegistryTier)
	value.InstalledBy = strings.TrimSpace(value.InstalledBy)
	return value
}

func extensionPermissions(manifest *Manifest) []string {
	if manifest == nil {
		return nil
	}
	items := permissionSet{}
	items.addValues("capabilities.provides", manifest.Capabilities.Provides)
	items.addValues("security.capability", manifest.Security.Capabilities)
	items.addValues("actions.requires", manifest.Actions.Requires)
	items.addValues("requires_env", manifest.RequiresEnv)
	if len(manifest.Resources.Publish.Families) > 0 {
		items.addValues("resources.publish.family", manifest.Resources.Publish.Families)
	}
	if strings.TrimSpace(string(manifest.Resources.Publish.MaxScope)) != "" {
		items.add("resources.publish.max_scope", string(manifest.Resources.Publish.MaxScope))
	}
	for name := range manifest.Resources.MCPServers {
		items.add("resources.mcp_server", name)
	}
	for name := range manifest.Resources.Tools {
		tool := manifest.Resources.Tools[name]
		toolName := strings.TrimSpace(name)
		if strings.TrimSpace(tool.Backend.Kind) != "" {
			items.add("tool.backend", toolName+":"+strings.TrimSpace(tool.Backend.Kind))
		}
		items.addValues("tool.required_capability:"+toolName, tool.RequiredCapabilities)
		items.addValues("tool.requires_env:"+toolName, tool.RequiresEnv)
	}
	if strings.TrimSpace(manifest.Subprocess.Command) != "" {
		items.add("subprocess.command", manifest.Subprocess.Command)
	}
	items.addMapKeys("subprocess.env", manifest.Subprocess.Env)
	items.addMapKeys("subprocess.secret_env", manifest.Subprocess.SecretEnv)
	for i := range manifest.Resources.Hooks {
		hook := &manifest.Resources.Hooks[i]
		hookName := strings.TrimSpace(hook.Name)
		items.addMapKeys("hook.env:"+hookName, hook.Env)
		items.addMapKeys("hook.secret_env:"+hookName, hook.SecretEnv)
		items.addMapKeys("hook.executor.env:"+hookName, hook.Executor.Env)
		items.addMapKeys("hook.executor.secret_env:"+hookName, hook.Executor.SecretEnv)
	}
	for _, slot := range manifest.Bridge.SecretSlots {
		items.add("bridge.secret_slot", slot.Name)
	}
	return items.sorted()
}

func extensionPermissionsFromParts(capabilities CapabilitiesConfig, actions ActionsConfig) []string {
	items := permissionSet{}
	items.addValues("capabilities.provides", capabilities.Provides)
	items.addValues("actions.requires", actions.Requires)
	return items.sorted()
}

type permissionSet map[string]struct{}

func (s permissionSet) add(prefix string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	s[prefix+":"+trimmed] = struct{}{}
}

func (s permissionSet) addValues(prefix string, values []string) {
	for _, value := range values {
		s.add(prefix, value)
	}
}

func (s permissionSet) addMapKeys(prefix string, values map[string]string) {
	for key := range values {
		s.add(prefix, key)
	}
}

func (s permissionSet) sorted() []string {
	if len(s) == 0 {
		return nil
	}
	items := make([]string, 0, len(s))
	for item := range s {
		items = append(items, item)
	}
	slices.Sort(items)
	return items
}

func installedFromForSource(source ExtensionSource) string {
	switch source {
	case SourceMarketplace:
		return ExtensionInstalledFromMarketplace
	case SourceUser, SourceWorkspace, SourceBundled:
		return ExtensionInstalledFromLocalPath
	default:
		return ExtensionInstalledFromLocalPath
	}
}

func registryTierForSource(source ExtensionSource, registryName string) string {
	if source != SourceMarketplace {
		return ExtensionRegistryTierUnverified
	}
	switch strings.ToLower(strings.TrimSpace(registryName)) {
	case extensionTrustGitHubSource:
		return ExtensionRegistryTierCommunity
	case "":
		return ExtensionRegistryTierUnverified
	default:
		return ExtensionRegistryTierCommunity
	}
}

func extensionTrustDecision(provenance ExtensionProvenance) string {
	if strings.EqualFold(strings.TrimSpace(provenance.RegistryTier), ExtensionRegistryTierUnverified) {
		if provenance.AllowUnverified {
			return ExtensionTrustDecisionAllowedUnverified
		}
		return ExtensionTrustDecisionBlocked
	}
	if provenance.ChecksumVerified {
		return ExtensionTrustDecisionVerified
	}
	if provenance.AllowUnverified {
		return ExtensionTrustDecisionAllowedUnverified
	}
	return ExtensionTrustDecisionBlocked
}
