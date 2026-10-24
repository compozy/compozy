package marketplace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// CompatibilityDocument keeps version requirements separate from the strict released v3 feed.
type CompatibilityDocument struct {
	FeedDigestSHA256 string            `json:"feed_digest_sha256"`
	MinimumVersions  map[string]string `json:"minimum_versions"`
}

// ApplyCompatibility binds requirements to the exact feed and archive bytes before persistence.
func ApplyCompatibility(document *Document, feed, raw []byte) error {
	var compatibility CompatibilityDocument
	if err := decodeStrict(raw, &compatibility); err != nil {
		return err
	}
	digest := sha256.Sum256(feed)
	if compatibility.FeedDigestSHA256 != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("%w: compatibility metadata does not match the extension feed", ErrCatalogValidation)
	}
	for index := range document.Entries {
		entry := &document.Entries[index]
		minimum := compatibility.MinimumVersions[entry.DigestSHA256]
		if minimum == "" {
			return fmt.Errorf("%w: missing compatibility requirement for %q", ErrCatalogValidation, entry.EntryID)
		}
		if _, err := semver.StrictNewVersion(minimum); err != nil {
			return fmt.Errorf("%w: invalid minimum version for %q: %w", ErrCatalogValidation, entry.EntryID, err)
		}
		var payload pluginEntry
		if err := json.Unmarshal(entry.Payload, &payload); err != nil {
			return err
		}
		payload.MinCompozyVersion = minimum
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		entry.Payload = encoded
	}
	return nil
}
