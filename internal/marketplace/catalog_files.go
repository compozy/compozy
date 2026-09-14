package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
)

// ValidateCatalogDirectory validates the complete publishable catalog using runtime schemas.
func ValidateCatalogDirectory(directory string) error {
	return validateV3CatalogFamily(filepath.Join(directory, "v3"))
}

func validateV3CatalogFamily(root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "extensions.json"))
	if err != nil {
		return fmt.Errorf("read v3 extensions: %w", err)
	}
	document, err := DecodeDocument(raw)
	if err != nil {
		return err
	}
	if len(document.Entries) == 0 {
		return fmt.Errorf("v3/extensions.json requires a non-empty version 3 catalog")
	}
	for _, entry := range document.Entries {
		if len(entry.Diagnostics) > 0 {
			return fmt.Errorf("catalog entry %q: %s", entry.EntryID, entry.Diagnostics[0].Message)
		}
	}
	presets, err := os.ReadFile(filepath.Join(root, "marketplaces.json"))
	if err != nil {
		return fmt.Errorf("read v3 presets: %w", err)
	}
	_, err = DecodePresets(presets)
	return err
}
