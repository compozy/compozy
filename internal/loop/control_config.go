package loop

import (
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
)

func pinnedEffectiveConfig(resolved *ResolvedDefinition) (EffectiveConfig, error) {
	if resolved == nil {
		return EffectiveConfig{}, fmt.Errorf("%w: resolved definition is required", ErrValidation)
	}
	effective := cloneEffectiveConfig(resolved.EffectiveConfig)
	if err := validateEffectiveConfig(effective); err != nil {
		return EffectiveConfig{}, err
	}
	return effective, nil
}

func coordinatorResolvedWithEffectiveConfig(
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
) *ResolvedDefinition {
	if resolved == nil {
		return nil
	}
	next := *resolved
	nodes := append([]dsl.Node(nil), resolved.Definition.Graph.Nodes...)
	for index, node := range nodes {
		if node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == dsl.ControlGate {
			node.MaxRevisions = effective.GateMaxRevisions
			nodes[index] = node
		}
	}
	next.Definition.Graph.Nodes = nodes
	return &next
}

func coordinatorFanOutWidth(effective EffectiveConfig) int {
	if effective.FanOutWidth <= 0 {
		return 1
	}
	return effective.FanOutWidth
}
