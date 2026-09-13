package marketplace

import (
	"encoding/json"
	"fmt"
	"strings"
)

func validateDocumentEntries(kind Kind, entries []Entry) error {
	if len(entries) > maxCatalogEntriesPerKind {
		return fmt.Errorf("marketplace catalog %q entries exceeds limit %d", kind, maxCatalogEntriesPerKind)
	}
	seenIDs := make(map[string]struct{}, len(entries))
	seenInstallSlugs := make(map[string]struct{}, len(entries))
	for index, entry := range entries {
		if entry.Kind != kind {
			return fmt.Errorf("marketplace catalog %q entry %d kind = %q", kind, index, entry.Kind)
		}
		entryID := strings.TrimSpace(entry.EntryID)
		if entryID == "" || strings.TrimSpace(entry.Name) == "" || strings.TrimSpace(entry.Description) == "" {
			return fmt.Errorf("marketplace catalog %q entry %d identity fields are required", kind, index)
		}
		if kind == KindSkill && strings.HasPrefix(entryID, RemoteSkillEntryPrefix) {
			return fmt.Errorf(
				"marketplace catalog %q entry_id %q uses reserved prefix %q",
				kind,
				entryID,
				RemoteSkillEntryPrefix,
			)
		}
		if _, exists := seenIDs[entryID]; exists {
			return fmt.Errorf("marketplace catalog %q entry_id %q is duplicated", kind, entryID)
		}
		seenIDs[entryID] = struct{}{}
		if kind == KindExtension || kind == KindSkill {
			installSlug := strings.TrimSpace(entry.InstallSlug)
			if installSlug == "" {
				return fmt.Errorf("marketplace catalog %q entry %q install_slug is required", kind, entryID)
			}
			if _, exists := seenInstallSlugs[installSlug]; exists {
				return fmt.Errorf("marketplace catalog %q install_slug %q is duplicated", kind, installSlug)
			}
			seenInstallSlugs[installSlug] = struct{}{}
		}
		if len(entry.Payload) == 0 || !json.Valid(entry.Payload) {
			return fmt.Errorf("marketplace catalog %q entry %q payload_json is invalid", kind, entryID)
		}
	}
	return nil
}
