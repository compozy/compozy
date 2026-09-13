package marketplace

import (
	"encoding/json"
	"fmt"
	"strings"
)

const CatalogSource = "compozy-catalog"

// EntryDetails is the typed public projection of one validated catalog payload.
type EntryDetails struct {
	Author    string
	Source    string
	Extension *ExtensionEntryDetails
}

type ExtensionEntryDetails struct {
	Inputs       []EntryInput
	InstallSlug  string
	ArtifactURL  string
	DigestSHA256 string
	Repository   string
	// Format is the curated display marker, never install policy: detection at install time decides
	// what a package actually is.
	Format string
}

// ProjectEntry decodes a validated extension row without duplicating feed schemas downstream.
func ProjectEntry(entry Entry) (EntryDetails, error) {
	var value extensionEntry
	if err := json.Unmarshal(entry.Payload, &value); err != nil {
		return EntryDetails{}, fmt.Errorf("marketplace: decode extension entry %q: %w", entry.EntryID, err)
	}
	format, err := normalizeExtensionFormat(entry.EntryID, value.Format)
	if err != nil {
		return EntryDetails{}, fmt.Errorf("marketplace: project extension entry %q: %w", entry.EntryID, err)
	}
	return EntryDetails{
		Author: strings.TrimSpace(value.Author), Source: CatalogSource,
		Extension: &ExtensionEntryDetails{
			Inputs:       value.Inputs,
			InstallSlug:  strings.TrimSpace(value.InstallSlug),
			ArtifactURL:  strings.TrimSpace(value.ArtifactURL),
			DigestSHA256: strings.ToLower(strings.TrimSpace(value.DigestSHA256)),
			Repository:   strings.TrimSpace(value.Repository),
			Format:       format,
		},
	}, nil
}
