package daemon

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/session"
)

const (
	durableMemoryAugmenterOrder = 100
	skillsAugmenterOrder        = 150
	situationAugmenterOrder     = 200
	situationAugmenterBudget    = 20_000
)

type promptInputAugmenterBudgetBehavior string

const (
	promptInputAugmenterBudgetBehaviorTrim promptInputAugmenterBudgetBehavior = "trim"
	promptInputAugmenterBudgetBehaviorOmit promptInputAugmenterBudgetBehavior = "omit"
)

type promptInputAugmenterResolver interface {
	ResolvePrompt(
		info *session.Info,
		source session.TurnSource,
		meta acp.PromptMeta,
	) (ResolvedHarnessContext, error)
}

type promptInputAugmenterDescriptor struct {
	Name           HarnessAugmenter
	Order          int
	Budget         int
	BudgetBehavior promptInputAugmenterBudgetBehavior
	Critical       bool
	Augmenter      session.PromptInputAugmenter
}

type promptInputComposite struct {
	logger      *slog.Logger
	resolver    promptInputAugmenterResolver
	recorder    *harnessLifecycleRecorder
	descriptors []promptInputAugmenterDescriptor
	byName      map[HarnessAugmenter]promptInputAugmenterDescriptor
}

func defaultPromptInputAugmenterDescriptors(
	durableMemory session.PromptInputAugmenter,
	skillsCatalog session.PromptInputAugmenter,
	situationAugmenters ...session.PromptInputAugmenter,
) []promptInputAugmenterDescriptor {
	descriptors := make([]promptInputAugmenterDescriptor, 0, 3)
	if durableMemory != nil {
		descriptors = append(descriptors, promptInputAugmenterDescriptor{
			Name:           HarnessAugmenterDurableMemory,
			Order:          durableMemoryAugmenterOrder,
			Budget:         memory.RecallAugmenterBudget,
			BudgetBehavior: promptInputAugmenterBudgetBehaviorOmit,
			Critical:       false,
			Augmenter:      durableMemory,
		})
	}
	if skillsCatalog != nil {
		descriptors = append(descriptors, promptInputAugmenterDescriptor{
			Name:           HarnessAugmenterSkills,
			Order:          skillsAugmenterOrder,
			Budget:         startupSkillsSectionBudget,
			BudgetBehavior: promptInputAugmenterBudgetBehaviorTrim,
			Critical:       false,
			Augmenter:      skillsCatalog,
		})
	}
	if len(situationAugmenters) > 0 && situationAugmenters[0] != nil {
		descriptors = append(descriptors, promptInputAugmenterDescriptor{
			Name:           HarnessAugmenterSituation,
			Order:          situationAugmenterOrder,
			Budget:         situationAugmenterBudget,
			BudgetBehavior: promptInputAugmenterBudgetBehaviorOmit,
			Critical:       false,
			Augmenter:      situationAugmenters[0],
		})
	}
	return descriptors
}

func newPromptInputCompositeAugmenter(
	logger *slog.Logger,
	resolver promptInputAugmenterResolver,
	recorder *harnessLifecycleRecorder,
	descriptors ...promptInputAugmenterDescriptor,
) (session.PromptInputAugmenter, error) {
	if resolver == nil {
		return nil, nil
	}

	normalized, err := normalizePromptInputAugmenterDescriptors(descriptors)
	if err != nil {
		return nil, err
	}

	composite := &promptInputComposite{
		logger:      logger,
		resolver:    resolver,
		recorder:    recorder,
		descriptors: normalized,
		byName:      make(map[HarnessAugmenter]promptInputAugmenterDescriptor, len(normalized)),
	}
	for _, descriptor := range normalized {
		composite.byName[descriptor.Name] = descriptor
	}
	return composite.Augment, nil
}

func normalizePromptInputAugmenterDescriptors(
	descriptors []promptInputAugmenterDescriptor,
) ([]promptInputAugmenterDescriptor, error) {
	if len(descriptors) == 0 {
		return nil, nil
	}

	normalized := make([]promptInputAugmenterDescriptor, 0, len(descriptors))
	seen := make(map[HarnessAugmenter]struct{}, len(descriptors))
	for _, descriptor := range descriptors {
		name := HarnessAugmenter(strings.TrimSpace(string(descriptor.Name)))
		if name == "" {
			return nil, errorsPromptInputDescriptor("name is required")
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("daemon: duplicate prompt input augmenter descriptor %q", name)
		}
		if descriptor.Augmenter == nil {
			return nil, fmt.Errorf("daemon: prompt input augmenter %q is missing an augmenter", name)
		}

		normalized = append(normalized, promptInputAugmenterDescriptor{
			Name:           name,
			Order:          descriptor.Order,
			Budget:         max(descriptor.Budget, 0),
			BudgetBehavior: normalizePromptInputAugmenterBudgetBehavior(descriptor.BudgetBehavior),
			Critical:       descriptor.Critical,
			Augmenter:      descriptor.Augmenter,
		})
		seen[name] = struct{}{}
	}

	slices.SortStableFunc(normalized, func(left, right promptInputAugmenterDescriptor) int {
		if left.Order != right.Order {
			return left.Order - right.Order
		}
		return strings.Compare(string(left.Name), string(right.Name))
	})
	return normalized, nil
}

func errorsPromptInputDescriptor(detail string) error {
	return fmt.Errorf("daemon: prompt input augmenter descriptor %s", detail)
}

func normalizePromptInputAugmenterBudgetBehavior(
	behavior promptInputAugmenterBudgetBehavior,
) promptInputAugmenterBudgetBehavior {
	switch promptInputAugmenterBudgetBehavior(strings.TrimSpace(string(behavior))) {
	case promptInputAugmenterBudgetBehaviorOmit:
		return promptInputAugmenterBudgetBehaviorOmit
	case "", promptInputAugmenterBudgetBehaviorTrim:
		return promptInputAugmenterBudgetBehaviorTrim
	default:
		return promptInputAugmenterBudgetBehaviorTrim
	}
}
