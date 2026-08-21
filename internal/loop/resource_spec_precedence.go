package loop

import (
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/resources"
)

// ResourceLens selects the four resource layers visible to one active profile
// inside one workspace.
type ResourceLens struct {
	WorkspaceID string
	ProfileID   string
	ProfileName string
}

// Normalize returns the canonical resource selection lens.
func (l ResourceLens) Normalize() ResourceLens {
	return ResourceLens{
		WorkspaceID: strings.TrimSpace(l.WorkspaceID),
		ProfileID:   strings.TrimSpace(l.ProfileID),
		ProfileName: strings.TrimSpace(l.ProfileName),
	}
}

// ResolveEffectiveResources overlays records for one read scope using loop precedence.
func ResolveEffectiveResources(
	records []resources.Record[ResourceSpec],
	lens ResourceLens,
) []resources.Record[ResourceSpec] {
	lens = lens.Normalize()
	effective := map[string]resources.Record[ResourceSpec]{}
	for _, record := range records {
		scope := record.Scope.Normalize()
		if !loopResourceScopeMatches(scope, lens) {
			continue
		}
		name := strings.TrimSpace(record.Spec.Name)
		if name == "" {
			continue
		}
		current, exists := effective[name]
		if !exists || compareLoopRecordPrecedence(record, current) > 0 {
			effective[name] = cloneLoopRecord(record)
		}
	}
	resolved := make([]resources.Record[ResourceSpec], 0, len(effective))
	for _, name := range sortedKeys(effective) {
		resolved = append(resolved, effective[name])
	}
	return resolved
}

func loopResourceScopeMatches(scope resources.ResourceScope, lens ResourceLens) bool {
	switch scope.Kind {
	case resources.ResourceScopeKindUser:
		return true
	case resources.ResourceScopeKindProfile:
		return lens.ProfileID != "" && scope.ID == lens.ProfileID
	case resources.ResourceScopeKindWorkspace:
		return lens.WorkspaceID != "" && scope.ID == lens.WorkspaceID
	case resources.ResourceScopeKindWorkspaceProfile:
		return lens.WorkspaceID != "" && lens.ProfileName != "" &&
			scope.ID == lens.WorkspaceID+"@pf:"+lens.ProfileName
	default:
		return false
	}
}

func compareLoopRecordPrecedence(
	left resources.Record[ResourceSpec],
	right resources.Record[ResourceSpec],
) int {
	leftRank := loopRecordPrecedenceRank(left)
	rightRank := loopRecordPrecedenceRank(right)
	if leftRank != rightRank {
		return leftRank - rightRank
	}
	if left.Version != right.Version {
		if left.Version > right.Version {
			return 1
		}
		return -1
	}
	return strings.Compare(left.ID, right.ID)
}

func loopRecordPrecedenceRank(record resources.Record[ResourceSpec]) int {
	rank := record.Spec.Source.PrecedenceRank()
	switch record.Scope.Normalize().Kind {
	case resources.ResourceScopeKindProfile:
		rank += 100
	case resources.ResourceScopeKindWorkspace:
		rank += 200
	case resources.ResourceScopeKindWorkspaceProfile:
		rank += 300
	}
	return rank
}

func cloneLoopRecord(record resources.Record[ResourceSpec]) resources.Record[ResourceSpec] {
	record.Spec = CloneResourceSpec(record.Spec)
	return record
}

func sortedKeys[T any](values map[string]T) []string {
	return slices.Sorted(maps.Keys(values))
}
