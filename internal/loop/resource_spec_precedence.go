package loop

import (
	"slices"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

// ResolveEffectiveResources overlays records for one read scope using loop precedence.
func ResolveEffectiveResources(
	records []resources.Record[ResourceSpec],
	workspaceID string,
) []resources.Record[ResourceSpec] {
	effective := map[string]resources.Record[ResourceSpec]{}
	for _, record := range records {
		scope := record.Scope.Normalize()
		if scope.Kind == resources.ResourceScopeKindWorkspace && scope.ID != strings.TrimSpace(workspaceID) {
			continue
		}
		if scope.Kind != resources.ResourceScopeKindGlobal && scope.Kind != resources.ResourceScopeKindWorkspace {
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
	if record.Scope.Normalize().Kind == resources.ResourceScopeKindWorkspace {
		rank += 100
	}
	return rank
}

func cloneLoopRecord(record resources.Record[ResourceSpec]) resources.Record[ResourceSpec] {
	record.Spec = CloneResourceSpec(record.Spec)
	return record
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
