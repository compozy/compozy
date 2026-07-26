package config

import (
	"fmt"
	"math"
	"strings"

	"github.com/compozy/agh/internal/reasoning"
)

// Validate reports whether the provider model block is usable.
func (m ProviderModelsConfig) Validate(path string) error {
	if strings.TrimSpace(m.Default) == "" && m.Default != "" {
		return fmt.Errorf("%s.default must not be whitespace-only", path)
	}
	seen := make(map[string]struct{}, len(m.Curated))
	for idx, model := range m.Curated {
		modelPath := fmt.Sprintf("%s.curated[%d]", path, idx)
		id := strings.TrimSpace(model.ID)
		if id == "" {
			return fmt.Errorf("%s.id is required", modelPath)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%s.id duplicates %q", modelPath, id)
		}
		seen[id] = struct{}{}
		efforts := make(map[string]struct{}, len(model.ReasoningEfforts))
		for effortIdx, effort := range model.ReasoningEfforts {
			effortPath := fmt.Sprintf("%s.reasoning_efforts[%d]", modelPath, effortIdx)
			trimmed := strings.TrimSpace(effort)
			if effort != trimmed {
				return &reasoning.InvalidEffortError{Path: effortPath, Value: effort}
			}
			if !reasoning.IsValid(effort) {
				return &reasoning.InvalidEffortError{Path: effortPath, Value: effort}
			}
			if _, ok := efforts[effort]; ok {
				return fmt.Errorf("%s duplicates %q", effortPath, effort)
			}
			efforts[effort] = struct{}{}
		}
		defaultEffort := model.DefaultReasoningEffort
		if defaultEffort != "" {
			defaultPath := modelPath + ".default_reasoning_effort"
			trimmedDefault := strings.TrimSpace(defaultEffort)
			if defaultEffort != trimmedDefault {
				return &reasoning.InvalidEffortError{Path: defaultPath, Value: defaultEffort}
			}
			if !reasoning.IsValid(defaultEffort) {
				return &reasoning.InvalidEffortError{Path: defaultPath, Value: defaultEffort}
			}
			if len(efforts) > 0 {
				if _, ok := efforts[defaultEffort]; !ok {
					return fmt.Errorf("%s must be listed in reasoning_efforts", defaultPath)
				}
			}
		}
		if err := validateProviderModelReleaseDate(modelPath, model.ReleaseDate); err != nil {
			return err
		}
		if err := validateProviderModelCosts(modelPath, model); err != nil {
			return err
		}
	}
	if err := m.Discovery.Validate(path + ".discovery"); err != nil {
		return err
	}
	return m.Reasoning.Validate(path + ".reasoning")
}

func validateProviderModelCosts(path string, model ProviderModelConfig) error {
	for _, cost := range []struct {
		name  string
		value *float64
	}{
		{name: "cost_input_per_million", value: model.CostInputPerMillion},
		{name: "cost_output_per_million", value: model.CostOutputPerMillion},
		{name: "cost_cache_read_per_million", value: model.CostCacheReadPerMillion},
		{name: "cost_cache_write_per_million", value: model.CostCacheWritePerMillion},
		{name: "cost_reasoning_per_million", value: model.CostReasoningPerMillion},
	} {
		if cost.value == nil {
			continue
		}
		if math.IsNaN(*cost.value) || math.IsInf(*cost.value, 0) || *cost.value < 0 {
			return fmt.Errorf("%s.%s must be finite and non-negative", path, cost.name)
		}
	}
	return nil
}
