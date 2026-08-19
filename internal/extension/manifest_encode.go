package extensionpkg

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/BurntSushi/toml"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func encodeManifestTOML(manifest *Manifest) ([]byte, error) {
	if manifest == nil {
		return nil, fmt.Errorf("extension: manifest is required")
	}
	document, err := manifestTOMLDocument(manifest)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	encoder := toml.NewEncoder(&output)
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("extension: encode generated manifest: %w", err)
	}
	return output.Bytes(), nil
}

func manifestTOMLDocument(manifest *Manifest) (map[string]any, error) {
	extension := map[string]any{
		manifestNameKey:              manifest.Name,
		manifestVersionKey:           manifest.Version,
		manifestMinCompozyVersionKey: manifest.MinCompozyVersion,
	}
	putNonEmpty(extension, manifestDescriptionKey, manifest.Description)
	putNonEmptyStrings(extension, "requires_env", manifest.RequiresEnv)
	document := map[string]any{
		manifestExtensionKey: extension,
		manifestCapabilitiesKey: map[string]any{
			"provides": manifest.Capabilities.Provides,
		},
		manifestPermissionsKey: map[string]any{
			"requires": manifest.Permissions.Requires,
		},
		manifestSubprocessKey: subprocessTOMLTable(manifest.Subprocess),
	}
	if requirement := manifest.NetworkParticipation.Normalize(); requirement != nil {
		networkParticipation := map[string]any{manifestRequiredKey: requirement.Required}
		putNonEmpty(networkParticipation, "mode", requirement.Mode)
		putNonEmptyStrings(networkParticipation, "channel_scopes", requirement.ChannelScopes)
		document[manifestFieldNetworkParticipation] = networkParticipation
	}

	resources, err := resourcesTOMLTable(manifest.Resources)
	if err != nil {
		return nil, err
	}
	if len(resources) > 0 {
		document["resources"] = resources
	}
	return document, nil
}

func subprocessTOMLTable(process SubprocessConfig) map[string]any {
	table := map[string]any{manifestCommandKey: process.Command}
	putNonEmptyStrings(table, manifestArgsKey, process.Args)
	putNonEmptyMap(table, manifestEnvKey, process.Env)
	return table
}

func resourcesTOMLTable(resources ResourcesConfig) (map[string]any, error) {
	table := make(map[string]any)
	putNonEmptyStrings(table, manifestSkillsKey, resources.Skills)
	putNonEmptyStrings(table, manifestLoopsKey, resources.Loops)
	putNonEmptyStrings(table, manifestAgentsKey, resources.Agents)
	putNonEmptyStrings(table, manifestAutomationKey, resources.Automation)
	putNonEmptyStrings(table, manifestLayoutsKey, resources.Layouts)
	if len(resources.Tools) > 0 {
		tools := make(map[string]any, len(resources.Tools))
		for name, config := range resources.Tools {
			encoded, err := toolTOMLTable(config)
			if err != nil {
				return nil, fmt.Errorf("extension: encode tool %q: %w", name, err)
			}
			tools[name] = encoded
		}
		table[manifestToolsKey] = tools
	}
	if len(resources.Hooks) > 0 {
		hooks := make([]map[string]any, 0, len(resources.Hooks))
		for index := range resources.Hooks {
			hooks = append(hooks, hookTOMLTable(&resources.Hooks[index]))
		}
		table[manifestHooksKey] = hooks
	}
	if len(resources.CommandGroups) > 0 {
		groups := make([]map[string]any, 0, len(resources.CommandGroups))
		for _, group := range resources.CommandGroups {
			groups = append(groups, map[string]any{
				manifestPathKey:    group.Path,
				manifestSummaryKey: group.Summary,
			})
		}
		table[manifestCommandGroupsKey] = groups
	}
	if len(resources.CmdPalette.Commands) > 0 || len(resources.CmdPalette.Views) > 0 {
		table[manifestCmdPaletteKey] = resources.CmdPalette
	}
	return table, nil
}

func toolTOMLTable(tool ToolConfig) (map[string]any, error) {
	table := map[string]any{
		"id":                tool.ID,
		manifestHandlerKey:  tool.Handler,
		manifestRiskKey:     tool.Risk,
		manifestReadOnlyKey: tool.ReadOnly,
		"destructive":       tool.Destructive,
		"open_world":        tool.OpenWorld,
		manifestBackendKey: map[string]any{
			manifestKindKey:    tool.Backend.Kind,
			manifestHandlerKey: tool.Backend.Handler,
		},
	}
	putNonEmpty(table, manifestDescriptionKey, tool.Description)
	putNonEmpty(table, "friendly_verb", tool.FriendlyVerb)
	putNonEmpty(table, "preview", tool.Preview)
	putNonEmpty(table, manifestVisibilityKey, tool.Visibility)
	putNonEmptyStrings(table, "required_capabilities", tool.RequiredCapabilities)
	if tool.Command != nil {
		command := map[string]any{
			"verb":    tool.Command.Verb,
			"summary": tool.Command.Summary,
		}
		putNonEmpty(command, "example", tool.Command.Example)
		if len(tool.Command.Flags) > 0 {
			command["flags"] = tool.Command.Flags
		}
		table["command"] = command
	}
	if tool.ConcurrencySafe {
		table["concurrency_safe"] = true
	}
	if tool.RequiresInteraction {
		table["requires_interaction"] = true
	}
	input, err := canonicalSchemaString(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("input schema: %w", err)
	}
	table[manifestInputSchemaKey] = input
	if len(tool.OutputSchema) > 0 {
		output, err := canonicalSchemaString(tool.OutputSchema)
		if err != nil {
			return nil, fmt.Errorf("output schema: %w", err)
		}
		table[manifestOutputSchemaKey] = output
	}
	return table, nil
}

func hookTOMLTable(hook *HookConfig) map[string]any {
	executor := map[string]any{
		manifestKindKey:    hook.Executor.Kind,
		manifestCommandKey: hook.Executor.Command,
	}
	table := map[string]any{
		manifestNameKey:     hook.Name,
		manifestEventKey:    hook.Event,
		manifestModeKey:     hook.Mode,
		manifestExecutorKey: executor,
	}
	putNonEmptyStrings(executor, manifestArgsKey, hook.Executor.Args)
	putNonEmptyMap(executor, manifestEnvKey, hook.Executor.Env)
	return table
}

func canonicalSchemaString(raw json.RawMessage) (string, error) {
	canonical, err := toolspkg.CanonicalJSON(raw)
	if err != nil {
		return "", err
	}
	return string(canonical), nil
}

func putNonEmpty(table map[string]any, key, value string) {
	if value != "" {
		table[key] = value
	}
}

func putNonEmptyStrings(table map[string]any, key string, values []string) {
	if len(values) > 0 {
		table[key] = values
	}
}

func putNonEmptyMap(table map[string]any, key string, values map[string]string) {
	if len(values) > 0 {
		table[key] = values
	}
}
