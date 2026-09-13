package marketplace

import (
	"encoding/json"
	"fmt"
	"strings"
)

func validateDocumentEntries(entries []Entry) error {
	if len(entries) > maxCatalogEntriesPerSource {
		return fmt.Errorf("marketplace catalog entries exceeds limit %d", maxCatalogEntriesPerSource)
	}
	seenIDs := make(map[string]struct{}, len(entries))
	seenInstallSlugs := make(map[string]struct{}, len(entries))
	for index, entry := range entries {
		entryID := strings.TrimSpace(entry.EntryID)
		if entryID == "" || strings.TrimSpace(entry.Name) == "" || strings.TrimSpace(entry.Description) == "" {
			return fmt.Errorf("marketplace catalog entry %d identity fields are required", index)
		}

		if _, exists := seenIDs[entryID]; exists {
			return fmt.Errorf("marketplace catalog entry_id %q is duplicated", entryID)
		}
		seenIDs[entryID] = struct{}{}

		installSlug := strings.TrimSpace(entry.InstallSlug)
		if installSlug == "" {
			return fmt.Errorf("marketplace catalog entry %q install_slug is required", entryID)
		}
		if _, exists := seenInstallSlugs[installSlug]; exists {
			return fmt.Errorf("marketplace catalog install_slug %q is duplicated", installSlug)
		}
		seenInstallSlugs[installSlug] = struct{}{}

		if len(entry.Payload) == 0 || !json.Valid(entry.Payload) {
			return fmt.Errorf("marketplace catalog entry %q payload_json is invalid", entryID)
		}
	}
	return nil
}
