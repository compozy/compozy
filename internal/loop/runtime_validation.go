package loop

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/modelcatalog"
)

// ValidateDefinitionRuntime validates only runtime values authored in one definition.
func ValidateDefinitionRuntime(
	ctx context.Context,
	catalog RuntimeCatalog,
	def dsl.Definition,
) error {
	if value, exists := def.Contract.Extra["model_defaults"]; exists {
		return runtimeValidation("model_defaults", value, "retired_key")
	}
	if defaults := def.Contract.RuntimeDefaults; defaults != nil {
		if err := validateRuntimeSpec(ctx, catalog, "runtime_defaults.worker", defaults.Worker); err != nil {
			return err
		}
		if err := validateRuntimeSpec(ctx, catalog, "runtime_defaults.judge", defaults.Judge); err != nil {
			return err
		}
	}
	if err := validateRuntimeRules(ctx, catalog, def.Contract.RuntimeRules); err != nil {
		return err
	}
	if err := validateRuntimeCriteria(ctx, catalog, "contract.verification", def.Contract.Verification); err != nil {
		return err
	}
	return validateRuntimeGraph(ctx, catalog, def.Inputs, def.Graph)
}

// ValidateLoopConfigRuntime validates runtime values in one raw configuration layer.
func ValidateLoopConfigRuntime(ctx context.Context, catalog RuntimeCatalog, cfg LoopConfig) error {
	if cfg.Environment != nil {
		if _, err := ResolveActionEnvironment(*cfg.Environment, dsl.EnvironmentSpec{}); err != nil {
			return err
		}
	}
	if cfg.RuntimeDefaults != nil {
		if err := validateRuntimeSpec(
			ctx,
			catalog,
			"runtime_defaults.worker",
			cfg.RuntimeDefaults.Worker,
		); err != nil {
			return err
		}
		if err := validateRuntimeSpec(
			ctx,
			catalog,
			"runtime_defaults.judge",
			cfg.RuntimeDefaults.Judge,
		); err != nil {
			return err
		}
	}
	return validateRuntimeRules(ctx, catalog, cfg.RuntimeRules)
}

// ValidateEffectiveRuntime validates every explicit runtime value before a run is created.
func ValidateEffectiveRuntime(ctx context.Context, catalog RuntimeCatalog, cfg EffectiveConfig) error {
	if err := validateRuntimeSpec(ctx, catalog, "runtime_defaults.worker", cfg.RuntimeDefaults.Worker); err != nil {
		return err
	}
	if err := validateRuntimeSpec(ctx, catalog, "runtime_defaults.judge", cfg.RuntimeDefaults.Judge); err != nil {
		return err
	}
	if err := validateRuntimeRules(ctx, catalog, cfg.RuntimeRules); err != nil {
		return err
	}
	return validateRuntimeRules(ctx, catalog, cfg.RunRuntimeRules)
}

func validateRuntimeGraph(
	ctx context.Context,
	catalog RuntimeCatalog,
	inputs map[string]dsl.Input,
	graph dsl.Graph,
) error {
	for _, node := range graph.Nodes {
		if value, exists := node.Params["cwd"]; exists {
			return runtimeValidation(
				fmt.Sprintf("graph.nodes.%s.cwd", node.ID),
				value,
				"retired_key_use_environment_directory",
			)
		}
		if value, exists := node.Params["model"]; exists {
			return runtimeValidation(fmt.Sprintf("graph.nodes.%s.params.model", node.ID), value, "retired_key")
		}
		switch {
		case node.Class == dsl.NodeClassAction && dsl.ActionKind(node.Kind) == dsl.ActionRunAgent:
			if err := validateNodeRuntimeAuthoring(
				ctx,
				catalog,
				inputs,
				fmt.Sprintf("graph.nodes.%s.params.runtime", node.ID),
				node.Params,
			); err != nil {
				return err
			}
		case node.Class == dsl.NodeClassAction && dsl.ActionKind(node.Kind) == dsl.ActionGoal:
			params, err := decodeGoalNodeParams(node.Params)
			if err != nil {
				return fmt.Errorf("%w: decode goal params for %q: %w", ErrValidation, node.ID, err)
			}
			if err := validateNodeRuntimeAuthoring(
				ctx,
				catalog,
				inputs,
				fmt.Sprintf("graph.nodes.%s.params.runtime", node.ID),
				node.Params,
			); err != nil {
				return err
			}
			if err := validateRuntimeCriteria(
				ctx,
				catalog,
				fmt.Sprintf("graph.nodes.%s.params.judge", node.ID),
				params.Judge,
			); err != nil {
				return err
			}
		}
		if err := validateRuntimeCriteria(
			ctx,
			catalog,
			fmt.Sprintf("graph.nodes.%s.criteria", node.ID),
			node.Criteria,
		); err != nil {
			return err
		}
		if node.Body != nil {
			if err := validateRuntimeGraph(ctx, catalog, inputs, *node.Body); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateRuntimeCriteria(
	ctx context.Context,
	catalog RuntimeCatalog,
	path string,
	criteria []dsl.GateCriterion,
) error {
	for index, criterion := range criteria {
		criterionPath := fmt.Sprintf("%s[%d]", path, index)
		if value, exists := criterion.Extra["model"]; exists {
			return runtimeValidation(criterionPath+".model", value, "retired_key")
		}
		if err := validateRuntimeSpec(ctx, catalog, criterionPath+".runtime", criterion.Runtime); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeRules(ctx context.Context, catalog RuntimeCatalog, rules []RuntimeRule) error {
	for index, rule := range rules {
		path := fmt.Sprintf("runtime_rules[%d]", index)
		if len(rule.Match.Extra) > 0 {
			keys := make([]string, 0, len(rule.Match.Extra))
			for key := range rule.Match.Extra {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			key := keys[0]
			return runtimeValidation(path+".match."+key, rule.Match.Extra[key], "unknown_field")
		}
		selectors := 0
		for _, value := range []string{rule.Match.ID, rule.Match.Type, rule.Match.Complexity} {
			if strings.TrimSpace(value) != "" {
				selectors++
			}
		}
		if selectors == 0 {
			return runtimeValidation(path+".match", "", "selector_required")
		}
		if selectors > 1 {
			return runtimeValidation(path+".match", "", "selector_collision")
		}
		if !runtimeSpecHasValue(rule.Runtime) {
			return runtimeValidation(path+".runtime", "", "empty_runtime")
		}
		if err := validateRuntimeSpec(ctx, catalog, path+".runtime", rule.Runtime); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimeSpec(
	ctx context.Context,
	catalog RuntimeCatalog,
	path string,
	runtime RuntimeSpec,
) error {
	if len(runtime.Extra) > 0 {
		keys := make([]string, 0, len(runtime.Extra))
		for key := range runtime.Extra {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		key := keys[0]
		return runtimeValidation(path+"."+key, runtime.Extra[key], "unknown_field")
	}
	if reasoning := strings.TrimSpace(runtime.Reasoning); reasoning != "" && !modelcatalog.IsValidEffort(reasoning) {
		return NewRuntimeValidationError(RuntimeValidationItem{
			Field:  runtimeFieldReasoning,
			Value:  reasoning,
			Reason: "unsupported_reasoning",
		})
	}
	if catalog == nil || !runtimeSpecHasValue(runtime) {
		return nil
	}
	_, err := ValidateResolvedRuntime(ctx, catalog, "", ResolvedRuntime{Runtime: runtime})
	return err
}

func runtimeValidation(field string, value any, reason string) error {
	return NewRuntimeValidationError(RuntimeValidationItem{
		Field:  field,
		Value:  strings.TrimSpace(fmt.Sprint(value)),
		Reason: reason,
	})
}
