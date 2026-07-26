package config

import (
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	capabilityCatalogTOMLName = "capabilities.toml"
	capabilityCatalogJSONName = "capabilities.json"
	capabilityCatalogDirName  = "capabilities"
	capabilityFileExtTOML     = ".toml"
	capabilityFileExtJSON     = ".json"
)

// CapabilityDef is one normalized, outcome-oriented capability declaration for an agent.
type CapabilityDef struct {
	ID                string   `json:"id"                     toml:"id"`
	Summary           string   `json:"summary"                toml:"summary"`
	Outcome           string   `json:"outcome"                toml:"outcome"`
	Version           string   `json:"version,omitempty"      toml:"version,omitempty"`
	ContextNeeded     []string `json:"context_needed"         toml:"context_needed"`
	ArtifactsExpected []string `json:"artifacts_expected"     toml:"artifacts_expected"`
	ExecutionOutline  []string `json:"execution_outline"      toml:"execution_outline"`
	Constraints       []string `json:"constraints"            toml:"constraints"`
	Examples          []string `json:"examples"               toml:"examples"`
	Requirements      []string `json:"requirements,omitempty" toml:"requirements,omitempty"`
	Digest            string   `json:"-"                      toml:"-"`
}

// CapabilityCatalog is the normalized local catalog loaded from one agent directory.
type CapabilityCatalog struct {
	Capabilities []CapabilityDef `json:"capabilities" toml:"capabilities"`
}

// CapabilityBrief is the compact discovery projection for one capability.
type CapabilityBrief struct {
	ID      string `json:"id"      toml:"id"`
	Summary string `json:"summary" toml:"summary"`
}

type capabilityCatalogLayoutMode string

const (
	capabilityCatalogLayoutModeNone      capabilityCatalogLayoutMode = ""
	capabilityCatalogLayoutModeFile      capabilityCatalogLayoutMode = "file"
	capabilityCatalogLayoutModeDirectory capabilityCatalogLayoutMode = "directory"
)

type capabilityCatalogLayout struct {
	mode capabilityCatalogLayoutMode
	file string
	dir  string
}

type capabilityCatalogRecord struct {
	source     string
	basename   string
	capability CapabilityDef
}

type canonicalCapabilityDigestPayload struct {
	ID                string   `json:"id"`
	Summary           string   `json:"summary"`
	Outcome           string   `json:"outcome"`
	Version           string   `json:"version,omitempty"`
	ContextNeeded     []string `json:"context_needed,omitempty"`
	ArtifactsExpected []string `json:"artifacts_expected,omitempty"`
	ExecutionOutline  []string `json:"execution_outline,omitempty"`
	Constraints       []string `json:"constraints,omitempty"`
	Examples          []string `json:"examples,omitempty"`
	Requirements      []string `json:"requirements,omitempty"`
}

// Clone returns a deep copy of the catalog.
func (c *CapabilityCatalog) Clone() *CapabilityCatalog {
	if c == nil {
		return nil
	}

	cloned := &CapabilityCatalog{
		Capabilities: make([]CapabilityDef, 0, len(c.Capabilities)),
	}
	for _, capability := range c.Capabilities {
		cloned.Capabilities = append(cloned.Capabilities, cloneCapabilityDef(capability))
	}

	return cloned
}

// LoadAgentCapabilities loads the optional capability catalog for one agent directory.
// When no supported capability catalog exists, it returns nil without error.
func LoadAgentCapabilities(agentDir string) (*CapabilityCatalog, error) {
	trimmedDir := strings.TrimSpace(agentDir)
	if trimmedDir == "" {
		return nil, errors.New("config: agent directory is required")
	}

	layout, err := detectCapabilityCatalogLayout(trimmedDir)
	if err != nil {
		return nil, err
	}

	switch layout.mode {
	case capabilityCatalogLayoutModeNone:
		return nil, nil
	case capabilityCatalogLayoutModeFile:
		return loadCapabilityCatalogFile(layout.file)
	case capabilityCatalogLayoutModeDirectory:
		return loadCapabilityCatalogDirectory(layout.dir)
	default:
		return nil, fmt.Errorf("config: unsupported capability catalog mode %q", layout.mode)
	}
}

// AgentCapabilityCatalogDependencyPaths returns the filesystem inputs that can
// affect LoadAgentCapabilities for one agent directory.
func AgentCapabilityCatalogDependencyPaths(agentDir string) ([]string, error) {
	trimmedDir := strings.TrimSpace(agentDir)
	if trimmedDir == "" {
		return nil, errors.New("config: agent directory is required")
	}

	tomlPath := filepath.Join(trimmedDir, capabilityCatalogTOMLName)
	jsonPath := filepath.Join(trimmedDir, capabilityCatalogJSONName)
	dirPath := filepath.Join(trimmedDir, capabilityCatalogDirName)
	paths := []string{tomlPath, jsonPath, dirPath}

	info, err := os.Lstat(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			sort.Strings(paths)
			return paths, nil
		}
		return nil, fmt.Errorf("config: stat capability catalog directory %q: %w", dirPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		sort.Strings(paths)
		return paths, nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("config: read capability catalog directory %q: %w", dirPath, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("config: read capability catalog entry %q: %w", filepath.Join(dirPath, name), err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		switch filepath.Ext(name) {
		case capabilityFileExtTOML, capabilityFileExtJSON:
			paths = append(paths, filepath.Join(dirPath, name))
		}
	}

	sort.Strings(paths)
	return paths, nil
}

func detectCapabilityCatalogLayout(agentDir string) (capabilityCatalogLayout, error) {
	tomlPath := filepath.Join(agentDir, capabilityCatalogTOMLName)
	jsonPath := filepath.Join(agentDir, capabilityCatalogJSONName)
	dirPath := filepath.Join(agentDir, capabilityCatalogDirName)

	tomlExists, err := existingCapabilityCatalogFile(tomlPath)
	if err != nil {
		return capabilityCatalogLayout{}, err
	}
	jsonExists, err := existingCapabilityCatalogFile(jsonPath)
	if err != nil {
		return capabilityCatalogLayout{}, err
	}
	dirExists, err := existingCapabilityCatalogDir(dirPath)
	if err != nil {
		return capabilityCatalogLayout{}, err
	}

	files := make([]string, 0, 2)
	if tomlExists {
		files = append(files, tomlPath)
	}
	if jsonExists {
		files = append(files, jsonPath)
	}

	if dirExists && len(files) > 0 {
		conflicts := append([]string(nil), files...)
		conflicts = append(conflicts, dirPath)
		return capabilityCatalogLayout{}, fmt.Errorf(
			"config: validate capability catalog %q: mixed capability catalog layouts: %s",
			agentDir,
			joinQuotedPaths(conflicts),
		)
	}
	if len(files) > 1 {
		return capabilityCatalogLayout{}, fmt.Errorf(
			"config: validate capability catalog %q: multiple capability catalog files: %s",
			agentDir,
			joinQuotedPaths(files),
		)
	}
	if len(files) == 1 {
		return capabilityCatalogLayout{
			mode: capabilityCatalogLayoutModeFile,
			file: files[0],
		}, nil
	}
	if dirExists {
		return capabilityCatalogLayout{
			mode: capabilityCatalogLayoutModeDirectory,
			dir:  dirPath,
		}, nil
	}

	return capabilityCatalogLayout{}, nil
}

func existingCapabilityCatalogFile(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("config: stat capability catalog file %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("config: capability catalog file %q must be a file, not a symlink", path)
	}
	if info.IsDir() {
		return false, fmt.Errorf("config: capability catalog file %q must be a file", path)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("config: capability catalog file %q must be a regular file", path)
	}
	return true, nil
}

func existingCapabilityCatalogDir(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("config: stat capability catalog directory %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("config: capability catalog directory %q must be a directory, not a symlink", path)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("config: capability catalog directory %q must be a directory", path)
	}
	return true, nil
}

func loadCapabilityCatalogFile(path string) (*CapabilityCatalog, error) {
	content, exists, err := readOptionalRegularFile(path, "capability catalog")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("config: capability catalog %q disappeared before read", path)
	}

	switch filepath.Ext(path) {
	case capabilityFileExtTOML:
		return parseCapabilityCatalogTOML(content, path)
	case capabilityFileExtJSON:
		return parseCapabilityCatalogJSON(content, path)
	default:
		return nil, fmt.Errorf("config: unsupported capability catalog file %q", path)
	}
}
