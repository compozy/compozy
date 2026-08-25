package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateForScope validates source policy for the layer that will own a write.
func (c SkillsConfig) ValidateForScope(scope WriteScope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := ValidateSkillSources(c.Sources); err != nil {
		return err
	}
	if err := validateCustomSkillSourcePaths(c.CustomSources, scope); err != nil {
		return err
	}
	return validateDuplicateSkillSourceRoots(c.Sources, c.CustomSources)
}

func validateCustomSkillSourcePaths(paths []string, scope WriteScope) error {
	seen := make(map[string]string, len(paths))
	for _, raw := range paths {
		path := strings.TrimSpace(raw)
		if path == "" {
			return invalidSkillSourcePath(raw, "skill source path is required")
		}
		if strings.HasPrefix(path, "~") && path != "~" && !strings.HasPrefix(path, "~/") {
			return invalidSkillSourcePath(raw, "only ~/ home-relative paths are supported")
		}
		expanded, err := expandUserPath(path)
		if err != nil {
			return invalidSkillSourcePath(raw, err.Error())
		}
		if !filepath.IsAbs(expanded) && scope != WriteScopeWorkspace {
			return invalidSkillSourcePath(raw, "workspace-relative paths require workspace scope")
		}
		canonical := canonicalSkillSourcePath(path)
		if existing, ok := seen[canonical]; ok {
			return &SkillSourceValidationError{
				Code:           "duplicate_skill_source",
				Path:           raw,
				ExistingSource: existing,
				Message:        fmt.Sprintf("duplicate skill source path %q already configured by %q", raw, existing),
			}
		}
		seen[canonical] = raw
	}
	return nil
}

func validateDuplicateSkillSourceRoots(sources []string, custom []string) error {
	owners := make(map[string]string, len(sources)*2)
	for _, slug := range sources {
		preset, ok := skillSourcePreset(slug)
		if !ok {
			continue
		}
		for _, path := range []string{preset.GlobalPath, preset.WorkspaceRel} {
			if strings.TrimSpace(path) == "" {
				continue
			}
			owners[canonicalSkillSourcePath(path)] = slug
		}
	}
	for _, raw := range custom {
		canonical := canonicalSkillSourcePath(raw)
		owner, ok := owners[canonical]
		if !ok {
			continue
		}
		return &SkillSourceValidationError{
			Code:           "duplicate_skill_source",
			Path:           raw,
			ExistingSource: owner,
			Message:        fmt.Sprintf("skill source path %q is already owned by source %q", raw, owner),
		}
	}
	return nil
}

func invalidSkillSourcePath(path string, reason string) error {
	return &SkillSourceValidationError{
		Code:    "invalid_source_path",
		Path:    path,
		Message: fmt.Sprintf("invalid skill source path %q: %s", path, reason),
	}
}
