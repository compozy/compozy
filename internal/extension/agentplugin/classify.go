package agentplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ClassifyManifest reads a regular, non-symlinked root plugin.json and triages
// its declared schema identifier.
func ClassifyManifest(dir string) (SchemaStatus, string, error) {
	path := filepath.Join(dir, "plugin.json")
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SchemaUnrelated, "", nil
		}
		return SchemaUnrelated, "", fmt.Errorf("inspect Agent Plugins manifest %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return SchemaUnrelated, "", nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return SchemaUnrelated, "", fmt.Errorf("read Agent Plugins manifest %q: %w", path, err)
	}
	status, declared := ClassifyManifestContent(content)
	return status, declared, nil
}

// ClassifyManifestContent applies the same schema triage to manifest bytes
// already acquired through a caller-owned filesystem capability.
func ClassifyManifestContent(content []byte) (SchemaStatus, string) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(content, &root); err != nil {
		return SchemaUnrelated, ""
	}
	var declared string
	if err := json.Unmarshal(root[fieldSchema], &declared); err != nil {
		return SchemaUnrelated, ""
	}
	switch {
	case declared == PluginSchemaID:
		return SchemaSupported, declared
	case strings.HasPrefix(declared, schemaPrefix):
		return SchemaUnsupportedVersion, declared
	default:
		return SchemaUnrelated, declared
	}
}

// ValidateName enforces the portable manifest name grammar byte-for-byte.
func ValidateName(name string) error {
	if name == "" || len(name) > 64 {
		return errors.New("name length must be between 1 and 64 bytes")
	}
	if strings.Contains(name, "--") || strings.Contains(name, "..") {
		return errors.New("name must not contain consecutive hyphens or periods")
	}
	for _, character := range []byte(name) {
		if isASCIILower(character) || isASCIIDigit(character) || character == '-' || character == '.' {
			continue
		}
		return errors.New("name may contain only lowercase ASCII letters, digits, hyphens, and periods")
	}
	if !isASCIIAlphanumeric(name[0]) || !isASCIIAlphanumeric(name[len(name)-1]) {
		return errors.New("name must start and end with an ASCII letter or digit")
	}
	return nil
}

func isASCIILower(character byte) bool {
	return character >= 'a' && character <= 'z'
}

func isASCIIDigit(character byte) bool {
	return character >= '0' && character <= '9'
}

func isASCIIAlphanumeric(character byte) bool {
	return isASCIILower(character) || isASCIIDigit(character)
}
