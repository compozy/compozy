package marketplace

import (
	"fmt"
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

type extensionEntry struct {
	entryCommon
	InstallSlug  string `json:"install_slug"`
	ArtifactURL  string `json:"artifact_url"`
	DigestSHA256 string `json:"digest_sha256"`
	Tier         string `json:"tier,omitempty"`
	Author       string `json:"author,omitempty"`
	Repository   string `json:"repository,omitempty"`
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
	entry, err := commonEntry(KindExtension, value.entryCommon, value)
	if err != nil {
		return Entry{}, err
	}
	entry.InstallSlug = strings.TrimSpace(value.InstallSlug)
	entry.DigestSHA256 = strings.ToLower(strings.TrimSpace(value.DigestSHA256))
	entry.Tier = tier
	return entry, nil
}

func validateExtensionArtifactURL(entryID string, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("marketplace catalog extension entry %q artifact_url is required", entryID)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme != protocolHTTPS || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf(
			"marketplace catalog extension entry %q artifact_url must be an absolute HTTPS URL without credentials",
			entryID,
		)
	}
	if parsed.Fragment != "" {
		return fmt.Errorf("marketplace catalog extension entry %q artifact_url must not contain a fragment", entryID)
	}
	return nil
}
