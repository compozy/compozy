package loop

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

const (
	// ResourceKind is the desired-state resource kind for projected loop definitions.
	ResourceKind = resources.ResourceKind("loop")
	// ResourceMaxBytes is intentionally small because loop resources project metadata only.
	ResourceMaxBytes = 64 * 1024
	// NamePattern is the canonical loop definition name pattern.
	NamePattern = `^[a-z][a-z0-9_-]*$`
)

const (
	// SourceMarketplace identifies marketplace extension-contributed loop definitions.
	SourceMarketplace Source = "marketplace"
	// SourceUser identifies user-owned global loop definitions under $AGH_HOME.
	SourceUser Source = "user"
	// SourceAdditional identifies loop definitions discovered through additional workspace roots.
	SourceAdditional Source = "additional"
	// SourceWorkspace identifies workspace-local loop definitions under <workspace>/.agh.
	SourceWorkspace Source = "workspace"
)

var loopNamePattern = regexp.MustCompile(NamePattern)

// Source names the source-precedence class for one loop definition.
type Source string

// CatalogResourceSpec is the matchable catalog metadata projected from a loop definition.
type CatalogResourceSpec struct {
	UseWhen  string   `json:"use_when,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
	Category string   `json:"category,omitempty"`
}

// StartResourceSpec is the matchable start-surface metadata projected from a loop definition.
type StartResourceSpec struct {
	Kind             string   `json:"kind"`
	InputKeys        []string `json:"input_keys,omitempty"`
	InputMappingKeys []string `json:"input_mapping_keys,omitempty"`
}

// ResourceSpec is the resource-kernel projection for a loop definition.
//
// The canonical YAML body remains on the source filesystem. This spec stores
// only matchable catalog/start metadata plus source references needed by
// catalog, fork, and immutability flows.
type ResourceSpec struct {
	Name                   string              `json:"name"`
	Version                int                 `json:"version,omitempty"`
	Description            string              `json:"description,omitempty"`
	Catalog                CatalogResourceSpec `json:"catalog,omitzero"`
	ContractGoal           string              `json:"contract_goal,omitempty"`
	Start                  []StartResourceSpec `json:"start,omitempty"`
	Source                 Source              `json:"source"`
	Dir                    string              `json:"dir,omitempty"`
	FilePath               string              `json:"file_path,omitempty"`
	DefinitionChecksum     string              `json:"definition_checksum,omitempty"`
	InstalledFromExtension string              `json:"installed_from_extension,omitempty"`
}

// ResourceParseOptions carries source context for a YAML-to-resource projection.
type ResourceParseOptions struct {
	Source                 Source
	Dir                    string
	FilePath               string
	InstalledFromExtension string
	Linter                 Linter
}

// NewResourceCodec returns the JSON codec for loop resource projections.
func NewResourceCodec() (resources.KindCodec[ResourceSpec], error) {
	return resources.NewJSONCodec(ResourceKind, ResourceMaxBytes, validateResourceSpec)
}

// Normalize returns the canonical source value.
func (s Source) Normalize() Source {
	return Source(strings.TrimSpace(string(s)))
}

// Valid reports whether s is a supported loop source class.
func (s Source) Valid() bool {
	switch s.Normalize() {
	case SourceMarketplace, SourceUser, SourceAdditional, SourceWorkspace:
		return true
	default:
		return false
	}
}

// PrecedenceRank returns the explicit task_03 source-precedence rank.
func (s Source) PrecedenceRank() int {
	switch s.Normalize() {
	case SourceMarketplace:
		return 20
	case SourceUser:
		return 30
	case SourceAdditional:
		return 40
	case SourceWorkspace:
		return 50
	default:
		return 0
	}
}

// ValidateName returns a trimmed loop name when it matches the canonical name pattern.
func ValidateName(name string) (string, error) {
	cleanName := strings.TrimSpace(name)
	if !loopNamePattern.MatchString(cleanName) {
		return "", fmt.Errorf("loop: name %q must match %s", name, NamePattern)
	}
	return cleanName, nil
}
