package gate

import (
	"fmt"
	"maps"
	"slices"

	"github.com/compozy/agh/internal/loop/dsl"
)

func cloneCriteria(criteria []dsl.GateCriterion) []dsl.GateCriterion {
	cloned := make([]dsl.GateCriterion, len(criteria))
	copy(cloned, criteria)
	for i := range cloned {
		cloned[i].Inputs = cloneMapValues(cloned[i].Inputs)
		cloned[i].Extra = cloneMapValues(cloned[i].Extra)
	}
	return cloned
}

func cloneMapValues(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneRouteValue(value)
	}
	return cloned
}

func cloneResultRoutes(routes map[string]any) map[string]any {
	if len(routes) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(routes))
	for key, value := range routes {
		cloned[key] = cloneRouteValue(value)
	}
	return cloned
}

func cloneRouteValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, nested := range typed {
			cloned[key] = cloneRouteValue(nested)
		}
		return cloned
	case map[string]string:
		return maps.Clone(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, nested := range typed {
			cloned[i] = cloneRouteValue(nested)
		}
		return cloned
	case []string:
		return slices.Clone(typed)
	default:
		return value
	}
}

func contractVerdictPolicy(criteria []dsl.GateCriterion) dsl.VerdictPolicy {
	for _, criterion := range criteria {
		switch criterion.Type {
		case dsl.CriterionAgentJudge, dsl.CriterionHuman:
			return dsl.VerdictPolicyReviseUntilClean
		default:
		}
	}
	return dsl.VerdictPolicyFixedPasses
}

func wrapInvalidGate(message string, args ...any) error {
	return fmt.Errorf("%w: "+message, append([]any{ErrInvalidGate}, args...)...)
}
