package resources

import (
	"fmt"
	"slices"
)

var scopeDownwardClosures = map[ResourceScopeKind][]ResourceScopeKind{
	ResourceScopeKindUser: {
		ResourceScopeKindUser,
		ResourceScopeKindWorkspace,
		ResourceScopeKindProfile,
		ResourceScopeKindWorkspaceProfile,
	},
	ResourceScopeKindWorkspace: {
		ResourceScopeKindWorkspace,
		ResourceScopeKindWorkspaceProfile,
	},
	ResourceScopeKindProfile: {
		ResourceScopeKindProfile,
		ResourceScopeKindWorkspaceProfile,
	},
	ResourceScopeKindWorkspaceProfile: {
		ResourceScopeKindWorkspaceProfile,
	},
}

// ScopesThrough returns the downward closure for one resource-scope ceiling.
func ScopesThrough(ceiling ResourceScopeKind) []ResourceScopeKind {
	closure, ok := scopeDownwardClosures[ceiling.Normalize()]
	if !ok {
		return nil
	}
	return slices.Clone(closure)
}

// MeetScopeCeilings narrows ceilings without widening either scope axis.
func MeetScopeCeilings(ceilings ...ResourceScopeKind) (ResourceScopeKind, error) {
	var intersection []ResourceScopeKind
	seen := false
	for _, ceiling := range ceilings {
		normalized := ceiling.Normalize()
		if normalized == "" {
			continue
		}
		if err := normalized.Validate("resource scope ceiling"); err != nil {
			return "", err
		}

		closure := scopeDownwardClosures[normalized]
		if !seen {
			intersection = slices.Clone(closure)
			seen = true
			continue
		}
		intersection = intersectScopeClosures(intersection, closure)
	}
	if !seen {
		return "", nil
	}

	for candidate, closure := range scopeDownwardClosures {
		if slices.Equal(closure, intersection) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%w: resource scope ceilings have no meet", ErrValidation)
}

func intersectScopeClosures(left, right []ResourceScopeKind) []ResourceScopeKind {
	intersection := make([]ResourceScopeKind, 0, min(len(left), len(right)))
	for _, scope := range left {
		if slices.Contains(right, scope) {
			intersection = append(intersection, scope)
		}
	}
	return intersection
}
