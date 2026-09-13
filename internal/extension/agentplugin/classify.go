package agentplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

// ClassifyManifest locates the authored manifest and triages its declared schema.
func ClassifyManifest(dir string) (SchemaStatus, string, error) {
	path, layout, err := LocateManifest(dir)
	if err != nil {
		if _, ok := errors.AsType[*NotManifestError](err); ok {
			return SchemaUnrelated, "", nil
		}
		if errors.Is(err, fileutil.ErrSymlink) || errors.Is(err, fileutil.ErrDirectory) ||
			errors.Is(err, fileutil.ErrNotRegular) {
			return SchemaUnrelated, "", nil
		}
		return SchemaUnrelated, "", err
	}
	content, _, err := fileutil.ReadRegularFile(path)
	if err != nil {
		return SchemaUnrelated, "", fmt.Errorf("read Agent Plugins manifest %q: %w", path, err)
	}
	if layout != LayoutStandard {
		var fields map[string]json.RawMessage
		if json.Unmarshal(content, &fields) == nil && fields != nil {
			if _, declared := fields[fieldSchema]; !declared {
				return SchemaSupported, "", nil
			}
		}
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
	const message = "must be 1-64 lowercase letters, digits, dots, or hyphens, " +
		"start and end alphanumeric, without \"--\" or \"..\""
	if name == "" || len(name) > 64 {
		return errors.New(message)
	}
	if strings.Contains(name, "--") || strings.Contains(name, "..") {
		return errors.New(message)
	}
	for _, character := range []byte(name) {
		if isASCIILower(character) || isASCIIDigit(character) || character == '-' || character == '.' {
			continue
		}
		return errors.New(message)
	}
	if !isASCIIAlphanumeric(name[0]) || !isASCIIAlphanumeric(name[len(name)-1]) {
		return errors.New(message)
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
