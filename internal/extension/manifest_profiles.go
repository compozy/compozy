package extensionpkg

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
	Sandbox  string `toml:"sandbox,omitempty"  json:"sandbox,omitempty"`
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
