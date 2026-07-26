package daemon

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	skillbundled "github.com/compozy/agh/skills"
)

const (
	promptSectionsSoulKey = "soul"
)

const (
	bundledAghSkillName         = "agh"
	bundledNetworkReference     = "references/network.md"
	bundledToolsReference       = "references/tools-and-skills.md"
	bundledNativeToolsReference = "references/native-tools.md"
	bundledTaskReference        = "references/tasks-and-orchestration.md"

	startupRuntimeIdentitySectionOrder = 10
	startupSituationSectionOrder       = 50
	startupMemorySectionOrder          = 100
	startupSoulSectionOrder            = 50
	startupSkillsSectionOrder          = 100
	startupToolsSectionOrder           = 150
	startupNetworkSectionOrder         = 200

	startupSituationSectionBudget = 20_000
	startupMemorySectionBudget    = 24_000
	startupSoulSectionBudget      = 16_000
	startupSkillsSectionBudget    = 16_000
	startupToolsSectionBudget     = 32_000
	startupNetworkSectionBudget   = 512
)

// PromptSectionPosition identifies whether a startup section renders before or
// after the base agent prompt.
type PromptSectionPosition string

const (
	// PromptSectionPositionPrepend renders the section before the base prompt.
	PromptSectionPositionPrepend PromptSectionPosition = "prepend"
	// PromptSectionPositionAppend renders the section after the base prompt.
	PromptSectionPositionAppend PromptSectionPosition = "append"
)

// PromptSectionBudgetBehavior controls how an oversized section is handled.
type PromptSectionBudgetBehavior string

const (
	// PromptSectionBudgetBehaviorTrim truncates the section to its declared budget.
	PromptSectionBudgetBehaviorTrim PromptSectionBudgetBehavior = "trim"
	// PromptSectionBudgetBehaviorOmit drops the section when it exceeds budget.
	PromptSectionBudgetBehaviorOmit PromptSectionBudgetBehavior = "omit"
)

// SectionPredicate decides whether a prompt section is eligible for one
// resolved startup policy.
type SectionPredicate func(ResolvedHarnessPolicy) bool

// StartupSectionPredicate decides whether a prompt section is eligible for the
// concrete startup context being assembled.
type StartupSectionPredicate func(session.StartupPromptContext) bool

// PromptSectionDescriptor describes one startup prompt section provider plus
// its ordering, policy eligibility, and budget behavior.
type PromptSectionDescriptor struct {
	Name             string
	Position         PromptSectionPosition
	Order            int
	Budget           int
	BudgetBehavior   PromptSectionBudgetBehavior
	Provider         session.PromptProvider
	Predicate        SectionPredicate
	StartupPredicate StartupSectionPredicate
}

func defaultStartupPromptSectionDescriptors(
	memoryProvider session.PromptProvider,
	skillsProvider session.PromptProvider,
	situationProvider session.PromptProvider,
	networkResponseGuidanceBudget ...int,
) []PromptSectionDescriptor {
	descriptors := make([]PromptSectionDescriptor, 0, 6)

	descriptors = append(descriptors, PromptSectionDescriptor{
		Name:      string(HarnessPromptSectionRuntimeIdentity),
		Position:  PromptSectionPositionPrepend,
		Order:     startupRuntimeIdentitySectionOrder,
		Provider:  aghRuntimePromptProvider{},
		Predicate: policyIncludesSection(HarnessPromptSectionRuntimeIdentity),
	})

	if situationProvider != nil {
		descriptors = append(descriptors, PromptSectionDescriptor{
			Name:           string(HarnessPromptSectionSituation),
			Position:       PromptSectionPositionPrepend,
			Order:          startupSituationSectionOrder,
			Budget:         startupSituationSectionBudget,
			BudgetBehavior: PromptSectionBudgetBehaviorOmit,
			Provider:       situationProvider,
			Predicate:      policyIncludesSection(HarnessPromptSectionSituation),
		})
	}

	if memoryProvider != nil {
		descriptors = append(descriptors, PromptSectionDescriptor{
			Name:           string(HarnessPromptSectionMemory),
			Position:       PromptSectionPositionPrepend,
			Order:          startupMemorySectionOrder,
			Budget:         startupMemorySectionBudget,
			BudgetBehavior: PromptSectionBudgetBehaviorTrim,
			Provider:       memoryProvider,
			Predicate:      policyIncludesSection(HarnessPromptSectionMemory),
		})
	}

	descriptors = append(descriptors, PromptSectionDescriptor{
		Name:             promptSectionsSoulKey,
		Position:         PromptSectionPositionAppend,
		Order:            startupSoulSectionOrder,
		Budget:           startupSoulSectionBudget,
		BudgetBehavior:   PromptSectionBudgetBehaviorTrim,
		Provider:         soulPromptSectionProvider{},
		StartupPredicate: startupHasSoulSnapshot,
	})

	if skillsProvider != nil {
		descriptors = append(descriptors, PromptSectionDescriptor{
			Name:           string(HarnessPromptSectionSkills),
			Position:       PromptSectionPositionAppend,
			Order:          startupSkillsSectionOrder,
			Budget:         startupSkillsSectionBudget,
			BudgetBehavior: PromptSectionBudgetBehaviorTrim,
			Provider:       skillsProvider,
			Predicate:      policyIncludesSection(HarnessPromptSectionSkills),
		})
	}

	descriptors = append(
		descriptors,
		defaultBundledStartupPromptSectionDescriptors(networkResponseGuidanceBudget...)...,
	)

	return descriptors
}

func defaultBundledStartupPromptSectionDescriptors(networkResponseGuidanceBudget ...int) []PromptSectionDescriptor {
	return []PromptSectionDescriptor{
		{
			Name:           string(HarnessPromptSectionTools),
			Position:       PromptSectionPositionAppend,
			Order:          startupToolsSectionOrder,
			Budget:         startupToolsSectionBudget,
			BudgetBehavior: PromptSectionBudgetBehaviorTrim,
			Provider: bundledReferencesPromptSectionProvider(
				bundledAghSkillName,
				bundledToolsReference,
				bundledNativeToolsReference,
			),
			Predicate: policyIncludesSection(HarnessPromptSectionTools),
		},
		{
			Name:           string(HarnessPromptSectionNetwork),
			Position:       PromptSectionPositionAppend,
			Order:          startupNetworkSectionOrder,
			Budget:         resolvedNetworkResponseGuidanceBudget(networkResponseGuidanceBudget...),
			BudgetBehavior: PromptSectionBudgetBehaviorTrim,
			Provider:       networkResponseRegisterPromptSectionProvider{},
			Predicate:      policyIncludesSection(HarnessPromptSectionNetwork),
		},
	}
}

func defaultStartupPromptSectionDescriptorsFromProviders(
	prependProviders []session.PromptProvider,
	appendProviders []session.PromptProvider,
	situationProvider session.PromptProvider,
	networkResponseGuidanceBudget ...int,
) []PromptSectionDescriptor {
	var memoryProvider session.PromptProvider
	for _, provider := range prependProviders {
		if provider != nil {
			memoryProvider = provider
			break
		}
	}

	var skillsProvider session.PromptProvider
	for _, provider := range appendProviders {
		if provider != nil {
			skillsProvider = provider
			break
		}
	}

	return defaultStartupPromptSectionDescriptors(
		memoryProvider,
		skillsProvider,
		situationProvider,
		networkResponseGuidanceBudget...,
	)
}

func resolvedNetworkResponseGuidanceBudget(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return startupNetworkSectionBudget
}

func policyIncludesSection(section HarnessPromptSection) SectionPredicate {
	return func(policy ResolvedHarnessPolicy) bool {
		return containsHarnessSection(policy.IncludeSections, section)
	}
}

func containsHarnessSection(sections []HarnessPromptSection, target HarnessPromptSection) bool {
	return slices.Contains(sections, target)
}

type promptSectionProviderFunc func(context.Context, *workspacepkg.ResolvedWorkspace) (string, error)

func (fn promptSectionProviderFunc) PromptSection(
	ctx context.Context,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return fn(ctx, workspace)
}

func bundledReferencesPromptSectionProvider(name string, referencePaths ...string) session.PromptProvider {
	return promptSectionProviderFunc(func(context.Context, *workspacepkg.ResolvedWorkspace) (string, error) {
		contents := make([]string, 0, len(referencePaths))
		for _, referencePath := range referencePaths {
			content, err := skillbundled.LoadResource(strings.TrimSpace(name), strings.TrimSpace(referencePath))
			if err != nil {
				return "", fmt.Errorf(
					"daemon: load bundled startup section %q file %q: %w",
					name,
					referencePath,
					err,
				)
			}
			contents = append(contents, strings.TrimSpace(content))
		}
		return strings.Join(contents, "\n\n"), nil
	})
}
