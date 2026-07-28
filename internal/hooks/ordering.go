package hooks

import (
	"errors"
	"fmt"
	"sort"
)

var ErrInvalidHookSource = errors.New("hooks: invalid hook source")

// DefaultHookPriority returns the documented default priority for the source.
func DefaultHookPriority(source HookSource) (int32, error) {
	switch source {
	case HookSourceNative:
		return 1000, nil
	case HookSourceConfig:
		return 500, nil
	case HookSourceAgentDefinition:
		return 100, nil
	case HookSourceSkill:
		return 0, nil
	default:
		return 0, fmt.Errorf("%w: %d", ErrInvalidHookSource, source)
	}
}

// SortResolvedHooks sorts the slice in place using deterministic dispatch
// precedence.
func SortResolvedHooks(hooks []*ResolvedHook) {
	sort.SliceStable(hooks, func(i, j int) bool {
		return resolvedHookLess(hooks[i], hooks[j])
	})
}

// OrderedResolvedHooks returns a sorted copy of the slice.
func OrderedResolvedHooks(hooks []*ResolvedHook) []*ResolvedHook {
	ordered := append([]*ResolvedHook(nil), hooks...)
	SortResolvedHooks(ordered)
	return ordered
}

func orderedResolvedHooksIfNeeded(hooks []*ResolvedHook) []*ResolvedHook {
	if len(hooks) < 2 {
		return hooks
	}

	for idx := 1; idx < len(hooks); idx++ {
		if resolvedHookLess(hooks[idx], hooks[idx-1]) {
			return OrderedResolvedHooks(hooks)
		}
	}

	return hooks
}

func resolvedHookLess(left *ResolvedHook, right *ResolvedHook) bool {
	if left == nil || right == nil {
		return left != nil && right == nil
	}

	if left.Source != right.Source {
		return left.Source < right.Source
	}
	if left.Priority != right.Priority {
		return left.Priority > right.Priority
	}
	if left.Source == HookSourceSkill && left.Decl.SkillSource != right.Decl.SkillSource {
		return hookSkillSourceRank(left.Decl.SkillSource) < hookSkillSourceRank(right.Decl.SkillSource)
	}
	if left.Name != right.Name {
		return left.Name < right.Name
	}

	return false
}

func hookSkillSourceRank(source HookSkillSource) int {
	switch source {
	case HookSkillSourceBundled:
		return 0
	case HookSkillSourceMarketplace:
		return 1
	case HookSkillSourceUser:
		return 2
	case HookSkillSourceAdditional:
		return 3
	case HookSkillSourceWorkspace:
		return 4
	default:
		return 5
	}
}
