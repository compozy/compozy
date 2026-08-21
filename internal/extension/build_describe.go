package extensionpkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func decodeDescribePayload(data []byte) (extensioncontract.DescribePayload, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var payload extensioncontract.DescribePayload
	if err := decoder.Decode(&payload); err != nil {
		return extensioncontract.DescribePayload{}, fmt.Errorf("extension: decode describe payload: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return extensioncontract.DescribePayload{}, errors.New(
				"extension: describe output contains multiple JSON values",
			)
		}
		return extensioncontract.DescribePayload{}, fmt.Errorf("extension: decode trailing describe output: %w", err)
	}
	return payload, nil
}

func manifestFromDescribe(input *extensioncontract.DescribePayload) (*Manifest, error) {
	if input == nil {
		return nil, errors.New("extension: describe payload is required")
	}
	payload := *input
	payload.Provides = sortedBuildStrings(payload.Provides)
	payload.Permissions = sortedBuildStrings(payload.Permissions)
	payload.RequiresEnv = sortedBuildStrings(payload.RequiresEnv)
	payload.Profiles = normalizeDescribeProfiles(payload.Profiles)
	payload.Resources = normalizeDescribeResources(payload.Resources)
	if err := validateDescribeCapabilityCoverage(&payload); err != nil {
		return nil, err
	}
	manifest := &Manifest{
		Name:              strings.TrimSpace(payload.Name),
		Version:           strings.TrimSpace(payload.Version),
		Description:       strings.TrimSpace(payload.Description),
		MinCompozyVersion: strings.TrimSpace(payload.SDK.MinCompozyVersion),
		RequiresEnv:       payload.RequiresEnv,
		Profiles:          manifestProfilesFromDescribe(payload.Profiles),
		Resources: ResourcesConfig{
			Skills:     manifestResourcePathsFromDescribe(payload.Resources.Skills),
			Loops:      manifestResourcePathsFromDescribe(payload.Resources.Loops),
			Agents:     manifestResourcePathsFromDescribe(payload.Resources.Agents),
			Automation: manifestResourcePathsFromDescribe(payload.Resources.Automation),
			Layouts:    manifestResourcePathsFromDescribe(payload.Resources.Layouts),
			CmdPalette: payload.Resources.CmdPalette,
		},
		Capabilities: CapabilitiesConfig{
			Provides: payload.Provides,
		},
		Permissions: PermissionsConfig{
			Requires: payload.Permissions,
		},
		Subprocess: SubprocessConfig{
			Command: strings.TrimSpace(payload.Subprocess.Command),
			Args:    slices.Clone(payload.Subprocess.Args),
			Env:     cloneStringMap(payload.Subprocess.Env),
		},
	}
	if payload.NetworkParticipation != nil {
		manifest.NetworkParticipation = (&NetworkParticipationRequirement{
			Required:      payload.NetworkParticipation.Required,
			Mode:          strings.TrimSpace(payload.NetworkParticipation.Mode),
			ChannelScopes: slices.Clone(payload.NetworkParticipation.ChannelScopes),
		}).Normalize()
	}
	if err := validateDescribeSDK(payload.SDK); err != nil {
		return nil, err
	}

	tools, err := manifestToolsFromDescribe(payload.Tools)
	if err != nil {
		return nil, err
	}
	manifest.Resources.Tools = tools
	manifest.Resources.CommandGroups = cloneCommandGroups(payload.CommandGroups)
	manifest.Resources.Hooks, err = manifestHooksFromDescribe(payload.HookEvents, payload.Subprocess)
	if err != nil {
		return nil, err
	}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("extension: validate describe manifest: %w", err)
	}
	if _, err := ResolveManifestToolDescriptors(manifest); err != nil {
		return nil, fmt.Errorf("extension: validate described tools: %w", err)
	}
	return manifest, nil
}

func normalizeDescribeResources(resources extensioncontract.DescribeResources) extensioncontract.DescribeResources {
	return extensioncontract.DescribeResources{
		Skills:     normalizeDescribeResourcePaths(resources.Skills),
		Loops:      normalizeDescribeResourcePaths(resources.Loops),
		Agents:     normalizeDescribeResourcePaths(resources.Agents),
		Automation: normalizeDescribeResourcePaths(resources.Automation),
		Layouts:    normalizeDescribeResourcePaths(resources.Layouts),
		CmdPalette: normalizeCmdPaletteConfig(resources.CmdPalette),
	}
}

func normalizeDescribeResourcePaths(
	resources []extensioncontract.DescribeResourcePath,
) []extensioncontract.DescribeResourcePath {
	normalized := make([]extensioncontract.DescribeResourcePath, 0, len(resources))
	for _, resource := range resources {
		normalized = append(normalized, extensioncontract.DescribeResourcePath{
			Path: strings.TrimSpace(resource.Path), Profile: strings.TrimSpace(resource.Profile),
		})
	}
	slices.SortFunc(normalized, func(left, right extensioncontract.DescribeResourcePath) int {
		if compared := strings.Compare(left.Path, right.Path); compared != 0 {
			return compared
		}
		return strings.Compare(left.Profile, right.Profile)
	})
	return slices.Compact(normalized)
}

func manifestResourcePathsFromDescribe(
	resources []extensioncontract.DescribeResourcePath,
) []ManifestResourcePath {
	result := make([]ManifestResourcePath, 0, len(resources))
	for _, resource := range resources {
		result = append(result, ManifestResourcePath{Path: resource.Path, Profile: resource.Profile})
	}
	return result
}

func normalizeDescribeProfiles(profiles []extensioncontract.DescribeProfile) []extensioncontract.DescribeProfile {
	normalized := make([]extensioncontract.DescribeProfile, 0, len(profiles))
	for _, profile := range profiles {
		credentials := make([]extensioncontract.DescribeProfileCredential, 0, len(profile.Credentials))
		for _, credential := range profile.Credentials {
			credentials = append(credentials, extensioncontract.DescribeProfileCredential{
				Provider: strings.TrimSpace(credential.Provider), Slot: strings.TrimSpace(credential.Slot),
			})
		}
		slices.SortFunc(credentials, func(left, right extensioncontract.DescribeProfileCredential) int {
			if compared := strings.Compare(left.Provider, right.Provider); compared != 0 {
				return compared
			}
			return strings.Compare(left.Slot, right.Slot)
		})
		normalized = append(normalized, extensioncontract.DescribeProfile{
			Name: strings.TrimSpace(profile.Name), Color: strings.TrimSpace(profile.Color),
			Icon: strings.TrimSpace(profile.Icon), Emoji: strings.TrimSpace(profile.Emoji),
			Defaults: extensioncontract.DescribeProfileDefaults{
				Agent:    strings.TrimSpace(profile.Defaults.Agent),
				Provider: strings.TrimSpace(profile.Defaults.Provider),
				Sandbox:  strings.TrimSpace(profile.Defaults.Sandbox),
			},
			Credentials: slices.Compact(credentials),
		})
	}
	slices.SortFunc(normalized, func(left, right extensioncontract.DescribeProfile) int {
		return strings.Compare(left.Name, right.Name)
	})
	return normalized
}

func manifestProfilesFromDescribe(profiles []extensioncontract.DescribeProfile) []ManifestProfile {
	result := make([]ManifestProfile, 0, len(profiles))
	for _, profile := range profiles {
		credentials := make([]ManifestProfileCredential, 0, len(profile.Credentials))
		for _, credential := range profile.Credentials {
			credentials = append(credentials, ManifestProfileCredential{
				Provider: credential.Provider, Slot: credential.Slot,
			})
		}
		result = append(result, ManifestProfile{
			Name: profile.Name, Color: profile.Color, Icon: profile.Icon, Emoji: profile.Emoji,
			Defaults: ManifestProfileDefaults{
				Agent: profile.Defaults.Agent, Provider: profile.Defaults.Provider, Sandbox: profile.Defaults.Sandbox,
			},
			Credentials: credentials,
		})
	}
	return result
}

func validateDescribeSDK(info extensioncontract.DescribeSDKInfo) error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "sdk.name", value: info.Name},
		{name: "sdk.version", value: info.Version},
		{name: "sdk.protocol_version", value: info.ProtocolVersion},
		{name: "sdk.min_compozy_version", value: info.MinCompozyVersion},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			return &ManifestValidationError{Field: field.name, Message: "value is required"}
		}
	}
	return nil
}

func manifestToolsFromDescribe(
	descriptors []toolspkg.ExtensionToolRuntimeDescriptor,
) (map[string]ToolConfig, error) {
	if len(descriptors) == 0 {
		return nil, nil
	}
	result := make(map[string]ToolConfig, len(descriptors))
	for idx, descriptor := range descriptors {
		if err := descriptor.Validate(); err != nil {
			return nil, fmt.Errorf("extension: validate described tool %d: %w", idx, err)
		}
		if err := validateDescribeToolSchemas(descriptor); err != nil {
			return nil, fmt.Errorf("extension: validate described tool %q: %w", descriptor.Handler, err)
		}
		handler := strings.TrimSpace(descriptor.Handler)
		if _, exists := result[handler]; exists {
			return nil, &ManifestValidationError{
				Field:   fmt.Sprintf("tools[%d].handler", idx),
				Value:   handler,
				Message: "duplicate tool handler",
			}
		}
		result[handler] = ToolConfig{
			Profile:      strings.TrimSpace(descriptor.Profile),
			ID:           descriptor.ID.String(),
			Description:  strings.TrimSpace(descriptor.Description),
			FriendlyVerb: strings.TrimSpace(descriptor.FriendlyVerb),
			Preview:      strings.TrimSpace(descriptor.Preview),
			Handler:      handler,
			Backend: ToolBackendConfig{
				Kind:    string(toolspkg.BackendExtensionHost),
				Handler: handler,
			},
			InputSchema:          cloneRawMessage(descriptor.InputSchema),
			OutputSchema:         cloneRawMessage(descriptor.OutputSchema),
			Risk:                 string(descriptor.Risk),
			ReadOnly:             descriptor.ReadOnly,
			RequiresInteraction:  descriptor.RequiresInteraction,
			Destructive:          descriptor.Risk == toolspkg.RiskDestructive,
			OpenWorld:            descriptor.Risk == toolspkg.RiskOpenWorld,
			ConcurrencySafe:      descriptor.ReadOnly,
			RequiredCapabilities: normalizeStrings(descriptor.Capabilities),
			Visibility:           string(toolspkg.VisibilityModel),
			Command:              cloneExtensionCommandSpec(descriptor.Command),
		}
	}
	return result, nil
}

func validateDescribeToolSchemas(descriptor toolspkg.ExtensionToolRuntimeDescriptor) error {
	inputDigest, err := toolspkg.SchemaDigest(descriptor.InputSchema)
	if err != nil {
		return fmt.Errorf("input_schema: %w", err)
	}
	if inputDigest != strings.TrimSpace(descriptor.InputSchemaDigest) {
		return errors.New("input_schema_digest does not match input_schema")
	}
	if len(bytes.TrimSpace(descriptor.OutputSchema)) == 0 {
		if strings.TrimSpace(descriptor.OutputSchemaDigest) != "" {
			return errors.New("output_schema_digest requires output_schema")
		}
		return nil
	}
	outputDigest, err := toolspkg.SchemaDigest(descriptor.OutputSchema)
	if err != nil {
		return fmt.Errorf("output_schema: %w", err)
	}
	if outputDigest != strings.TrimSpace(descriptor.OutputSchemaDigest) {
		return errors.New("output_schema_digest does not match output_schema")
	}
	return nil
}

func manifestHooksFromDescribe(
	events []extensioncontract.DescribeHookEvent,
	process extensioncontract.DescribeSubprocess,
) ([]HookConfig, error) {
	normalized := normalizeDescribeHookEvents(events)
	if len(normalized) == 0 {
		return nil, nil
	}
	hooks := make([]HookConfig, 0, len(normalized))
	for _, described := range normalized {
		event := described.Event
		if err := event.Validate(); err != nil {
			return nil, err
		}
		mode := string(hookspkg.HookModeAsync)
		if event.SyncEligible() {
			mode = string(hookspkg.HookModeSync)
		}
		hooks = append(hooks, HookConfig{
			Profile: described.Profile,
			Name:    strings.ReplaceAll(string(described.Event), ".", "-"),
			Event:   string(described.Event),
			Mode:    mode,
			Executor: HookExecutorConfig{
				Kind:    describeSubprocessKey,
				Command: strings.TrimSpace(process.Command),
				Args:    slices.Clone(process.Args),
				Env:     cloneStringMap(process.Env),
			},
		})
	}
	return hooks, nil
}

func normalizeDescribeHookEvents(events []extensioncontract.DescribeHookEvent) []extensioncontract.DescribeHookEvent {
	normalized := make([]extensioncontract.DescribeHookEvent, 0, len(events))
	for _, event := range events {
		normalized = append(normalized, extensioncontract.DescribeHookEvent{
			Event:   hookspkg.HookEvent(strings.TrimSpace(string(event.Event))),
			Profile: strings.TrimSpace(event.Profile),
		})
	}
	slices.SortFunc(normalized, func(left, right extensioncontract.DescribeHookEvent) int {
		if compared := strings.Compare(string(left.Event), string(right.Event)); compared != 0 {
			return compared
		}
		return strings.Compare(left.Profile, right.Profile)
	})
	return slices.Compact(normalized)
}

func sortedBuildStrings(values []string) []string {
	normalized := normalizeStrings(values)
	slices.Sort(normalized)
	return normalized
}

func validateDescribeCapabilityCoverage(payload *extensioncontract.DescribePayload) error {
	if len(payload.Tools) > 0 && !slices.Contains(payload.Provides, extensionprotocol.CapabilityToolProvider) {
		return errors.New("extension: described tools require tool.provider")
	}
	if len(payload.WatchSourceKinds) > 0 &&
		!slices.Contains(payload.Provides, extensionprotocol.CapabilityProvideWatchSource) {
		return errors.New("extension: described watch sources require loop.watch_source")
	}
	return nil
}
