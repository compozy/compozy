package marketplace

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

const CatalogSource = "compozy-catalog"

// EntryDetails is the typed public projection of one validated catalog payload.
type EntryDetails struct {
	Author    string
	Source    string
	SourceRef string
	Extension *ExtensionEntryDetails
}

type ExtensionEntryDetails struct {
	InstanceName string
	Inputs       []EntryInput
	InstallSlug  string
	ArtifactURL  string
	DigestSHA256 string
	Repository   string
	Homepage     string
	License      string
	Category     string
	Keywords     []string
	Acquisition  *pluginsource.AcquisitionRecord
	Contents     PluginContents
	// Format is the curated display marker, never install policy: detection at install time decides
	// what a package actually is.
	Format string
}

// ProjectEntry decodes a validated extension row without duplicating feed schemas downstream.
func ProjectEntry(entry Entry) (EntryDetails, error) {
	var value pluginEntry
	if err := json.Unmarshal(entry.Payload, &value); err != nil {
		return EntryDetails{}, fmt.Errorf("marketplace: decode extension entry %q: %w", entry.EntryID, err)
	}
	format, err := normalizeExtensionFormat(entry.EntryID, value.Format)
	if err != nil {
		return EntryDetails{}, fmt.Errorf("marketplace: project extension entry %q: %w", entry.EntryID, err)
	}
	source, sourceRef := entry.SourceName, value.SourceRef
	if source == "" {
		source = CatalogSource
	}
	if sourceRef == "" {
		sourceRef = CompozyCatalogRef
	}
	return EntryDetails{
		Author: strings.TrimSpace(value.Author), Source: source, SourceRef: sourceRef,
		Extension: &ExtensionEntryDetails{
			InstanceName: value.InstanceName,
			Inputs:       value.Inputs,
			InstallSlug:  strings.TrimSpace(value.InstallSlug),
			ArtifactURL:  strings.TrimSpace(value.ArtifactURL),
			DigestSHA256: strings.ToLower(strings.TrimSpace(value.DigestSHA256)),
			Repository:   strings.TrimSpace(value.Repository),
			Format:       format,
			Homepage:     value.Homepage, License: value.License, Category: value.Category,
			Keywords: value.Keywords, Acquisition: value.Acquisition, Contents: value.Contents,
		},
	}, nil
}
