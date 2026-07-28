package config

import (
	"bytes"

	"encoding/json"

	"fmt"

	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

func loadCapabilityCatalogDirectory(dir string) (*CapabilityCatalog, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("config: read capability catalog directory %q: %w", dir, err)
	}

	tomlFiles := make([]string, 0)
	jsonFiles := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("config: read capability catalog entry %q: %w", filepath.Join(dir, name), err)
		}
		if !info.Mode().IsRegular() {
			continue
		}

		path := filepath.Join(dir, name)
		switch filepath.Ext(name) {
		case capabilityFileExtTOML:
			tomlFiles = append(tomlFiles, path)
		case capabilityFileExtJSON:
			jsonFiles = append(jsonFiles, path)
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

	records := make([]capabilityCatalogRecord, 0, len(selected))
	for _, path := range selected {
		capability, err := loadCapabilityDefFile(path)
		if err != nil {
			return nil, err
		}
		records = append(records, capabilityCatalogRecord{
			source:     path,
			basename:   strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
			capability: capability,
		})
	}

	return normalizeCapabilityCatalogRecords(records, dir)
}

func loadCapabilityDefFile(path string) (CapabilityDef, error) {
	content, exists, err := readOptionalRegularFile(path, "capability definition")
	if err != nil {
		return CapabilityDef{}, err
	}
	if !exists {
		return CapabilityDef{}, fmt.Errorf("config: capability definition %q disappeared before read", path)
	}

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
