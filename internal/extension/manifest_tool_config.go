package extensionpkg

import (
	"encoding/json"

	"fmt"

	"sort"

	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func validateToolConfigs(extensionName string, tools map[string]ToolConfig) error {
	if len(tools) == 0 {
		return nil
	}

	seenIDs := make(map[toolspkg.ToolID]string, len(tools))
	for _, name := range sortedMapKeys(tools) {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			continue
		}
		field := "resources.tools." + trimmedName
		cfg := tools[name]
		if err := validateEnvRequirements(field+".requires_env", cfg.RequiresEnv); err != nil {
			return err
		}
		if err := validateDottedIdentifiers(
			field+".required_capabilities",
			cfg.RequiredCapabilities,
			false,
		); err != nil {
			return err
		}
		descriptor, err := resolveManifestToolDescriptor(extensionName, trimmedName, cfg)
		if err != nil {
			return &ManifestValidationError{
				Field:   field,
				Message: err.Error(),
			}
		}
		if previous, ok := seenIDs[descriptor.Tool.ID]; ok {
			return &ManifestValidationError{
				Field: field + ".id",
				Value: descriptor.Tool.ID.String(),
				Message: fmt.Sprintf(
					"duplicate tool id also declared by resources.tools.%s",
					previous,
				),
			}
		}
		seenIDs[descriptor.Tool.ID] = trimmedName
	}
	return nil
}

func validEnvRequirementName(name string) bool {
	if name == "" {
		return false
	}
	for idx, r := range name {
		if idx == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func normalizeHooks(src []HookConfig) []HookConfig {
	if len(src) == 0 {
		return nil
	}

	dst := make([]HookConfig, 0, len(src))
	for idx := range src {
		hook := &src[idx]
		dst = append(dst, HookConfig{
			Name:     strings.TrimSpace(hook.Name),
			Event:    strings.TrimSpace(hook.Event),
			Mode:     strings.TrimSpace(hook.Mode),
			Required: hook.Required,
			Priority: cloneIntPointer(hook.Priority),
			Timeout:  hook.Timeout,
			Matcher: HookMatcherConfig{
				AgentName:          strings.TrimSpace(hook.Matcher.AgentName),
				AgentType:          strings.TrimSpace(hook.Matcher.AgentType),
				WorkspaceID:        strings.TrimSpace(hook.Matcher.WorkspaceID),
				WorkspaceRoot:      strings.TrimSpace(hook.Matcher.WorkspaceRoot),
				SessionType:        strings.TrimSpace(hook.Matcher.SessionType),
				InputClass:         strings.TrimSpace(hook.Matcher.InputClass),
				ACPEventType:       strings.TrimSpace(hook.Matcher.ACPEventType),
				TurnID:             strings.TrimSpace(hook.Matcher.TurnID),
				ToolID:             strings.TrimSpace(hook.Matcher.ToolID),
				ToolName:           strings.TrimSpace(hook.Matcher.ToolName),
				ToolReadOnly:       cloneBoolPointer(hook.Matcher.ToolReadOnly),
				DecisionClass:      strings.TrimSpace(hook.Matcher.DecisionClass),
				MessageRole:        strings.TrimSpace(hook.Matcher.MessageRole),
				MessageDeltaType:   strings.TrimSpace(hook.Matcher.MessageDeltaType),
				Channel:            strings.TrimSpace(hook.Matcher.Channel),
				Surface:            strings.TrimSpace(hook.Matcher.Surface),
				Kind:               strings.TrimSpace(hook.Matcher.Kind),
				Direction:          strings.TrimSpace(hook.Matcher.Direction),
				WorkState:          strings.TrimSpace(hook.Matcher.WorkState),
				CompactionReason:   strings.TrimSpace(hook.Matcher.CompactionReason),
				CompactionStrategy: strings.TrimSpace(hook.Matcher.CompactionStrategy),
			},
			Command:   strings.TrimSpace(hook.Command),
			Args:      normalizeStrings(hook.Args),
			Env:       normalizeStringMap(hook.Env),
			SecretEnv: normalizeStringMap(hook.SecretEnv),
			Executor: HookExecutorConfig{
				Kind:      strings.TrimSpace(hook.Executor.Kind),
				Command:   strings.TrimSpace(hook.Executor.Command),
				Args:      normalizeStrings(hook.Executor.Args),
				Env:       normalizeStringMap(hook.Executor.Env),
				SecretEnv: normalizeStringMap(hook.Executor.SecretEnv),
			},
		})
	}

	return dst
}

func normalizeStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}

	dst := make([]string, 0, len(src))
	for _, value := range src {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		dst = append(dst, trimmed)
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}

func normalizeStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	dst := make(map[string]string, len(src))
	for _, key := range sortedMapKeys(src) {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		dst[trimmedKey] = strings.TrimSpace(src[key])
	}
	if len(dst) == 0 {
		return nil
	}
	return dst
}

func sortedMapKeys[V any](src map[string]V) []string {
	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneManifestRawMessage(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), src...)
}

func requireField(field, value string) error {
	if strings.TrimSpace(value) != "" {
		return nil
	}
	return &ManifestValidationError{
		Field:   field,
		Message: "value is required",
	}
}

func validateSemanticVersionField(field, value string) error {
	if _, ok := parseSemanticVersion(value); ok {
		return nil
	}
	return &ManifestValidationError{
		Field:   field,
		Value:   value,
		Message: manifestMustBeASemanticVersionValue,
	}
}
