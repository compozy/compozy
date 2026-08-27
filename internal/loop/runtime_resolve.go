package loop

import (
	"context"
	"fmt"
	"maps"
	"slices"
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

// ValidateRuntimeRules enforces the supported selector shapes and non-empty runtime values.
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
	options := make(map[string]runtimeOptionCandidate)
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
		for _, option := range rule.Runtime.ACPOptions {
			option.ID = strings.TrimSpace(option.ID)
			if option.ID == "" {
				continue
			}
			candidate := runtimeOptionCandidate{
				selection:   option,
				specificity: specificity,
				index:       index,
			}
			previous, exists := options[option.ID]
			if !exists || candidate.specificity > previous.specificity ||
				(candidate.specificity == previous.specificity && candidate.index > previous.index) {
				options[option.ID] = candidate
			}
		}
	}
	resolved := RuntimeSpec{
		Provider:  fields[0].value,
		Model:     fields[1].value,
		Reasoning: fields[2].value,
		Speed:     speedpkg.Speed(fields[3].value),
	}
	for _, candidate := range options {
		resolved.ACPOptions = append(resolved.ACPOptions, candidate.selection)
	}
	slices.SortFunc(resolved.ACPOptions, func(left ACPOptionSelection, right ACPOptionSelection) int {
		return strings.Compare(left.ID, right.ID)
	})
	return resolved
}

type runtimeCandidate struct {
	value       string
	specificity int
	index       int
	set         bool
}

type runtimeOptionCandidate struct {
	selection   ACPOptionSelection
	specificity int
	index       int
}

func runtimeRuleSpecificity(match RuntimeMatch, item ItemRuntime) (int, bool) {
	id := strings.TrimSpace(match.ID)
	taskType := strings.TrimSpace(match.Type)
	complexity := strings.TrimSpace(match.Complexity)
	if id != "" {
		return 4, id == strings.TrimSpace(item.TaskID)
	}
	if taskType != "" && complexity != "" {
		return 3,
			taskType == strings.TrimSpace(item.TaskType) &&
				complexity == strings.TrimSpace(item.Complexity)
	}
	if taskType != "" {
		return 2, taskType == strings.TrimSpace(item.TaskType)
	}
	if complexity != "" {
		return 1, complexity == strings.TrimSpace(item.Complexity)
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
	for _, option := range runtime.ACPOptions {
		option.ID = strings.TrimSpace(option.ID)
		if option.ID == "" {
			continue
		}
		option.ValueID = strings.TrimSpace(option.ValueID)
		updated := false
		for index := range resolved.Runtime.ACPOptions {
			if strings.TrimSpace(resolved.Runtime.ACPOptions[index].ID) != option.ID {
				continue
			}
			resolved.Runtime.ACPOptions[index] = option
			updated = true
			break
		}
		if !updated {
			resolved.Runtime.ACPOptions = append(resolved.Runtime.ACPOptions, option)
		}
		if resolved.Source.ACPOptions == nil {
			resolved.Source.ACPOptions = make(map[string]RuntimeSource)
		}
		resolved.Source.ACPOptions[option.ID] = source
	}
}

func normalizeResolvedRuntime(resolved ResolvedRuntime) ResolvedRuntime {
	resolved.Runtime.Provider = strings.TrimSpace(resolved.Runtime.Provider)
	resolved.Runtime.Model = strings.TrimSpace(resolved.Runtime.Model)
	resolved.Runtime.Reasoning = strings.TrimSpace(resolved.Runtime.Reasoning)
	resolved.Runtime.Speed = speedpkg.Speed(strings.TrimSpace(string(resolved.Runtime.Speed)))
	for index := range resolved.Runtime.ACPOptions {
		resolved.Runtime.ACPOptions[index].ID = strings.TrimSpace(resolved.Runtime.ACPOptions[index].ID)
		resolved.Runtime.ACPOptions[index].ValueID = strings.TrimSpace(resolved.Runtime.ACPOptions[index].ValueID)
	}
	slices.SortFunc(resolved.Runtime.ACPOptions, func(left ACPOptionSelection, right ACPOptionSelection) int {
		return strings.Compare(left.ID, right.ID)
	})
	if len(resolved.Source.ACPOptions) == 0 {
		resolved.Source.ACPOptions = nil
	} else {
		resolved.Source.ACPOptions = maps.Clone(resolved.Source.ACPOptions)
	}
	resolved.SpeedResolution = speedpkg.CloneResolution(resolved.SpeedResolution)
	return resolved
}

func runtimeSpecHasValue(runtime RuntimeSpec) bool {
	return strings.TrimSpace(runtime.Provider) != "" ||
		strings.TrimSpace(runtime.Model) != "" ||
		strings.TrimSpace(runtime.Reasoning) != "" ||
		strings.TrimSpace(string(runtime.Speed)) != "" ||
		len(runtime.ACPOptions) > 0
}
