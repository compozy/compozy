package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/modelcatalog"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

// ResolveItemRuntime resolves one worker item using the ADR-001 field precedence.
func ResolveItemRuntime(layers RuntimeLayers, item ItemRuntime) (ResolvedRuntime, error) {
	if err := ValidateRuntimeRules(layers.ConfigRules); err != nil {
		return ResolvedRuntime{}, err
	}
	if err := ValidateRuntimeRules(layers.RunRules); err != nil {
		return ResolvedRuntime{}, err
	}
	resolved := ResolvedRuntime{}
	applyRuntime(&resolved, layers.Defaults, RuntimeSourceDefault)
	applyRuntime(&resolved, item.Node, RuntimeSourceNode)
	applyRuntime(&resolved, resolveMatchingRuntime(layers.ConfigRules, item), RuntimeSourceConfig)
	applyRuntime(&resolved, item.Input, RuntimeSourceInput)
	applyRuntime(&resolved, item.Frontmatter, RuntimeSourceFrontmatter)
	applyRuntime(&resolved, resolveMatchingRuntime(layers.RunRules, item), RuntimeSourceRun)
	return normalizeResolvedRuntime(resolved), nil
}

// ResolveJudgeRuntime merges judge defaults with one criterion runtime.
func ResolveJudgeRuntime(defaults RuntimeSpec, criterion RuntimeSpec) ResolvedRuntime {
	resolved := ResolvedRuntime{}
	applyRuntime(&resolved, defaults, RuntimeSourceDefault)
	applyRuntime(&resolved, criterion, RuntimeSourceCriterion)
	return normalizeResolvedRuntime(resolved)
}

// ValidateRuntimeRules enforces exclusive selectors and non-empty runtime values.
func ValidateRuntimeRules(rules []RuntimeRule) error {
	return validateRuntimeRules(context.Background(), nil, rules)
}

// ValidateResolvedRuntime validates canonical runtime intent before a session bind.
func ValidateResolvedRuntime(
	ctx context.Context,
	catalog RuntimeCatalog,
	taskID string,
	resolved ResolvedRuntime,
) (ResolvedRuntime, error) {
	resolved = normalizeResolvedRuntime(resolved)
	if reasoning := resolved.Runtime.Reasoning; reasoning != "" && !modelcatalog.IsValidEffort(reasoning) {
		return ResolvedRuntime{}, NewRuntimeValidationError(RuntimeValidationItem{
			TaskID: taskID,
			Field:  runtimeFieldReasoning,
			Value:  reasoning,
			Reason: "unsupported_reasoning",
		})
	}
	if requested := resolved.Runtime.Speed; requested != "" {
		if _, err := speedpkg.Parse(string(requested)); err != nil {
			return ResolvedRuntime{}, NewRuntimeValidationError(RuntimeValidationItem{
				TaskID: taskID,
				Field:  runtimeFieldSpeed,
				Value:  string(requested),
				Reason: "unsupported_speed",
			})
		}
	}
	if catalog == nil {
		if resolved.Runtime.Provider == "" && resolved.Runtime.Model == "" {
			return resolved, nil
		}
		return ResolvedRuntime{}, fmt.Errorf("%w: runtime catalog is unavailable", ErrActionDependencyMissing)
	}
	if provider := catalog.CanonicalProvider(resolved.Runtime.Provider); provider != "" {
		resolved.Runtime.Provider = provider
	}
	if err := catalog.ValidateRuntime(ctx, resolved.Runtime); err != nil {
		if validation, ok := AsRuntimeValidationError(err); ok {
			for index := range validation.Items {
				if strings.TrimSpace(validation.Items[index].TaskID) == "" {
					validation.Items[index].TaskID = strings.TrimSpace(taskID)
				}
			}
		}
		return ResolvedRuntime{}, err
	}
	return resolved, nil
}

func resolveMatchingRuntime(rules []RuntimeRule, item ItemRuntime) RuntimeSpec {
	fields := [4]runtimeCandidate{}
	for index, rule := range rules {
		specificity, matches := runtimeRuleSpecificity(rule.Match, item)
		if !matches {
			continue
		}
		values := [4]string{
			rule.Runtime.Provider,
			rule.Runtime.Model,
			rule.Runtime.Reasoning,
			string(rule.Runtime.Speed),
		}
		for field, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			candidate := runtimeCandidate{value: value, specificity: specificity, index: index, set: true}
			if !fields[field].set || candidate.specificity > fields[field].specificity ||
				(candidate.specificity == fields[field].specificity && candidate.index > fields[field].index) {
				fields[field] = candidate
			}
		}
	}
	return RuntimeSpec{
		Provider:  fields[0].value,
		Model:     fields[1].value,
		Reasoning: fields[2].value,
		Speed:     speedpkg.Speed(fields[3].value),
	}
}

type runtimeCandidate struct {
	value       string
	specificity int
	index       int
	set         bool
}

func runtimeRuleSpecificity(match RuntimeMatch, item ItemRuntime) (int, bool) {
	if value := strings.TrimSpace(match.ID); value != "" {
		return 3, value == strings.TrimSpace(item.TaskID)
	}
	if value := strings.TrimSpace(match.Type); value != "" {
		return 2, value == strings.TrimSpace(item.TaskType)
	}
	if value := strings.TrimSpace(match.Complexity); value != "" {
		return 1, value == strings.TrimSpace(item.Complexity)
	}
	return 0, false
}

func applyRuntime(resolved *ResolvedRuntime, runtime RuntimeSpec, source RuntimeSource) {
	if value := strings.TrimSpace(runtime.Provider); value != "" {
		resolved.Runtime.Provider = value
		resolved.Source.Provider = source
	}
	if value := strings.TrimSpace(runtime.Model); value != "" {
		resolved.Runtime.Model = value
		resolved.Source.Model = source
	}
	if value := strings.TrimSpace(runtime.Reasoning); value != "" {
		resolved.Runtime.Reasoning = value
		resolved.Source.Reasoning = source
	}
	if value := strings.TrimSpace(string(runtime.Speed)); value != "" {
		resolved.Runtime.Speed = speedpkg.Speed(value)
		resolved.Source.Speed = source
	}
}

func normalizeResolvedRuntime(resolved ResolvedRuntime) ResolvedRuntime {
	resolved.Runtime.Provider = strings.TrimSpace(resolved.Runtime.Provider)
	resolved.Runtime.Model = strings.TrimSpace(resolved.Runtime.Model)
	resolved.Runtime.Reasoning = strings.TrimSpace(resolved.Runtime.Reasoning)
	resolved.Runtime.Speed = speedpkg.Speed(strings.TrimSpace(string(resolved.Runtime.Speed)))
	resolved.SpeedResolution = speedpkg.CloneResolution(resolved.SpeedResolution)
	return resolved
}

func runtimeSpecHasValue(runtime RuntimeSpec) bool {
	return strings.TrimSpace(runtime.Provider) != "" ||
		strings.TrimSpace(runtime.Model) != "" ||
		strings.TrimSpace(runtime.Reasoning) != "" ||
		strings.TrimSpace(string(runtime.Speed)) != ""
}
