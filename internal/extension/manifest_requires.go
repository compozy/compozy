package extensionpkg

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

// dependenciesFloor is the first release that enforces requires.extensions (rule 1c). Older daemons
// ignore the unknown table, so the manifest must require this release to be refused there.
const dependenciesFloor = "0.7.9"

const (
	manifestRequiresKey             = "requires"
	manifestExtensionsKey           = "extensions"
	manifestRequiresExtensionsField = manifestRequiresKey + "." + manifestExtensionsKey
	manifestDependencySourceKey     = "source"
	manifestDependencyRefKey        = "ref"
	manifestDependencyMinVersionKey = "min_version"
)

// ManifestRequires groups install-time requirements. Experimental.
type ManifestRequires struct {
	Extensions []ExtensionDependency `toml:"extensions,omitempty" json:"extensions,omitempty"`
}

// ExtensionDependency declares one extension this package needs. Experimental.
type ExtensionDependency struct {
	Name       string `toml:"name"                  json:"name"`
	Source     string `toml:"source"                json:"source"`
	Ref        string `toml:"ref"                   json:"ref"`
	MinVersion string `toml:"min_version,omitempty" json:"min_version,omitempty"`
}

// VersionSatisfies reports whether version is at least minVersion. An empty minimum always holds.
func VersionSatisfies(version, minVersion string) bool {
	if strings.TrimSpace(minVersion) == "" {
		return true
	}
	current, currentOK := parseSemanticVersion(version)
	required, requiredOK := parseSemanticVersion(minVersion)
	return currentOK && requiredOK && compareSemanticVersions(current, required) >= 0
}

// HigherVersion returns the higher of two semantic versions, treating empty as the lowest.
func HigherVersion(left, right string) string {
	if strings.TrimSpace(left) == "" || VersionSatisfies(right, left) {
		return right
	}
	return left
}

func normalizeManifestRequires(requires ManifestRequires) ManifestRequires {
	if len(requires.Extensions) == 0 {
		return ManifestRequires{}
	}
	normalized := make([]ExtensionDependency, 0, len(requires.Extensions))
	for _, dependency := range requires.Extensions {
		normalized = append(normalized, ExtensionDependency{
			Name:       strings.TrimSpace(dependency.Name),
			Source:     strings.ToLower(strings.TrimSpace(dependency.Source)),
			Ref:        strings.TrimSpace(dependency.Ref),
			MinVersion: strings.TrimSpace(dependency.MinVersion),
		})
	}
	return ManifestRequires{Extensions: normalized}
}

// validateManifestRequires checks the experimental dependency declarations (rule 10) and their floor (rule 1c).
func validateManifestRequires(manifest *Manifest) error {
	dependencies := manifest.Requires.Extensions
	if len(dependencies) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(dependencies))
	for index, dependency := range dependencies {
		field := fmt.Sprintf("%s[%d]", manifestRequiresExtensionsField, index)
		if err := validateExtensionDependency(field, dependency); err != nil {
			return err
		}
		if dependency.Name == strings.TrimSpace(manifest.Name) {
			return &ManifestValidationError{
				Field:   field + "." + manifestNameKey,
				Value:   dependency.Name,
				Message: "an extension cannot depend on itself",
			}
		}
		if _, duplicate := seen[dependency.Name]; duplicate {
			return &ManifestValidationError{
				Field:   field + "." + manifestNameKey,
				Value:   dependency.Name,
				Message: "dependency is declared more than once",
			}
		}
		seen[dependency.Name] = struct{}{}
	}
	return validateFeatureFloor(manifest.MinCompozyVersion, "extension dependencies", dependenciesFloor)
}

func validateExtensionDependency(field string, dependency ExtensionDependency) error {
	if err := requireField(field+"."+manifestNameKey, dependency.Name); err != nil {
		return err
	}
	if err := requireField(field+"."+manifestDependencyRefKey, dependency.Ref); err != nil {
		return err
	}
	switch contract.InstallExtensionSource(dependency.Source) {
	case contract.InstallExtensionSourceCurated, contract.InstallExtensionSourceMarketplace,
		contract.InstallExtensionSourceGitHub, contract.InstallExtensionSourceGit,
		contract.InstallExtensionSourceLocalPath:
	default:
		return &ManifestValidationError{
			Field: field + "." + manifestDependencySourceKey, Value: dependency.Source,
			Message: "must be one of curated, marketplace, github, git, local_path",
		}
	}
	if dependency.MinVersion != "" {
		return validateSemanticVersionField(field+"."+manifestDependencyMinVersionKey, dependency.MinVersion)
	}
	return nil
}

func manifestRequiresTOML(requires ManifestRequires) map[string]any {
	if len(requires.Extensions) == 0 {
		return nil
	}
	entries := make([]map[string]any, 0, len(requires.Extensions))
	for _, dependency := range requires.Extensions {
		entry := map[string]any{
			manifestNameKey:             dependency.Name,
			manifestDependencySourceKey: dependency.Source,
			manifestDependencyRefKey:    dependency.Ref,
		}
		putNonEmpty(entry, manifestDependencyMinVersionKey, dependency.MinVersion)
		entries = append(entries, entry)
	}
	return map[string]any{manifestExtensionsKey: entries}
}
