package marketplace

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var sha256Pattern = regexp.MustCompile(`^[a-fA-F0-9]{64}$`)

const (
	extensionTierOfficial   = "official"
	extensionTierCommunity  = "community"
	extensionTierUnverified = "unverified"
)

// Display-only format markers. Install-time detection stays authoritative — an entry may omit the
// marker and still resolve to a portable package.
const (
	ExtensionFormatCompozy     = "compozy"
	ExtensionFormatAgentPlugin = "agent-plugin"
)

type extensionEntry struct {
	entryCommon
	InstallSlug  string `json:"install_slug"`
	ArtifactURL  string `json:"artifact_url"`
	DigestSHA256 string `json:"digest_sha256"`
	Tier         string `json:"tier,omitempty"`
	Author       string `json:"author,omitempty"`
	Repository   string `json:"repository,omitempty"`
	Format       string `json:"format,omitempty"`
}

func decodeExtensionEntry(raw []byte) (Entry, error) {
	var value extensionEntry
	if err := decodeStrict(raw, &value); err != nil {
		return Entry{}, err
	}
	if err := value.validate(KindExtension); err != nil {
		return Entry{}, err
	}
	if strings.TrimSpace(value.Version) == "" {
		return Entry{}, fmt.Errorf("marketplace catalog extension entry %q version is required", value.EntryID)
	}
	if strings.TrimSpace(value.InstallSlug) == "" {
		return Entry{}, fmt.Errorf("marketplace catalog extension entry %q install_slug is required", value.EntryID)
	}
	if err := validateExtensionArtifactURL(value.EntryID, value.ArtifactURL); err != nil {
		return Entry{}, err
	}
	if strings.TrimSpace(value.DigestSHA256) == "" {
		return Entry{}, fmt.Errorf("marketplace catalog extension entry %q digest_sha256 is required", value.EntryID)
	}
	if !sha256Pattern.MatchString(strings.TrimSpace(value.DigestSHA256)) {
		return Entry{}, fmt.Errorf(
			"marketplace catalog extension entry %q digest_sha256 must be 64 hexadecimal characters",
			value.EntryID,
		)
	}
	tier := strings.ToLower(strings.TrimSpace(value.Tier))
	if tier == "" {
		return Entry{}, fmt.Errorf("marketplace catalog extension entry %q tier is required", value.EntryID)
	}
	switch tier {
	case extensionTierOfficial, extensionTierCommunity, extensionTierUnverified:
	default:
		return Entry{}, fmt.Errorf(
			"marketplace catalog extension entry %q has unsupported tier %q",
			value.EntryID,
			tier,
		)
	}
	format, err := normalizeExtensionFormat(value.EntryID, value.Format)
	if err != nil {
		return Entry{}, err
	}
	value.Format = format
	entry, err := commonEntry(KindExtension, value.entryCommon, value)
	if err != nil {
		return Entry{}, err
	}
	entry.InstallSlug = strings.TrimSpace(value.InstallSlug)
	entry.DigestSHA256 = strings.ToLower(strings.TrimSpace(value.DigestSHA256))
	entry.Tier = tier
	return entry, nil
}

// normalizeExtensionFormat resolves the optional display marker to the closed format set. An absent
// marker means the native Compozy format — the same default the daemon persists for an unmarked row.
func normalizeExtensionFormat(entryID string, raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	switch trimmed {
	case "":
		return ExtensionFormatCompozy, nil
	case ExtensionFormatCompozy, ExtensionFormatAgentPlugin:
		return trimmed, nil
	default:
		return "", fmt.Errorf(
			"marketplace catalog extension entry %q has unsupported format %q",
			strings.TrimSpace(entryID),
			trimmed,
		)
	}
}

func validateExtensionArtifactURL(entryID string, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("marketplace catalog extension entry %q artifact_url is required", entryID)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || parsed.User != nil || !isAllowedExtensionArtifactURL(parsed) {
		return fmt.Errorf(
			"marketplace catalog extension entry %q artifact_url must be an absolute HTTPS URL "+
				"without credentials; HTTP is allowed only for loopback hosts",
			entryID,
		)
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("marketplace catalog extension entry %q artifact_url must not contain a fragment", entryID)
	}
	return nil
}

func isAllowedExtensionArtifactURL(parsed *url.URL) bool {
	if parsed.Scheme == protocolHTTPS {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}
