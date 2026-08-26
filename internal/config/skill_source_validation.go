package config

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

const (
	skillSourceErrorDuplicate   = "duplicate_skill_source"
	skillSourceErrorInvalidPath = "invalid_source_path"
	skillSourceErrorUnknown     = "unknown_skill_source"
)

// SkillSourceValidationError reports one portable source-policy validation failure.
type SkillSourceValidationError struct {
	Code           string
	Source         string
	Field          string
	Path           string
	ExistingSource string
	Valid          []string
	Suggestion     string
	Message        string
}

func (e *SkillSourceValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// ValidateSkillSources rejects duplicates and slugs outside the curated table.
func ValidateSkillSources(slugs []string) error {
	valid := configurableSkillSourceSlugs()
	seen := make(map[string]struct{}, len(slugs))
	for _, raw := range slugs {
		slug := strings.TrimSpace(raw)
		if _, ok := seen[slug]; ok {
			return &SkillSourceValidationError{
				Code: skillSourceErrorDuplicate, Source: slug,
				Message: fmt.Sprintf("duplicate skill source preset %q", slug),
			}
		}
		seen[slug] = struct{}{}
		if slices.Contains(valid, slug) {
			continue
		}
		suggestion := closestSkillSource(slug, valid)
		message := fmt.Sprintf("unknown skill source preset %q; valid: %s", slug, strings.Join(valid, ", "))
		if suggestion != "" {
			message = fmt.Sprintf(
				"unknown skill source preset %q (did you mean %q?); valid: %s",
				slug, suggestion, strings.Join(valid, ", "),
			)
		}
		return &SkillSourceValidationError{
			Code: skillSourceErrorUnknown, Source: slug, Valid: valid,
			Suggestion: suggestion, Message: message,
		}
	}
	return nil
}

func closestSkillSource(value string, candidates []string) string {
	best := ""
	bestDistance := -1
	for _, candidate := range candidates {
		distance := editDistance(strings.ToLower(value), candidate)
		if bestDistance < 0 || distance < bestDistance || (distance == bestDistance && candidate < best) {
			best = candidate
			bestDistance = distance
		}
	}
	if bestDistance > 3 {
		return ""
	}
	return best
}

func editDistance(left string, right string) int {
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for leftIndex, leftRune := range []rune(left) {
		current := make([]int, len(previous))
		current[0] = leftIndex + 1
		for rightIndex, rightRune := range []rune(right) {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			current[rightIndex+1] = min(
				current[rightIndex]+1,
				previous[rightIndex+1]+1,
				previous[rightIndex]+cost,
			)
		}
		previous = current
	}
	return previous[len(previous)-1]
}

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

// ValidateForScopeAtRoot rejects physical collisions using the owning layer's path base.
func (c SkillsConfig) ValidateForScopeAtRoot(scope WriteScope, layerRoot string, home HomePaths) error {
	if err := c.ValidateForScope(scope); err != nil {
		return err
	}
	var roots []SkillRootSpec
	switch scope {
	case WriteScopeUser:
		roots = ResolveGlobalSkillRoots(&c, home)
	case WriteScopeProfile:
		roots = (WorkspaceDiscoveryRoot{Dir: layerRoot, Source: WorkspaceDiscoverySourceProfile}).SkillsDirs(&c)
	case WriteScopeWorkspace:
		roots = (WorkspaceDiscoveryRoot{
			Dir: layerRoot, WorkspaceRoot: layerRoot, Source: WorkspaceDiscoverySourceWorkspace,
		}).SkillsDirs(&c)
	default:
		return nil
	}
	seen := make(map[string]SkillRootSpec, len(roots))
	for _, root := range roots {
		canonical := canonicalSkillSourcePath(root.Dir)
		if existing, ok := seen[canonical]; ok {
			path := root.Dir
			owner := existing.SourceSlug
			if existing.Kind == RootKindCustom && root.Kind != RootKindCustom {
				path = existing.Dir
				owner = root.SourceSlug
			}
			return &SkillSourceValidationError{
				Code: skillSourceErrorDuplicate, Path: path, ExistingSource: owner,
				Message: fmt.Sprintf("skill source path %q is already owned by source %q", path, owner),
			}
		}
		seen[canonical] = root
	}
	return nil
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
				Code:           skillSourceErrorDuplicate,
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
			Code:           skillSourceErrorDuplicate,
			Path:           raw,
			ExistingSource: owner,
			Message:        fmt.Sprintf("skill source path %q is already owned by source %q", raw, owner),
		}
	}
	return nil
}

func invalidSkillSourcePath(path string, reason string) error {
	return &SkillSourceValidationError{
		Code:    skillSourceErrorInvalidPath,
		Path:    path,
		Message: fmt.Sprintf("invalid skill source path %q: %s", path, reason),
	}
}
