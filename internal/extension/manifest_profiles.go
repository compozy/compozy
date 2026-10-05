package extensionpkg

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/BurntSushi/toml"
)

// ManifestProfile declares one profile an extension creates at install time.
type ManifestProfile struct {
	Name        string                      `toml:"name"                  json:"name"`
	Color       string                      `toml:"color,omitempty"       json:"color,omitempty"`
	Icon        string                      `toml:"icon,omitempty"        json:"icon,omitempty"`
	Emoji       string                      `toml:"emoji,omitempty"       json:"emoji,omitempty"`
	Defaults    ManifestProfileDefaults     `toml:"defaults,omitempty"    json:"defaults,omitzero"`
	Credentials []ManifestProfileCredential `toml:"credentials,omitempty" json:"credentials,omitempty"`
}

// ManifestProfileDefaults are seeded once when a declaration creates a profile.
type ManifestProfileDefaults struct {
	Agent    string `toml:"agent,omitempty"    json:"agent,omitempty"`
	Provider string `toml:"provider,omitempty" json:"provider,omitempty"`
}

// ManifestProfileCredential is one vault-backed setup requirement.
type ManifestProfileCredential struct {
	Provider string `toml:"provider" json:"provider"`
	Slot     string `toml:"slot"     json:"slot"`
}

// ManifestResourcePath binds one static resource path to every profile or to
// exactly one profile name when Profile is set.
type ManifestResourcePath struct {
	Path    string `toml:"path"              json:"path"`
	Profile string `toml:"profile,omitempty" json:"profile,omitempty"`
}

var (
	_ json.Unmarshaler = (*ManifestResourcePath)(nil)
	_ toml.Unmarshaler = (*ManifestResourcePath)(nil)
)

// UnmarshalJSON translates string paths until their removal in v0.3.0-beta.31.
func (r *ManifestResourcePath) UnmarshalJSON(data []byte) error {
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '"' {
		var path string
		if err := json.Unmarshal(data, &path); err != nil {
			return err
		}
		*r = ManifestResourcePath{Path: path}
		return nil
	}
	type resourcePath ManifestResourcePath
	var decoded resourcePath
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*r = ManifestResourcePath(decoded)
	return nil
}

// UnmarshalTOML shares the JSON boundary's path and placement validation.
func (r *ManifestResourcePath) UnmarshalTOML(value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("extension: encode TOML resource path: %w", err)
	}
	return r.UnmarshalJSON(encoded)
}

func manifestResourcePaths(resources []ManifestResourcePath) []string {
	paths := make([]string, 0, len(resources))
	for _, resource := range resources {
		paths = append(paths, resource.Path)
	}
	return paths
}

// ResourcePaths projects typed manifest resource declarations for loaders that
// only consume package-relative paths.
func ResourcePaths(resources []ManifestResourcePath) []string {
	return manifestResourcePaths(resources)
}

func unplacedManifestResourcePaths(paths []string) []ManifestResourcePath {
	resources := make([]ManifestResourcePath, 0, len(paths))
	for _, path := range paths {
		resources = append(resources, ManifestResourcePath{Path: path})
	}
	return resources
}
