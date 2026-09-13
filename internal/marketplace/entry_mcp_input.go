package marketplace

import (
	"fmt"
	"net/url"
	"strings"
)

func validateMCPInputs(entryID string, launch mcpLaunch, inputs []EntryInput) error {
	seen := make(map[string]struct{}, len(inputs))
	seenBindings := make(map[string]struct{}, len(inputs))
	urlQueryNames, err := launchURLQueryNames(launch)
	if err != nil {
		return fmt.Errorf("marketplace catalog MCP entry %q: %w", entryID, err)
	}
	for index, input := range inputs {
		if err := input.validate(entryID, launch.Type, index); err != nil {
			return err
		}
		id := strings.TrimSpace(input.ID)
		if _, exists := seen[id]; exists {
			return fmt.Errorf("marketplace catalog MCP entry %q input id %q is duplicated", entryID, id)
		}
		seen[id] = struct{}{}
		binding := input.Binding.Type + "\x00" + strings.TrimSpace(input.Binding.Name)
		if _, exists := seenBindings[binding]; exists {
			return fmt.Errorf(
				"marketplace catalog MCP entry %q input binding %s/%q is duplicated",
				entryID,
				input.Binding.Type,
				strings.TrimSpace(input.Binding.Name),
			)
		}
		seenBindings[binding] = struct{}{}
		if input.Binding.Type == mcpInputBindingQuery {
			if _, exists := urlQueryNames[strings.TrimSpace(input.Binding.Name)]; exists {
				return fmt.Errorf(
					"marketplace catalog MCP entry %q input binding url_query/%q conflicts with launch URL",
					entryID,
					strings.TrimSpace(input.Binding.Name),
				)
			}
		}
	}
	return nil
}

func launchURLQueryNames(launch mcpLaunch) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	if launch.Type != mcpLaunchTypeRemote {
		return names, nil
	}
	parsed, err := url.Parse(launch.URL)
	if err != nil {
		return nil, fmt.Errorf("parse launch URL: %w", err)
	}
	for name := range parsed.Query() {
		names[name] = struct{}{}
	}
	return names, nil
}

func (i EntryInput) validate(entryID string, launchType string, index int) error {
	prefix := fmt.Sprintf("marketplace catalog MCP entry %q inputs[%d]", entryID, index)
	if err := i.validateGrammar(prefix); err != nil {
		return err
	}
	switch i.Binding.Type {
	case mcpInputBindingEnv:
		if launchType == mcpLaunchTypeRemote {
			return fmt.Errorf("%s.binding env is only supported for stdio launch", prefix)
		}
	case mcpInputBindingQuery:
		if launchType != mcpLaunchTypeRemote {
			return fmt.Errorf("%s.binding url_query is only supported for remote launch", prefix)
		}
	}
	return nil
}
