package skills

import (
	"fmt"
	"slices"
	"strings"
)

var activationGateKeys = []string{
	"environments",
	"platforms",
	"requires_capabilities",
	"requires_tools",
}

func parseAGHMetadata(skill *Skill) error {
	if skill == nil || skill.Meta.Metadata == nil {
		return nil
	}

	rawAGH, ok := skill.Meta.Metadata["agh"]
	if !ok || rawAGH == nil {
		return nil
	}

	agh, ok := rawAGH.(map[string]any)
	if !ok {
		warnAGHMetadata(skill, "skills: malformed metadata.agh block", "type", fmt.Sprintf("%T", rawAGH))
		return nil
	}

	if rawMCPServers, ok := agh["mcp_servers"]; ok {
		skill.MCPServers = parseMCPServerDecls(skill, rawMCPServers)
	}
	if rawHooks, ok := agh["hooks"]; ok {
		hooks, err := parseHookDecls(skill, rawHooks)
		if err != nil {
			return err
		}
		skill.Hooks = hooks
	}
	if rawWhen, ok := agh["when"]; ok {
		gates, err := parseActivationGates(rawWhen)
		if err != nil {
			return err
		}
		skill.ActivationGates = gates
	}

	return nil
}

func parseActivationGates(raw any) (ActivationGates, error) {
	if raw == nil {
		return ActivationGates{}, nil
	}
	when, ok := raw.(map[string]any)
	if !ok {
		return ActivationGates{}, fmt.Errorf("metadata.agh.when must be a mapping, got %T", raw)
	}

	keys := make([]string, 0, len(when))
	for key := range when {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		if !slices.Contains(activationGateKeys, key) {
			return ActivationGates{}, fmt.Errorf("metadata.agh.when: unknown key %q", key)
		}
	}

	platforms, err := activationAtoms(when["platforms"], "platforms")
	if err != nil {
		return ActivationGates{}, err
	}
	environments, err := activationAtoms(when["environments"], "environments")
	if err != nil {
		return ActivationGates{}, err
	}
	requiredTools, err := activationAtoms(when["requires_tools"], "requires_tools")
	if err != nil {
		return ActivationGates{}, err
	}
	requiredCapabilities, err := activationAtoms(when["requires_capabilities"], "requires_capabilities")
	if err != nil {
		return ActivationGates{}, err
	}
	return ActivationGates{
		Platforms:            platforms,
		Environments:         environments,
		RequiresTools:        requiredTools,
		RequiresCapabilities: requiredCapabilities,
	}, nil
}

func activationAtoms(raw any, field string) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("metadata.agh.when.%s must be a string array, got %T", field, raw)
	}

	values := make([]string, 0, len(items))
	for idx, item := range items {
		value, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("metadata.agh.when.%s[%d] must be a string, got %T", field, idx, item)
		}
		values = append(values, value)
	}
	return normalizeActivationValues(values, field)
}

func normalizeActivationValues(values []string, field string) ([]string, error) {
	normalizedValues := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for idx, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return nil, fmt.Errorf("metadata.agh.when.%s[%d] is required", field, idx)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalizedValues = append(normalizedValues, value)
	}
	if len(normalizedValues) == 0 {
		return nil, nil
	}
	return slices.Clip(normalizedValues), nil
}

func normalizeActivationGates(gates ActivationGates) (ActivationGates, error) {
	platforms, err := normalizeActivationValues(gates.Platforms, "platforms")
	if err != nil {
		return ActivationGates{}, err
	}
	environments, err := normalizeActivationValues(gates.Environments, "environments")
	if err != nil {
		return ActivationGates{}, err
	}
	requiredTools, err := normalizeActivationValues(gates.RequiresTools, "requires_tools")
	if err != nil {
		return ActivationGates{}, err
	}
	requiredCapabilities, err := normalizeActivationValues(
		gates.RequiresCapabilities,
		"requires_capabilities",
	)
	if err != nil {
		return ActivationGates{}, err
	}
	return ActivationGates{
		Platforms:            platforms,
		Environments:         environments,
		RequiresTools:        requiredTools,
		RequiresCapabilities: requiredCapabilities,
	}, nil
}

func cloneActivationGates(gates ActivationGates) ActivationGates {
	return ActivationGates{
		Platforms:            slices.Clone(gates.Platforms),
		Environments:         slices.Clone(gates.Environments),
		RequiresTools:        slices.Clone(gates.RequiresTools),
		RequiresCapabilities: slices.Clone(gates.RequiresCapabilities),
	}
}
