package cli

import (
	"fmt"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/modelcatalog"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

const (
	loopRuntimeSelectorWorker     = "worker"
	loopRuntimeSelectorJudge      = "judge"
	loopRuntimeSelectorID         = "id"
	loopRuntimeSelectorType       = "type"
	loopRuntimeSelectorComplexity = "complexity"
)

func parseLoopRuntimeFlags(values []string) (looppkg.LoopConfig, error) {
	config := looppkg.LoopConfig{}
	for _, raw := range values {
		selector, expression, ok := strings.Cut(strings.TrimSpace(raw), "=")
		selector = strings.TrimSpace(selector)
		if !ok || selector == "" || strings.TrimSpace(expression) == "" {
			return looppkg.LoopConfig{}, runtimeFlagError("expected selector=runtime", raw)
		}
		switch selector {
		case loopRuntimeSelectorWorker, loopRuntimeSelectorJudge:
			runtime, err := parseLoopRuntimeExpression(expression)
			if err != nil {
				return looppkg.LoopConfig{}, err
			}
			if config.RuntimeDefaults == nil {
				config.RuntimeDefaults = &looppkg.RuntimeDefaults{}
			}
			if selector == loopRuntimeSelectorWorker {
				mergeRuntimeFlagSpec(&config.RuntimeDefaults.Worker, runtime)
			} else {
				mergeRuntimeFlagSpec(&config.RuntimeDefaults.Judge, runtime)
			}
		case loopRuntimeSelectorID, loopRuntimeSelectorType, loopRuntimeSelectorComplexity:
			matchValue, runtimeValue, found := strings.Cut(expression, ":")
			matchValue = strings.TrimSpace(matchValue)
			if !found {
				return looppkg.LoopConfig{}, runtimeFlagError("rule requires match:runtime", raw)
			}
			runtimeIdentity := runtimeValue
			if beforeSpeed, _, hasSpeed := strings.Cut(runtimeIdentity, ":speed="); hasSpeed {
				runtimeIdentity = beforeSpeed
			}
			if strings.Contains(runtimeIdentity, ":") {
				return looppkg.LoopConfig{}, runtimeFlagError(
					"rule match must contain one selector value without ':'",
					raw,
				)
			}
			if strings.Contains(matchValue, ",") {
				return looppkg.LoopConfig{}, runtimeFlagError(
					"rule match contains duplicate selectors",
					raw,
				)
			}
			if matchValue == "" || strings.Contains(matchValue, ":") {
				return looppkg.LoopConfig{}, runtimeFlagError(
					"rule match must contain one selector value without ':'",
					raw,
				)
			}
			runtime, err := parseLoopRuntimeExpression(runtimeValue)
			if err != nil {
				return looppkg.LoopConfig{}, err
			}
			match := looppkg.RuntimeMatch{}
			switch selector {
			case loopRuntimeSelectorID:
				match.ID = matchValue
			case loopRuntimeSelectorType:
				match.Type = matchValue
			case loopRuntimeSelectorComplexity:
				match.Complexity = matchValue
			}
			config.RuntimeRules = append(config.RuntimeRules, looppkg.RuntimeRule{
				Match: match, Runtime: runtime,
			})
		default:
			return looppkg.LoopConfig{}, runtimeFlagError("unknown selector", raw)
		}
	}
	return config, nil
}

func parseLoopRuntimeExpression(raw string) (looppkg.RuntimeSpec, error) {
	expression := strings.TrimSpace(raw)
	if expression == "" {
		return looppkg.RuntimeSpec{}, runtimeFlagError("runtime expression is empty", raw)
	}
	speed := speedpkg.Speed("")
	if before, after, found := strings.Cut(expression, ":speed="); found {
		if strings.Contains(before, ":speed=") || strings.Contains(after, ":speed=") ||
			strings.TrimSpace(before) == "" {
			return looppkg.RuntimeSpec{}, runtimeFlagError("invalid speed suffix", raw)
		}
		parsed, err := speedpkg.Parse(after)
		if err != nil {
			return looppkg.RuntimeSpec{}, runtimeFlagError("invalid speed suffix", raw)
		}
		expression = strings.TrimSpace(before)
		speed = parsed
	}
	reasoning := ""
	if before, after, found := strings.Cut(expression, "@"); found {
		if strings.Contains(after, "@") || strings.TrimSpace(after) == "" ||
			!modelcatalog.IsValidEffort(strings.TrimSpace(after)) {
			return looppkg.RuntimeSpec{}, runtimeFlagError("invalid reasoning suffix", raw)
		}
		expression = strings.TrimSpace(before)
		reasoning = strings.TrimSpace(after)
	}
	provider, model := "", expression
	if before, after, found := strings.Cut(expression, "/"); found {
		provider = strings.TrimSpace(before)
		model = strings.TrimSpace(after)
		if provider == "" || model == "" {
			return looppkg.RuntimeSpec{}, runtimeFlagError(
				"empty provider/model segments must use '-' to skip a field",
				raw,
			)
		}
	}
	if provider == "-" {
		provider = ""
	}
	if model == "-" {
		model = ""
	}
	if provider == "" && model == "" && reasoning == "" && speed == "" {
		return looppkg.RuntimeSpec{}, runtimeFlagError("runtime expression selects no fields", raw)
	}
	return looppkg.RuntimeSpec{
		Provider: provider, Model: model, Reasoning: reasoning, Speed: speed,
	}, nil
}

func mergeRuntimeFlagSpec(target *looppkg.RuntimeSpec, value looppkg.RuntimeSpec) {
	if value.Provider != "" {
		target.Provider = value.Provider
	}
	if value.Model != "" {
		target.Model = value.Model
	}
	if value.Reasoning != "" {
		target.Reasoning = value.Reasoning
	}
	if value.Speed != "" {
		target.Speed = value.Speed
	}
}

func mergeLoopRuntimeFlags(target *looppkg.LoopConfig, flags looppkg.LoopConfig) {
	if flags.RuntimeDefaults != nil {
		if target.RuntimeDefaults == nil {
			target.RuntimeDefaults = &looppkg.RuntimeDefaults{}
		}
		mergeRuntimeFlagSpec(&target.RuntimeDefaults.Worker, flags.RuntimeDefaults.Worker)
		mergeRuntimeFlagSpec(&target.RuntimeDefaults.Judge, flags.RuntimeDefaults.Judge)
	}
	target.RuntimeRules = append(target.RuntimeRules, flags.RuntimeRules...)
}

func runtimeFlagError(reason string, value string) error {
	return fmt.Errorf(
		"cli: invalid --runtime %q: %s; see MIGRATION_GUIDE.md#per-task-runtime-selection",
		value,
		reason,
	)
}
