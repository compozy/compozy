package prompts

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
)

//go:embed *.*.tmpl *.*.md
var embeddedFS embed.FS

// VersionV1 is the first stable prompt asset version for Memory v2 Slice 1.
const VersionV1 = "v1"

var (
	// ErrAssetNotFound reports that a prompt asset name is unknown.
	ErrAssetNotFound = errors.New("memory prompts: asset not found")
	// ErrVersionNotFound reports that a prompt asset has no requested version.
	ErrVersionNotFound = errors.New("memory prompts: version not found")
)

// Name identifies one versioned memory prompt or policy asset.
type Name string

const (
	// NameDecide loads the write-controller tiebreaker prompt.
	NameDecide Name = "decide"
	// NameCheckpointSummary loads the workspace checkpoint update prompt.
	NameCheckpointSummary Name = "checkpoint_summary"
	// NameDream loads the dreaming curator prompt.
	NameDream Name = "dream"
	// NameExtract loads the turn extractor prompt.
	NameExtract Name = "extract"
	// NameWhatNotToSave loads the deterministic persistence denylist policy.
	NameWhatNotToSave Name = "what_not_to_save"
)

// Asset is one loaded prompt or policy asset.
type Asset struct {
	Name     Name
	Version  string
	Filename string
	Content  string
}

// Registry loads versioned memory prompt assets from an explicit filesystem.
type Registry struct {
	fsys   fs.FS
	assets map[Name]map[string]string
	latest map[Name]string
}

// DefaultRegistry returns a registry backed by the embedded Memory v2 assets.
func DefaultRegistry() Registry {
	return NewRegistry(embeddedFS)
}

// NewRegistry creates a registry that reads known asset filenames from fsys.
func NewRegistry(fsys fs.FS) Registry {
	return Registry{
		fsys:   fsys,
		assets: defaultAssetIndex(),
		latest: defaultLatestIndex(),
	}
}

// Load returns a named asset by explicit version from the embedded registry.
func Load(name Name, version string) (Asset, error) {
	return DefaultRegistry().Load(name, version)
}

// LoadLatest returns the latest embedded version for a named asset.
func LoadLatest(name Name) (Asset, error) {
	return DefaultRegistry().LoadLatest(name)
}

// ParseTemplate parses a named embedded asset version with missing keys rejected.
func ParseTemplate(name Name, version string) (*template.Template, error) {
	return DefaultRegistry().ParseTemplate(name, version)
}

// LoadLatest returns the latest configured version for a named asset.
func (r Registry) LoadLatest(name Name) (Asset, error) {
	normalized := normalizeName(name)
	version, ok := r.latest[normalized]
	if !ok {
		return Asset{}, fmt.Errorf("%w: %s", ErrAssetNotFound, normalized)
	}
	return r.Load(normalized, version)
}

// Load returns a named asset by explicit version from the registry filesystem.
func (r Registry) Load(name Name, version string) (Asset, error) {
	normalized := normalizeName(name)
	filename, err := r.filename(normalized, version)
	if err != nil {
		return Asset{}, err
	}
	content, err := fs.ReadFile(r.fsys, filename)
	if err != nil {
		return Asset{}, fmt.Errorf("memory prompts: read %s %s: %w", normalized, version, err)
	}
	return Asset{
		Name:     normalized,
		Version:  normalizeVersion(version),
		Filename: filename,
		Content:  string(content),
	}, nil
}

// ParseTemplate parses a named asset version with missing keys rejected.
func (r Registry) ParseTemplate(name Name, version string) (*template.Template, error) {
	asset, err := r.Load(name, version)
	if err != nil {
		return nil, err
	}
	parsed, err := template.New(asset.Filename).Option("missingkey=error").Parse(asset.Content)
	if err != nil {
		return nil, fmt.Errorf("memory prompts: parse %s %s: %w", asset.Name, asset.Version, err)
	}
	return parsed, nil
}

func (r Registry) filename(name Name, version string) (string, error) {
	normalized := normalizeName(name)
	versions, ok := r.assets[normalized]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrAssetNotFound, normalized)
	}
	normalizedVersion := normalizeVersion(version)
	filename, ok := versions[normalizedVersion]
	if !ok {
		return "", fmt.Errorf("%w: %s %s", ErrVersionNotFound, normalized, normalizedVersion)
	}
	return filename, nil
}

func defaultAssetIndex() map[Name]map[string]string {
	return map[Name]map[string]string{
		NameCheckpointSummary: {VersionV1: "checkpoint_summary.v1.tmpl"},
		NameDecide:            {VersionV1: "decide.v1.tmpl"},
		NameDream:             {VersionV1: "dream.v1.tmpl"},
		NameExtract:           {VersionV1: "extract.v1.tmpl"},
		NameWhatNotToSave:     {VersionV1: "what_not_to_save.v1.md"},
	}
}

func defaultLatestIndex() map[Name]string {
	return map[Name]string{
		NameCheckpointSummary: VersionV1,
		NameDecide:            VersionV1,
		NameDream:             VersionV1,
		NameExtract:           VersionV1,
		NameWhatNotToSave:     VersionV1,
	}
}

func normalizeName(name Name) Name {
	return Name(strings.TrimSpace(strings.ToLower(string(name))))
}

func normalizeVersion(version string) string {
	return strings.TrimSpace(strings.ToLower(version))
}
