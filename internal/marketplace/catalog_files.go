package marketplace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ValidateCatalogDirectory validates the complete publishable catalog using runtime schemas.
func ValidateCatalogDirectory(directory string) error {
	raw, err := os.ReadFile(filepath.Join(directory, "extensions.json"))
	if err != nil {
		return err
	}
	root, err := DecodeDocument(KindExtension, raw)
	if err != nil {
		return err
	}
	if root.ManifestVersion == ManifestVersionV3 {
		return validateV3CatalogFamily(directory)
	}

	for _, kind := range AllKinds() {
		filename, err := kindFilename(kind)
		if err != nil {
			return err
		}
		path := filepath.Join(directory, filename)
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("marketplace catalog: read %q: %w", path, err)
		}
		document, err := DecodeDocument(kind, raw)
		if err != nil {
			return fmt.Errorf("marketplace catalog: validate %q: %w", path, err)
		}
		if len(document.Entries) == 0 {
			return fmt.Errorf("marketplace catalog: validate %q: at least one curated entry is required", path)
		}
	}
	return validateV3CatalogDirectory(directory)
}

func validateV3CatalogDirectory(directory string) error {
	root := filepath.Join(directory, "v3")
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return validateV3CatalogFamily(root)
}

func validateV3CatalogFamily(root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "extensions.json"))
	if err != nil {
		return fmt.Errorf("read v3 extensions: %w", err)
	}
	document, err := DecodeDocument(KindExtension, raw)
	if err != nil {
		return err
	}
	if document.ManifestVersion != ManifestVersionV3 || len(document.Entries) == 0 {
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
