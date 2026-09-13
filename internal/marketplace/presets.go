package marketplace

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Preset is one ordered built-in plugin marketplace registration.
type Preset struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Default     string `json:"default"`
}

// PresetDocument is the v3 source catalog, separate from extension entries.
type PresetDocument struct {
	ManifestVersion int      `json:"manifest_version"`
	GeneratedAt     string   `json:"generated_at"`
	Entries         []Preset `json:"entries"`
}

var presetNamePattern = regexp.MustCompile(`^[a-z0-9._-]{1,64}$`)

// DecodePresets validates publication metadata while preserving registration order.
func DecodePresets(raw []byte) (*PresetDocument, error) {
	var document PresetDocument
	if err := decodeStrict(raw, &document); err != nil {
		return nil, err
	}
	if document.ManifestVersion != ManifestVersion {
		return nil, fmt.Errorf("marketplace presets require manifest_version 3")
	}
	if _, err := time.Parse(time.RFC3339, document.GeneratedAt); err != nil {
		return nil, fmt.Errorf("marketplace presets generated_at: %w", err)
	}
	if document.Entries == nil {
		return nil, fmt.Errorf("marketplace presets entries is required")
	}
	seen := make(map[string]bool, len(document.Entries))
	for index, entry := range document.Entries {
		if !presetNamePattern.MatchString(entry.Name) || entry.Name == "compozy" || entry.Name == CompozyCatalogSource {
			return nil, fmt.Errorf("marketplace presets entries[%d].name is invalid or reserved", index)
		}
		if seen[entry.Name] {
			return nil, fmt.Errorf("marketplace preset %q is duplicated", entry.Name)
		}
		seen[entry.Name] = true
		if strings.TrimSpace(entry.Source) == "" || strings.TrimSpace(entry.Description) == "" {
			return nil, fmt.Errorf("marketplace preset %q source and description are required", entry.Name)
		}
		if entry.Default != "on" && entry.Default != "off" {
			return nil, fmt.Errorf("marketplace preset %q default must be on or off", entry.Name)
		}
	}
	return &document, nil
}
