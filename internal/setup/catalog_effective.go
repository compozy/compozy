package setup

import (
	"slices"
	"strings"
)

// CatalogAssetKind identifies the setup asset family participating in conflict resolution.
type CatalogAssetKind string

const (
	// CatalogAssetKindSkill identifies skill assets.
	CatalogAssetKindSkill CatalogAssetKind = "skill"
	// CatalogAssetKindReusableAgent identifies reusable-agent assets.
	CatalogAssetKindReusableAgent CatalogAssetKind = "reusable-agent"
)

// CatalogConflictResolution describes why one setup asset won over another.
type CatalogConflictResolution string

const (
	// CatalogConflictCoreWins indicates a bundled core asset shadowed an extension asset.
	CatalogConflictCoreWins CatalogConflictResolution = "core-wins"
	// CatalogConflictExtensionPrecedence indicates one extension asset shadowed another by precedence.
	CatalogConflictExtensionPrecedence CatalogConflictResolution = "extension-precedence"
)

// AssetRef captures the source metadata needed to explain one effective-setup conflict.
type AssetRef struct {
	Origin          AssetOrigin
	Name            string
	ExtensionName   string
	ExtensionSource string
	ManifestPath    string
	ResolvedPath    string
}

// CatalogConflict records one ignored setup asset and the winner that shadowed it.
type CatalogConflict struct {
	Kind       CatalogAssetKind
	Name       string
	Resolution CatalogConflictResolution
	Winner     AssetRef
	Ignored    AssetRef
}

// EffectiveCatalog summarizes the setup assets eligible for installation after conflict resolution.
type EffectiveCatalog struct {
	Skills         []Skill
	ReusableAgents []ReusableAgent
	Conflicts      []CatalogConflict
}

// BuildEffectiveCatalog merges bundled and extension setup assets into one effective install catalog.
func BuildEffectiveCatalog(
	bundledSkills []Skill,
	extensionSkills []Skill,
	bundledReusableAgents []ReusableAgent,
	extensionReusableAgents []ReusableAgent,
) EffectiveCatalog {
	skills, skillConflicts := resolveEffectiveSkills(bundledSkills, extensionSkills)
	reusableAgents, reusableAgentConflicts := resolveEffectiveReusableAgents(
		bundledReusableAgents,
		extensionReusableAgents,
	)

	conflicts := make([]CatalogConflict, 0, len(skillConflicts)+len(reusableAgentConflicts))
	conflicts = append(conflicts, skillConflicts...)
	conflicts = append(conflicts, reusableAgentConflicts...)
	slices.SortFunc(conflicts, compareCatalogConflict)

	return EffectiveCatalog{
		Skills:         skills,
		ReusableAgents: reusableAgents,
		Conflicts:      conflicts,
	}
}

func resolveEffectiveSkills(bundled []Skill, extension []Skill) ([]Skill, []CatalogConflict) {
	index := make(map[string]Skill, len(bundled)+len(extension))
	effective := make([]Skill, 0, len(bundled)+len(extension))
	conflicts := make([]CatalogConflict, 0)

	for i := range bundled {
		skill := bundled[i]
		key := catalogAssetKey(skill.Name)
		if _, ok := index[key]; ok {
			continue
		}
		index[key] = skill
		effective = append(effective, skill)
	}

	orderedExtensionSkills := sortedExtensionSkills(extension)
	for i := range orderedExtensionSkills {
		skill := orderedExtensionSkills[i]
		key := catalogAssetKey(skill.Name)
		winner, ok := index[key]
		if !ok {
			index[key] = skill
			effective = append(effective, skill)
			continue
		}

		resolution := CatalogConflictExtensionPrecedence
		if winner.Origin == AssetOriginBundled {
			resolution = CatalogConflictCoreWins
		}
		conflicts = append(conflicts, CatalogConflict{
			Kind:       CatalogAssetKindSkill,
			Name:       skill.Name,
			Resolution: resolution,
			Winner:     skillAssetRef(winner),
			Ignored:    skillAssetRef(skill),
		})
	}

	slices.SortFunc(effective, func(left, right Skill) int {
		if diff := strings.Compare(left.Name, right.Name); diff != 0 {
			return diff
		}
		return compareAssetRef(skillAssetRef(left), skillAssetRef(right))
	})

	return effective, conflicts
}

func resolveEffectiveReusableAgents(
	bundled []ReusableAgent,
	extension []ReusableAgent,
) ([]ReusableAgent, []CatalogConflict) {
	index := make(map[string]ReusableAgent, len(bundled)+len(extension))
	effective := make([]ReusableAgent, 0, len(bundled)+len(extension))
	conflicts := make([]CatalogConflict, 0)

	for i := range bundled {
		reusableAgent := bundled[i]
		key := catalogAssetKey(reusableAgent.Name)
		if _, ok := index[key]; ok {
			continue
		}
		index[key] = reusableAgent
		effective = append(effective, reusableAgent)
	}

	orderedExtensionReusableAgents := sortedExtensionReusableAgents(extension)
	for i := range orderedExtensionReusableAgents {
		reusableAgent := orderedExtensionReusableAgents[i]
		key := catalogAssetKey(reusableAgent.Name)
		winner, ok := index[key]
		if !ok {
			index[key] = reusableAgent
			effective = append(effective, reusableAgent)
			continue
		}

		resolution := CatalogConflictExtensionPrecedence
		if winner.Origin == AssetOriginBundled {
			resolution = CatalogConflictCoreWins
		}
		conflicts = append(conflicts, CatalogConflict{
			Kind:       CatalogAssetKindReusableAgent,
			Name:       reusableAgent.Name,
			Resolution: resolution,
			Winner:     reusableAgentAssetRef(winner),
			Ignored:    reusableAgentAssetRef(reusableAgent),
		})
	}

	slices.SortFunc(effective, func(left, right ReusableAgent) int {
		if diff := strings.Compare(left.Name, right.Name); diff != 0 {
			return diff
		}
		return compareAssetRef(reusableAgentAssetRef(left), reusableAgentAssetRef(right))
	})

	return effective, conflicts
}

func sortedExtensionSkills(skills []Skill) []Skill {
	ordered := append([]Skill(nil), skills...)
	slices.SortFunc(ordered, func(left, right Skill) int {
		if diff := compareExtensionAssetPrecedence(
			left.ExtensionSource,
			left.ManifestPath,
			left.ResolvedPath,
			left.ExtensionName,
			right.ExtensionSource,
			right.ManifestPath,
			right.ResolvedPath,
			right.ExtensionName,
		); diff != 0 {
			return diff
		}
		if diff := strings.Compare(left.Name, right.Name); diff != 0 {
			return diff
		}
		return compareAssetRef(skillAssetRef(left), skillAssetRef(right))
	})
	return ordered
}

func sortedExtensionReusableAgents(reusableAgents []ReusableAgent) []ReusableAgent {
	ordered := append([]ReusableAgent(nil), reusableAgents...)
	slices.SortFunc(ordered, func(left, right ReusableAgent) int {
		if diff := compareExtensionAssetPrecedence(
			left.ExtensionSource,
			left.ManifestPath,
			left.ResolvedPath,
			left.ExtensionName,
			right.ExtensionSource,
			right.ManifestPath,
			right.ResolvedPath,
			right.ExtensionName,
		); diff != 0 {
			return diff
		}
		if diff := strings.Compare(left.Name, right.Name); diff != 0 {
			return diff
		}
		return compareAssetRef(reusableAgentAssetRef(left), reusableAgentAssetRef(right))
	})
	return ordered
}

func compareExtensionAssetPrecedence(
	leftSource string,
	leftManifestPath string,
	leftResolvedPath string,
	leftExtensionName string,
	rightSource string,
	rightManifestPath string,
	rightResolvedPath string,
	rightExtensionName string,
) int {
	if diff := extensionSourcePriority(rightSource) - extensionSourcePriority(leftSource); diff != 0 {
		return diff
	}
	if diff := strings.Compare(leftManifestPath, rightManifestPath); diff != 0 {
		return diff
	}
	if diff := strings.Compare(leftResolvedPath, rightResolvedPath); diff != 0 {
		return diff
	}
	return strings.Compare(leftExtensionName, rightExtensionName)
}

func extensionSourcePriority(source string) int {
	switch strings.TrimSpace(source) {
	case "workspace":
		return 3
	case "user":
		return 2
	case "bundled":
		return 1
	default:
		return 0
	}
}

func compareCatalogConflict(left, right CatalogConflict) int {
	if diff := strings.Compare(string(left.Kind), string(right.Kind)); diff != 0 {
		return diff
	}
	if diff := strings.Compare(left.Name, right.Name); diff != 0 {
		return diff
	}
	if diff := strings.Compare(string(left.Resolution), string(right.Resolution)); diff != 0 {
		return diff
	}
	if diff := compareAssetRef(left.Winner, right.Winner); diff != 0 {
		return diff
	}
	return compareAssetRef(left.Ignored, right.Ignored)
}

func compareAssetRef(left, right AssetRef) int {
	if diff := strings.Compare(left.Name, right.Name); diff != 0 {
		return diff
	}
	if diff := strings.Compare(string(left.Origin), string(right.Origin)); diff != 0 {
		return diff
	}
	if diff := strings.Compare(left.ExtensionSource, right.ExtensionSource); diff != 0 {
		return diff
	}
	if diff := strings.Compare(left.ExtensionName, right.ExtensionName); diff != 0 {
		return diff
	}
	if diff := strings.Compare(left.ManifestPath, right.ManifestPath); diff != 0 {
		return diff
	}
	return strings.Compare(left.ResolvedPath, right.ResolvedPath)
}

func skillAssetRef(skill Skill) AssetRef {
	return AssetRef{
		Origin:          skill.Origin,
		Name:            skill.Name,
		ExtensionName:   skill.ExtensionName,
		ExtensionSource: skill.ExtensionSource,
		ManifestPath:    skill.ManifestPath,
		ResolvedPath:    skill.ResolvedPath,
	}
}

func reusableAgentAssetRef(reusableAgent ReusableAgent) AssetRef {
	return AssetRef{
		Origin:          reusableAgent.Origin,
		Name:            reusableAgent.Name,
		ExtensionName:   reusableAgent.ExtensionName,
		ExtensionSource: reusableAgent.ExtensionSource,
		ManifestPath:    reusableAgent.ManifestPath,
		ResolvedPath:    reusableAgent.ResolvedPath,
	}
}

func catalogAssetKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
