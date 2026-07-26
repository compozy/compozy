package daemon

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// ComposedAssembler assembles selected startup prompt sections around the base
// agent prompt.
type ComposedAssembler struct {
	selector    *SectionSelector
	descriptors []PromptSectionDescriptor
}

// ComposedAssemblerOption customizes the prompt section chain for a
// ComposedAssembler.
type ComposedAssemblerOption func(*ComposedAssembler)

type startupPromptSectionProvider interface {
	PromptStartupSection(
		ctx context.Context,
		startup session.StartupPromptContext,
		agent aghconfig.AgentDef,
		workspace *workspacepkg.ResolvedWorkspace,
	) (string, error)
}

type agentPromptSectionProvider interface {
	PromptAgentSection(
		ctx context.Context,
		agent aghconfig.AgentDef,
		workspace *workspacepkg.ResolvedWorkspace,
	) (string, error)
}

type agentSessionPromptSectionProvider interface {
	PromptAgentSessionSection(
		ctx context.Context,
		sessionID string,
		agent aghconfig.AgentDef,
		workspace *workspacepkg.ResolvedWorkspace,
	) (string, error)
}

var (
	_ session.PromptAssembler        = (*ComposedAssembler)(nil)
	_ session.StartupPromptAssembler = (*ComposedAssembler)(nil)
	_ session.ResumeContextProvider  = (*ComposedAssembler)(nil)
)

// NewComposedAssembler constructs a ComposedAssembler from startup section
// descriptors.
func NewComposedAssembler(opts ...ComposedAssemblerOption) *ComposedAssembler {
	assembler := &ComposedAssembler{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(assembler)
	}
	return assembler
}

// WithSectionSelector installs the daemon-owned startup section selector.
func WithSectionSelector(selector *SectionSelector) ComposedAssemblerOption {
	return func(assembler *ComposedAssembler) {
		if assembler == nil {
			return
		}
		assembler.selector = selector
	}
}

// WithPromptSectionDescriptors appends explicit startup prompt section
// descriptors to the assembler.
func WithPromptSectionDescriptors(descriptors ...PromptSectionDescriptor) ComposedAssemblerOption {
	copied := append([]PromptSectionDescriptor(nil), descriptors...)
	return func(assembler *ComposedAssembler) {
		if assembler == nil || len(copied) == 0 {
			return
		}
		assembler.descriptors = append(assembler.descriptors, copied...)
	}
}

// WithPrependPromptProviders preserves the legacy prepend provider option by
// wrapping providers in explicit prepend descriptors.
func WithPrependPromptProviders(providers ...session.PromptProvider) ComposedAssemblerOption {
	copied := append([]session.PromptProvider(nil), providers...)
	return func(assembler *ComposedAssembler) {
		if assembler == nil || len(copied) == 0 {
			return
		}
		baseOrder := len(assembler.descriptors) * 10
		for idx, provider := range copied {
			if provider == nil {
				continue
			}
			assembler.descriptors = append(assembler.descriptors, PromptSectionDescriptor{
				Name:     fmt.Sprintf("legacy-prepend-%d", baseOrder+idx),
				Position: PromptSectionPositionPrepend,
				Order:    baseOrder + idx,
				Provider: provider,
			})
		}
	}
}

// WithAppendPromptProviders preserves the legacy append provider option by
// wrapping providers in explicit append descriptors.
func WithAppendPromptProviders(providers ...session.PromptProvider) ComposedAssemblerOption {
	copied := append([]session.PromptProvider(nil), providers...)
	return func(assembler *ComposedAssembler) {
		if assembler == nil || len(copied) == 0 {
			return
		}
		baseOrder := len(assembler.descriptors) * 10
		for idx, provider := range copied {
			if provider == nil {
				continue
			}
			assembler.descriptors = append(assembler.descriptors, PromptSectionDescriptor{
				Name:     fmt.Sprintf("legacy-append-%d", baseOrder+idx),
				Position: PromptSectionPositionAppend,
				Order:    baseOrder + idx,
				Provider: provider,
			})
		}
	}
}

// Assemble renders the selected startup sections using a baseline startup
// context for callers that only know about the legacy assembler seam.
func (a *ComposedAssembler) Assemble(
	ctx context.Context,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	return a.AssembleStartup(ctx, session.StartupPromptContext{
		AgentName:   strings.TrimSpace(agent.Name),
		SessionType: session.SessionTypeUser,
	}, agent, workspace)
}

// AssembleStartup renders eligible prepend sections, the trimmed base prompt,
// and eligible append sections into one composed startup system prompt.
func (a *ComposedAssembler) AssembleStartup(
	ctx context.Context,
	startup session.StartupPromptContext,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	basePrompt := strings.TrimSpace(agent.Prompt)
	if a == nil {
		return basePrompt, nil
	}

	selected, err := a.selectDescriptors(startup)
	if err != nil {
		return "", err
	}

	prependSections, appendSections, err := gatherPromptSections(ctx, startup, agent, workspace, selected)
	if err != nil {
		return "", err
	}

	sections := make([]string, 0, len(prependSections)+len(appendSections)+1)
	sections = append(sections, prependSections...)
	if basePrompt != "" {
		sections = append(sections, basePrompt)
	}
	sections = append(sections, appendSections...)

	return strings.Join(sections, "\n\n"), nil
}

// ResumeContextSection gathers resume-only sections from the same selected
// startup providers used by normal prompt assembly.
func (a *ComposedAssembler) ResumeContextSection(
	ctx context.Context,
	startup session.StartupPromptContext,
) (string, error) {
	if a == nil {
		return "", nil
	}
	selected, err := a.selectDescriptors(startup)
	if err != nil {
		return "", err
	}
	sections := make([]string, 0, len(selected))
	for _, descriptor := range selected {
		provider, ok := descriptor.Provider.(session.ResumeContextProvider)
		if !ok || provider == nil {
			continue
		}
		section, err := provider.ResumeContextSection(ctx, startup)
		if err != nil {
			return "", fmt.Errorf("daemon: resume prompt section %q: %w", descriptor.Name, err)
		}
		if trimmed := strings.TrimSpace(section); trimmed != "" {
			sections = append(sections, trimmed)
		}
	}
	return strings.Join(sections, "\n\n"), nil
}

func (a *ComposedAssembler) selectDescriptors(
	startup session.StartupPromptContext,
) ([]PromptSectionDescriptor, error) {
	if len(a.descriptors) == 0 {
		return nil, nil
	}
	if err := validatePromptSectionDescriptors(a.descriptors); err != nil {
		return nil, err
	}
	if a.selector == nil {
		return filterPromptDescriptorsForStartup(normalizeAndSortPromptSectionDescriptors(a.descriptors), startup), nil
	}
	selected, _, err := a.selector.Select(startup, a.descriptors)
	return selected, err
}

func filterPromptDescriptorsForStartup(
	descriptors []PromptSectionDescriptor,
	startup session.StartupPromptContext,
) []PromptSectionDescriptor {
	if len(descriptors) == 0 {
		return nil
	}
	filtered := make([]PromptSectionDescriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if descriptor.StartupPredicate != nil && !descriptor.StartupPredicate(startup) {
			continue
		}
		filtered = append(filtered, descriptor)
	}
	return filtered
}

func gatherPromptSections(
	ctx context.Context,
	startup session.StartupPromptContext,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
	descriptors []PromptSectionDescriptor,
) ([]string, []string, error) {
	prependSections := make([]string, 0, len(descriptors))
	appendSections := make([]string, 0, len(descriptors))

	for _, descriptor := range descriptors {
		if descriptor.Provider == nil {
			continue
		}

		section, err := promptSection(ctx, descriptor.Provider, startup, agent, workspace)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"daemon: %s prompt section %q: %w",
				descriptor.Position,
				descriptor.Name,
				err,
			)
		}

		section = applyPromptSectionBudget(strings.TrimSpace(section), descriptor)
		if section == "" {
			continue
		}

		switch descriptor.Position {
		case PromptSectionPositionPrepend:
			prependSections = append(prependSections, section)
		case PromptSectionPositionAppend:
			appendSections = append(appendSections, section)
		default:
			return nil, nil, fmt.Errorf(
				"daemon: invalid prompt section position %q for %q",
				descriptor.Position,
				descriptor.Name,
			)
		}
	}

	return prependSections, appendSections, nil
}

func promptSection(
	ctx context.Context,
	provider session.PromptProvider,
	startup session.StartupPromptContext,
	agent aghconfig.AgentDef,
	workspace *workspacepkg.ResolvedWorkspace,
) (string, error) {
	if sessionProvider, ok := provider.(agentSessionPromptSectionProvider); ok {
		return sessionProvider.PromptAgentSessionSection(ctx, startup.SessionID, agent, workspace)
	}
	if startupProvider, ok := provider.(startupPromptSectionProvider); ok {
		return startupProvider.PromptStartupSection(ctx, startup, agent, workspace)
	}
	if agentProvider, ok := provider.(agentPromptSectionProvider); ok {
		return agentProvider.PromptAgentSection(ctx, agent, workspace)
	}
	return provider.PromptSection(ctx, workspace)
}

func applyPromptSectionBudget(section string, descriptor PromptSectionDescriptor) string {
	if section == "" || descriptor.Budget <= 0 {
		return section
	}
	if utf8.RuneCountInString(section) <= descriptor.Budget {
		return section
	}
	if descriptor.BudgetBehavior == PromptSectionBudgetBehaviorOmit {
		return ""
	}
	return strings.TrimSpace(trimStringToRunes(section, descriptor.Budget))
}

func trimStringToRunes(value string, budget int) string {
	if budget <= 0 || value == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(value))

	count := 0
	for _, r := range value {
		if count == budget {
			break
		}
		builder.WriteRune(r)
		count++
	}
	return builder.String()
}
