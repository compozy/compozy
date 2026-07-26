package loop

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
)

type commandCheckOverride struct {
	Enabled *bool
	Command string
}

func applyEffectiveGateConfig(runtimeGate gate.Gate, effective EffectiveConfig) (gate.Gate, bool, error) {
	overrides, err := commandCheckOverrides(effective.EnabledChecks)
	if err != nil {
		return gate.Gate{}, false, err
	}
	criteria := make([]dsl.GateCriterion, 0, len(runtimeGate.Criteria))
	for _, criterion := range runtimeGate.Criteria {
		switch criterion.Type {
		case dsl.CriterionHuman:
			if !effective.HumanGateEnabled {
				continue
			}
		case dsl.CriterionCommand:
			if override, ok := overrides[strings.TrimSpace(criterion.ID)]; ok {
				if override.Enabled != nil && !*override.Enabled {
					continue
				}
				if strings.TrimSpace(override.Command) != "" {
					criterion.Check = override.Command
				}
			}
			if strings.TrimSpace(criterion.Check) == "" {
				continue
			}
		}
		criteria = append(criteria, criterion)
	}
	runtimeGate.Criteria = criteria
	return runtimeGate, len(criteria) == 0, nil
}

func commandCheckOverrides(raw json.RawMessage) (map[string]commandCheckOverride, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("%w: enabled_checks_json must be valid JSON", ErrValidation)
	}
	object, ok := decoded.(map[string]any)
	if !ok {
		return nil, nil
	}
	overrides := make(map[string]commandCheckOverride, len(object))
	for id, rawEntry := range object {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		override := commandCheckOverride{}
		if enabled, ok := entry["enabled"].(bool); ok {
			override.Enabled = &enabled
		}
		if command, ok := entry["command"].(string); ok {
			override.Command = command
		}
		overrides[strings.TrimSpace(id)] = override
	}
	return overrides, nil
}

func gateApprovedOutputRef() (string, error) {
	return gateVerdictOutputRef(gate.Verdict{
		Outcome: gate.VerdictOutcomeApproved,
		Route: gate.RouteDecision{
			Action: gate.RouteContinue,
		},
	})
}
