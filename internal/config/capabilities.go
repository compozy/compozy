package config

import (
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	capabilityCatalogTOMLName = "capabilities.toml"
	capabilityCatalogJSONName = "capabilities.json"
	capabilityCatalogDirName  = "capabilities"
	capabilityFileExtTOML     = ".toml"
	capabilityFileExtJSON     = ".json"
)

type capabilityCatalogLayoutMode string

const capabilityCatalogLayoutModeFile capabilityCatalogLayoutMode = "file"

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
func LoadAgentCapabilities(agentDir string) (catalog *CapabilityCatalog, err error) {
	trimmedDir := strings.TrimSpace(agentDir)
	if trimmedDir == "" {
		return nil, errors.New("config: agent directory is required")
	}

	directory, err := fileutil.OpenDirectory(trimmedDir)
	if err != nil {
		return nil, fmt.Errorf("config: open agent directory %q: %w", trimmedDir, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			catalog = nil
			err = errors.Join(err, fmt.Errorf("config: close agent directory %q: %w", trimmedDir, closeErr))
		}
	}()

	return loadAgentCapabilitiesFromDirectory(directory, trimmedDir)
}

func loadAgentCapabilitiesFromDirectory(directory *fileutil.Directory, agentDir string) (*CapabilityCatalog, error) {
	tomlPath := filepath.Join(agentDir, capabilityCatalogTOMLName)
	jsonPath := filepath.Join(agentDir, capabilityCatalogJSONName)
	dirPath := filepath.Join(agentDir, capabilityCatalogDirName)

	tomlContent, tomlExists, err := readOptionalCapabilityCatalogFile(directory, capabilityCatalogTOMLName, tomlPath)
	if err != nil {
		return nil, err
	}
	jsonContent, jsonExists, err := readOptionalCapabilityCatalogFile(directory, capabilityCatalogJSONName, jsonPath)
	if err != nil {
		return nil, err
	}
	catalogDirectory, directoryExists, err := openOptionalCapabilityCatalogDirectory(directory, dirPath)
	if err != nil {
		return nil, err
	}

	if directoryExists && (tomlExists || jsonExists) {
		conflicts := make([]string, 0, 3)
		if tomlExists {
			conflicts = append(conflicts, tomlPath)
		}
		if jsonExists {
			conflicts = append(conflicts, jsonPath)
		}
		conflicts = append(conflicts, dirPath)
		validationErr := fmt.Errorf(
			"config: validate capability catalog %q: mixed capability catalog layouts: %s",
			agentDir,
			joinQuotedPaths(conflicts),
		)
		if closeErr := catalogDirectory.Close(); closeErr != nil {
			return nil, errors.Join(
				validationErr,
				fmt.Errorf("config: close capability catalog directory %q: %w", dirPath, closeErr),
			)
		}
		return nil, validationErr
	}

	if tomlExists && jsonExists {
		return nil, fmt.Errorf(
			"config: validate capability catalog %q: multiple capability catalog files: %s",
			agentDir,
			joinQuotedPaths([]string{tomlPath, jsonPath}),
		)
	}
	if tomlExists {
		return parseCapabilityCatalogTOML(tomlContent, tomlPath)
	}
	if jsonExists {
		return parseCapabilityCatalogJSON(jsonContent, jsonPath)
	}
	if directoryExists {
		return loadCapabilityCatalogDirectoryFromDirectory(catalogDirectory, dirPath)
	}

	return nil, nil
}

func readOptionalCapabilityCatalogFile(directory *fileutil.Directory, name string, path string) ([]byte, bool, error) {
	content, _, err := directory.ReadRegularFile(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		if errors.Is(err, fileutil.ErrSymlink) {
			return nil, false, fmt.Errorf("config: capability catalog file %q must be a file, not a symlink", path)
		}
		if errors.Is(err, fileutil.ErrDirectory) {
			return nil, false, fmt.Errorf("config: capability catalog file %q must be a file", path)
		}
		if errors.Is(err, fileutil.ErrNotRegular) {
			return nil, false, fmt.Errorf("config: capability catalog file %q must be a regular file", path)
		}
		return nil, false, fmt.Errorf("config: open capability catalog file %q: %w", path, err)
	}
	return content, true, nil
}

func openOptionalCapabilityCatalogDirectory(
	parent *fileutil.Directory,
	path string,
) (*fileutil.Directory, bool, error) {
	directory, err := parent.OpenDirectory(capabilityCatalogDirName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		if errors.Is(err, fileutil.ErrSymlink) {
			return nil, false, fmt.Errorf(
				"config: capability catalog directory %q must be a directory, not a symlink",
				path,
			)
		}
		if errors.Is(err, fileutil.ErrNotDirectory) {
			return nil, false, fmt.Errorf("config: capability catalog directory %q must be a directory", path)
		}
		return nil, false, fmt.Errorf("config: open capability catalog directory %q: %w", path, err)
	}
	return directory, true, nil
}

// AgentCapabilityCatalogDependencyPaths returns the filesystem inputs that can
// affect LoadAgentCapabilities for one agent directory.
func AgentCapabilityCatalogDependencyPaths(agentDir string) (paths []string, err error) {
	trimmedDir := strings.TrimSpace(agentDir)
	if trimmedDir == "" {
		return nil, errors.New("config: agent directory is required")
	}

	tomlPath := filepath.Join(trimmedDir, capabilityCatalogTOMLName)
	jsonPath := filepath.Join(trimmedDir, capabilityCatalogJSONName)
	dirPath := filepath.Join(trimmedDir, capabilityCatalogDirName)
	paths = []string{tomlPath, jsonPath, dirPath}

	directory, err := fileutil.OpenDirectory(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			sort.Strings(paths)
			return paths, nil
		}
		if errors.Is(err, fileutil.ErrNotDirectory) {
			sort.Strings(paths)
			return paths, nil
		}
		return nil, fmt.Errorf("config: open capability catalog directory %q: %w", dirPath, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			paths = nil
			err = errors.Join(err, fmt.Errorf("config: close capability catalog directory %q: %w", dirPath, closeErr))
		}
	}()

	entries, err := directory.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("config: read capability catalog directory %q: %w", dirPath, err)
	}

	for _, name := range entries {
		if strings.HasPrefix(name, ".") {
			continue
		}
		switch filepath.Ext(name) {
		case capabilityFileExtTOML, capabilityFileExtJSON:
			file, openErr := directory.OpenRegularFile(name)
			if errors.Is(openErr, fileutil.ErrDirectory) || errors.Is(openErr, fileutil.ErrNotRegular) {
				continue
			}
			if openErr != nil {
				return nil, fmt.Errorf(
					"config: read capability catalog entry %q: %w",
					filepath.Join(dirPath, name),
					openErr,
				)
			}
			if closeErr := file.Close(); closeErr != nil {
				return nil, fmt.Errorf(
					"config: close capability catalog entry %q: %w",
					filepath.Join(dirPath, name),
					closeErr,
				)
			}
			paths = append(paths, filepath.Join(dirPath, name))
		}
	}

	sort.Strings(paths)
	return paths, nil
}
