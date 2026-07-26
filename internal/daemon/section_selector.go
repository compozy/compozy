package daemon

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/session"
)

// SectionSelector resolves startup policy and selects eligible startup prompt
// sections before final assembly.
type SectionSelector struct {
	resolver *HarnessContextResolver
	recorder *harnessLifecycleRecorder
}

// NewSectionSelector constructs a daemon-owned startup section selector.
func NewSectionSelector(
	resolver *HarnessContextResolver,
	recorder *harnessLifecycleRecorder,
) *SectionSelector {
	if resolver == nil {
		return nil
	}
	return &SectionSelector{resolver: resolver, recorder: recorder}
}

// Select resolves startup policy for the provided startup context and returns
// the eligible startup sections in deterministic order without duplicates.
func (s *SectionSelector) Select(
	startup session.StartupPromptContext,
	descriptors []PromptSectionDescriptor,
) ([]PromptSectionDescriptor, ResolvedHarnessContext, error) {
	normalized := normalizeAndSortPromptSectionDescriptors(descriptors)
	resolved := ResolvedHarnessContext{}
	if s != nil && s.resolver != nil {
		var err error
		resolved, err = s.resolver.ResolveStartup(startup)
		if err != nil {
			return nil, ResolvedHarnessContext{}, err
		}
	}
	var timestamp time.Time
	if s != nil && s.recorder != nil && s.resolver != nil {
		timestamp = s.recorder.timestamp(time.Time{})
		s.recorder.RecordStartupContextResolved(startup, resolved, timestamp)
	}

	selected := make([]PromptSectionDescriptor, 0, len(normalized))
	seen := make(map[string]struct{}, len(normalized))

	for _, descriptor := range normalized {
		if descriptor.Provider == nil {
			continue
		}
		if descriptor.StartupPredicate != nil && !descriptor.StartupPredicate(startup) {
			continue
		}
		if s != nil && s.resolver != nil && descriptor.Predicate != nil && !descriptor.Predicate(resolved.Policy) {
			continue
		}
		if _, exists := seen[descriptor.Name]; exists {
			continue
		}
		seen[descriptor.Name] = struct{}{}
		selected = append(selected, descriptor)
	}
	if s != nil && s.recorder != nil && s.resolver != nil {
		s.recorder.RecordStartupSectionSelected(startup, resolved, selected, timestamp)
	}

	return selected, resolved, nil
}

func normalizeAndSortPromptSectionDescriptors(
	descriptors []PromptSectionDescriptor,
) []PromptSectionDescriptor {
	normalized := make([]PromptSectionDescriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		name := strings.TrimSpace(descriptor.Name)
		if name == "" {
			continue
		}

		position, ok := normalizePromptSectionPosition(descriptor.Position)
		if !ok {
			continue
		}

		normalized = append(normalized, PromptSectionDescriptor{
			Name:             name,
			Position:         position,
			Order:            descriptor.Order,
			Budget:           descriptor.Budget,
			BudgetBehavior:   normalizePromptSectionBudgetBehavior(descriptor.BudgetBehavior),
			Provider:         descriptor.Provider,
			Predicate:        descriptor.Predicate,
			StartupPredicate: descriptor.StartupPredicate,
		})
	}

	slices.SortStableFunc(normalized, func(left, right PromptSectionDescriptor) int {
		if cmp := cmp.Compare(
			promptSectionPositionRank(left.Position),
			promptSectionPositionRank(right.Position),
		); cmp != 0 {
			return cmp
		}
		if cmp := cmp.Compare(left.Order, right.Order); cmp != 0 {
			return cmp
		}
		return cmp.Compare(left.Name, right.Name)
	})

	return normalized
}

func normalizePromptSectionPosition(position PromptSectionPosition) (PromptSectionPosition, bool) {
	switch PromptSectionPosition(strings.TrimSpace(string(position))) {
	case PromptSectionPositionPrepend:
		return PromptSectionPositionPrepend, true
	case "", PromptSectionPositionAppend:
		return PromptSectionPositionAppend, true
	default:
		return "", false
	}
}

func normalizePromptSectionBudgetBehavior(
	behavior PromptSectionBudgetBehavior,
) PromptSectionBudgetBehavior {
	switch PromptSectionBudgetBehavior(strings.TrimSpace(string(behavior))) {
	case PromptSectionBudgetBehaviorOmit:
		return PromptSectionBudgetBehaviorOmit
	case "", PromptSectionBudgetBehaviorTrim:
		return PromptSectionBudgetBehaviorTrim
	default:
		return PromptSectionBudgetBehaviorTrim
	}
}

func promptSectionPositionRank(position PromptSectionPosition) int {
	switch position {
	case PromptSectionPositionPrepend:
		return 0
	case PromptSectionPositionAppend:
		return 1
	default:
		return 2
	}
}

func validatePromptSectionDescriptors(descriptors []PromptSectionDescriptor) error {
	for _, descriptor := range descriptors {
		if strings.TrimSpace(descriptor.Name) == "" {
			return fmt.Errorf("daemon: startup prompt section name is required")
		}
		if _, ok := normalizePromptSectionPosition(descriptor.Position); !ok {
			return fmt.Errorf(
				"daemon: invalid startup prompt section position %q for %q",
				descriptor.Position,
				descriptor.Name,
			)
		}
	}
	return nil
}
