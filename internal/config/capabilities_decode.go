package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/fileutil"
)

func loadCapabilityCatalogDirectoryFromDirectory(
	directory *fileutil.Directory,
	dir string,
) (catalog *CapabilityCatalog, err error) {
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			catalog = nil
			err = errors.Join(err, fmt.Errorf("config: close capability catalog directory %q: %w", dir, closeErr))
		}
	}()

	entries, err := directory.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("config: read capability catalog directory %q: %w", dir, err)
	}

	tomlFiles := make([]string, 0)
	jsonFiles := make([]string, 0)
	contentsByPath := make(map[string][]byte)
	for _, name := range entries {
		if strings.HasPrefix(name, ".") {
			continue
		}

		path := filepath.Join(dir, name)
		switch filepath.Ext(name) {
		case capabilityFileExtTOML:
			content, regular, readErr := readCapabilityCatalogEntry(directory, name, path)
			if readErr != nil {
				return nil, readErr
			}
			if regular {
				tomlFiles = append(tomlFiles, path)
				contentsByPath[path] = content
			}
		case capabilityFileExtJSON:
			content, regular, readErr := readCapabilityCatalogEntry(directory, name, path)
			if readErr != nil {
				return nil, readErr
			}
			if regular {
				jsonFiles = append(jsonFiles, path)
				contentsByPath[path] = content
			}
		}
	}

	sort.Strings(tomlFiles)
	sort.Strings(jsonFiles)

	if len(tomlFiles) > 0 && len(jsonFiles) > 0 {
		conflicts := append(append([]string(nil), tomlFiles...), jsonFiles...)
		return nil, fmt.Errorf(
			"config: validate capability catalog %q: mixed capability file formats: %s",
			dir,
			joinQuotedPaths(conflicts),
		)
	}

	selected := tomlFiles
	if len(selected) == 0 {
		selected = jsonFiles
	}
	if len(selected) == 0 {
		return &CapabilityCatalog{Capabilities: []CapabilityDef{}}, nil
	}

	records, err := loadCapabilityCatalogRecords(selected, contentsByPath)
	if err != nil {
		return nil, err
	}
	return normalizeCapabilityCatalogRecords(records, dir)
}

func loadCapabilityCatalogRecords(
	paths []string,
	contentsByPath map[string][]byte,
) ([]capabilityCatalogRecord, error) {
	records := make([]capabilityCatalogRecord, 0, len(paths))
	for _, path := range paths {
		capability, err := loadCapabilityDefBytes(contentsByPath[path], path)
		if err != nil {
			return nil, err
		}
		records = append(records, capabilityCatalogRecord{
			source:     path,
			basename:   strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			capability: capability,
		})
	}
	return records, nil
}

func readCapabilityCatalogEntry(directory *fileutil.Directory, name string, path string) ([]byte, bool, error) {
	content, _, err := directory.ReadRegularFile(name)
	if errors.Is(err, fileutil.ErrDirectory) || errors.Is(err, fileutil.ErrNotRegular) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("config: read capability catalog entry %q: %w", path, err)
	}
	return content, true, nil
}

func loadCapabilityDefBytes(content []byte, path string) (CapabilityDef, error) {
	switch filepath.Ext(path) {
	case capabilityFileExtTOML:
		return parseCapabilityDefTOML(content, path)
	case capabilityFileExtJSON:
		return parseCapabilityDefJSON(content, path)
	default:
		return CapabilityDef{}, fmt.Errorf("config: unsupported capability definition file %q", path)
	}
}

func parseCapabilityCatalogJSON(content []byte, source string) (*CapabilityCatalog, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()

	var catalog CapabilityCatalog
	if err := decoder.Decode(&catalog); err != nil {
		return nil, fmt.Errorf("config: decode capability JSON %q: %w", source, err)
	}
	if err := ensureJSONDocumentEOF(decoder, source, "capability JSON"); err != nil {
		return nil, err
	}

	return normalizeCapabilityCatalog(&catalog, source)
}

func parseCapabilityCatalogTOML(content []byte, source string) (*CapabilityCatalog, error) {
	var catalog CapabilityCatalog
	if err := decodeStrictCapabilityTOML(content, &catalog); err != nil {
		return nil, fmt.Errorf("config: decode capability TOML %q: %w", source, err)
	}

	return normalizeCapabilityCatalog(&catalog, source)
}

func parseCapabilityDefJSON(content []byte, source string) (CapabilityDef, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()

	var capability CapabilityDef
	if err := decoder.Decode(&capability); err != nil {
		return CapabilityDef{}, fmt.Errorf("config: decode capability JSON %q: %w", source, err)
	}
	if err := ensureJSONDocumentEOF(decoder, source, "capability JSON"); err != nil {
		return CapabilityDef{}, err
	}

	return capability, nil
}

func parseCapabilityDefTOML(content []byte, source string) (CapabilityDef, error) {
	var capability CapabilityDef
	if err := decodeStrictCapabilityTOML(content, &capability); err != nil {
		return CapabilityDef{}, fmt.Errorf("config: decode capability TOML %q: %w", source, err)
	}

	return capability, nil
}

func decodeStrictCapabilityTOML(content []byte, dest any) error {
	meta, err := toml.Decode(string(content), dest)
	if err != nil {
		return err
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return fmt.Errorf("unknown field %q", undecoded[0].String())
	}
	return nil
}
