package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
)

// ValidateCatalogDirectory validates the complete publishable catalog using runtime schemas.
func ValidateCatalogDirectory(directory string) error {
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
	return nil
}
