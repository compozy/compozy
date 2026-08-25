package config

import (
	"fmt"
	"slices"
	"strings"
)

const (
	SkillSourceCompozy = "compozy"
	SkillSourceAgents  = "agents"
	SkillSourceClaude  = "claude"
)

// SkillSourcePreset describes one curated filesystem convention understood by CompozyOS.
type SkillSourcePreset struct {
	Slug                   string
	Label                  string
	WorkspaceRel           string
	GlobalPath             string
	WorkspaceNativeReaders []string
	GlobalNativeReaders    []string
	AlwaysOn               bool
	DefaultOn              bool
}

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

// Error returns the stable human-readable validation message.
func (e *SkillSourceValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// SkillSourcePresets returns the closed preset table in public display order.
func SkillSourcePresets() []SkillSourcePreset {
	return []SkillSourcePreset{
		{
			Slug:                   SkillSourceCompozy,
			Label:                  "Compozy",
			AlwaysOn:               true,
			WorkspaceNativeReaders: []string{},
			GlobalNativeReaders:    []string{},
		},
		{
			Slug:                   SkillSourceAgents,
			Label:                  "Agents",
			WorkspaceRel:           ".agents/skills",
			GlobalPath:             "~/.agents/skills",
			WorkspaceNativeReaders: []string{"openclaw", "hermes"},
			GlobalNativeReaders:    []string{"openclaw"},
			DefaultOn:              true,
		},
		{
			Slug:                   SkillSourceClaude,
			Label:                  "Claude",
			WorkspaceRel:           ".claude/skills",
			GlobalPath:             "~/.claude/skills",
			WorkspaceNativeReaders: []string{"claude"},
			GlobalNativeReaders:    []string{"claude"},
		},
	}
}

// ValidateSkillSources rejects duplicates and slugs outside the curated table.
func ValidateSkillSources(slugs []string) error {
	valid := configurableSkillSourceSlugs()
	seen := make(map[string]struct{}, len(slugs))
	for _, raw := range slugs {
		slug := strings.TrimSpace(raw)
		if _, ok := seen[slug]; ok {
			return &SkillSourceValidationError{
				Code:    "duplicate_skill_source",
				Source:  slug,
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
			message = fmt.Sprintf("unknown skill source preset %q (did you mean %q?); valid: %s", slug, suggestion, strings.Join(valid, ", "))
		}
		return &SkillSourceValidationError{
			Code:       "unknown_skill_source",
			Source:     slug,
			Valid:      valid,
			Suggestion: suggestion,
			Message:    message,
		}
	}
	return nil
}

func configurableSkillSourceSlugs() []string {
	return []string{SkillSourceAgents, SkillSourceClaude}
}

func skillSourcePreset(slug string) (SkillSourcePreset, bool) {
	for _, preset := range SkillSourcePresets() {
		if preset.Slug == slug {
			return preset, true
		}
	}
	return SkillSourcePreset{}, false
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
