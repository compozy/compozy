package skills

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"
)

// ActivationContextProvider resolves dynamic offer-time inputs for one catalog projection.
type ActivationContextProvider func(context.Context, ActivationTarget) (ActivationContext, error)

// ActivationTarget identifies the runtime scope whose skill catalog is being projected.
type ActivationTarget struct {
	WorkspaceID       string
	SessionID         string
	AgentName         string
	Capabilities      []string
	NeedsPlatform     bool
	NeedsEnvironments bool
	NeedsTools        bool
}

// ActivationContext is the authoritative runtime input used to evaluate activation gates.
type ActivationContext struct {
	Platform     string
	Environments []string
	Tools        []string
	Capabilities []string
}

// WithActivationContextProvider injects late-bound runtime activation inputs.
func WithActivationContextProvider(provider ActivationContextProvider) Option {
	return func(registry *Registry) {
		registry.activationContext = provider
	}
}

func (r *Registry) projectSkillActivation(
	ctx context.Context,
	skills []*Skill,
	target ActivationTarget,
) ([]*Skill, error) {
	activation := ActivationContext{
		Platform:     runtime.GOOS,
		Capabilities: slices.Clone(target.Capabilities),
	}
	target.NeedsPlatform, target.NeedsEnvironments, target.NeedsTools = activationContextNeeds(skills)
	if r != nil && r.activationContext != nil &&
		(target.NeedsPlatform || target.NeedsEnvironments || target.NeedsTools) {
		resolved, err := r.activationContext(ctx, target)
		if err != nil {
			return nil, fmt.Errorf("skills: resolve activation context: %w", err)
		}
		if strings.TrimSpace(resolved.Platform) != "" {
			activation.Platform = resolved.Platform
		}
		activation.Environments = slices.Clone(resolved.Environments)
		activation.Tools = slices.Clone(resolved.Tools)
	}

	for _, skill := range skills {
		evaluateSkillActivation(skill, activation)
	}
	return skills, nil
}

func activationContextNeeds(skills []*Skill) (platform bool, environments bool, tools bool) {
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		platform = platform || len(skill.ActivationGates.Platforms) > 0
		environments = environments || len(skill.ActivationGates.Environments) > 0
		tools = tools || len(skill.ActivationGates.RequiresTools) > 0
	}
	return platform, environments, tools
}

func evaluateSkillActivation(skill *Skill, activation ActivationContext) {
	if skill == nil {
		return
	}

	reasons := make([]ActivationReason, 0, 4)
	if required := skill.ActivationGates.Platforms; len(required) > 0 &&
		!containsAnyFold(required, []string{activation.Platform}) {
		reasons = append(reasons, newActivationReason(
			ActivationGatePlatforms,
			ActivationReasonPlatformMismatch,
			required,
		))
	}
	if required := skill.ActivationGates.Environments; len(required) > 0 {
		switch {
		case activation.Environments == nil:
			reasons = append(reasons, newActivationReason(
				ActivationGateEnvironments,
				ActivationReasonEnvironmentContextUnavailable,
				required,
			))
		case !containsAnyFold(required, activation.Environments):
			reasons = append(reasons, newActivationReason(
				ActivationGateEnvironments,
				ActivationReasonEnvironmentMismatch,
				required,
			))
		}
	}
	if required := skill.ActivationGates.RequiresTools; len(required) > 0 {
		code := ActivationReasonMissingTool
		missing := missingFold(required, activation.Tools)
		if activation.Tools == nil {
			code = ActivationReasonToolContextUnavailable
			missing = slices.Clone(required)
		}
		if len(missing) > 0 {
			reasons = append(reasons, newActivationReason(ActivationGateRequiresTools, code, missing))
		}
	}
	if required := skill.ActivationGates.RequiresCapabilities; len(required) > 0 {
		code := ActivationReasonMissingCapability
		missing := missingFold(required, activation.Capabilities)
		if activation.Capabilities == nil {
			code = ActivationReasonCapabilityContextUnavailable
			missing = slices.Clone(required)
		}
		if len(missing) > 0 {
			reasons = append(reasons, newActivationReason(ActivationGateRequiresCapabilities, code, missing))
		}
	}

	skill.Activation = SkillActivation{
		Active:    len(reasons) == 0,
		Evaluated: true,
		Reasons:   reasons,
	}
}

func skillIsActive(skill *Skill) bool {
	return skill != nil && (!skill.Activation.Evaluated || skill.Activation.Active)
}

func newActivationReason(gate ActivationGate, code ActivationReasonCode, missing []string) ActivationReason {
	cloned := slices.Clone(missing)
	return ActivationReason{
		Gate:    gate,
		Code:    code,
		Missing: cloned,
		Message: fmt.Sprintf("gate %s unmet: %s", gate, strings.Join(cloned, ", ")),
	}
}

func containsAnyFold(required []string, available []string) bool {
	availableSet := normalizedActivationSet(available)
	for _, value := range required {
		if _, ok := availableSet[strings.ToLower(strings.TrimSpace(value))]; ok {
			return true
		}
	}
	return false
}

func missingFold(required []string, available []string) []string {
	availableSet := normalizedActivationSet(available)
	missing := make([]string, 0, len(required))
	for _, value := range required {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if _, ok := availableSet[normalized]; !ok {
			missing = append(missing, normalized)
		}
	}
	return missing
}

func normalizedActivationSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	return set
}

func cloneSkillActivation(src SkillActivation) SkillActivation {
	clone := src
	if len(src.Reasons) == 0 {
		clone.Reasons = nil
		return clone
	}
	clone.Reasons = make([]ActivationReason, 0, len(src.Reasons))
	for _, reason := range src.Reasons {
		cloned := reason
		cloned.Missing = slices.Clone(reason.Missing)
		clone.Reasons = append(clone.Reasons, cloned)
	}
	return clone
}
