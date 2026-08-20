package compozysdk

import (
	"encoding/json"
	"io"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/sdk/go/contracts"
)

const describeArgument = "__describe"

// Describe returns the deterministic build-time contract assembled from SDK registrations.
func (e *Extension) Describe() (contracts.DescribePayload, error) {
	if e == nil {
		return contracts.DescribePayload{}, NewInternalError("extension is required")
	}

	e.mu.RLock()
	defer e.mu.RUnlock()
	if err := e.definition.validate(); err != nil {
		return contracts.DescribePayload{}, err
	}
	if strings.TrimSpace(e.definition.Subprocess.Command) == "" {
		return contracts.DescribePayload{}, NewInvalidParamsError("subprocess command is required", nil)
	}

	tools := make([]contracts.ExtensionToolRuntimeDescriptor, 0, len(e.toolHandlers))
	for _, registered := range e.toolHandlers {
		descriptor := registered.descriptor
		tools = append(tools, contracts.ExtensionToolRuntimeDescriptor{
			ID:                  contracts.ToolID(descriptor.ID),
			Handler:             descriptor.Handler,
			Description:         descriptor.Description,
			FriendlyVerb:        descriptor.FriendlyVerb,
			Preview:             descriptor.Preview,
			InputSchema:         cloneRawMessage(descriptor.InputSchema),
			OutputSchema:        cloneRawMessage(descriptor.OutputSchema),
			InputSchemaDigest:   descriptor.InputSchemaDigest,
			OutputSchemaDigest:  descriptor.OutputSchemaDigest,
			ReadOnly:            descriptor.ReadOnly,
			Risk:                contracts.RiskClass(descriptor.Risk),
			RequiresInteraction: descriptor.RequiresInteraction,
			Capabilities:        slices.Clone(descriptor.Capabilities),
			Command:             cloneSDKCommandSpec(descriptor.Command),
		})
	}
	slices.SortFunc(tools, func(left, right contracts.ExtensionToolRuntimeDescriptor) int {
		return strings.Compare(left.Handler, right.Handler)
	})

	permissions := make([]string, 0, len(e.definition.Permissions.Requires))
	for _, method := range e.definition.Permissions.Requires {
		permissions = append(permissions, string(method))
	}

	return contracts.DescribePayload{
		Name:        strings.TrimSpace(e.definition.Name),
		Version:     strings.TrimSpace(e.definition.Version),
		Description: strings.TrimSpace(e.definition.Description),
		Provides:    requiredStringList(e.definition.Capabilities.Provides),
		Permissions: requiredStringList(permissions),
		RequiresEnv: normalizeStrings(e.definition.RequiresEnv),
		Resources: contracts.DescribeResources{
			Skills:     normalizeStrings(e.definition.Resources.Skills),
			Loops:      normalizeStrings(e.definition.Resources.Loops),
			Agents:     normalizeStrings(e.definition.Resources.Agents),
			Automation: normalizeStrings(e.definition.Resources.Automation),
			Layouts:    normalizeStrings(e.definition.Resources.Layouts),
			CmdPalette: cloneCmdPaletteConfig(e.definition.Resources.CmdPalette),
		},
		Subprocess: contracts.DescribeSubprocess{
			Command: strings.TrimSpace(e.definition.Subprocess.Command),
			Args:    slices.Clone(e.definition.Subprocess.Args),
			Env:     cloneStringMap(e.definition.Subprocess.Env),
		},
		NetworkParticipation: normalizeDescribeNetworkParticipation(e.definition.NetworkParticipation),
		Tools:                tools,
		CommandGroups:        e.commandGroupsLocked(),
		HookEvents:           normalizeStrings(e.definition.SupportedHookEvents),
		WatchSourceKinds:     e.watchSourceKindsLocked(),
		CmdPaletteViews:      cmdPaletteViewIDs(e.definition.Resources.CmdPalette),
		SDK: contracts.DescribeSDKInfo{
			Name:              SDKName,
			Version:           e.sdkVersion,
			ProtocolVersion:   ProtocolVersion,
			MinCompozyVersion: MinCompozyVersion,
		},
	}, nil
}

func cmdPaletteViewIDs(config contracts.CmdPaletteConfig) []string {
	ids := make([]string, 0, len(config.Views))
	for _, view := range config.Views {
		id := strings.TrimSpace(view.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func cloneCmdPaletteConfig(value contracts.CmdPaletteConfig) contracts.CmdPaletteConfig {
	cloned := contracts.CmdPaletteConfig{
		Commands: slices.Clone(value.Commands),
		Views:    slices.Clone(value.Views),
	}
	for index := range cloned.Commands {
		command := &cloned.Commands[index]
		command.Keywords = slices.Clone(value.Commands[index].Keywords)
		command.Arguments = slices.Clone(value.Commands[index].Arguments)
		for argumentIndex := range command.Arguments {
			command.Arguments[argumentIndex].Options = slices.Clone(
				value.Commands[index].Arguments[argumentIndex].Options,
			)
		}
		command.Action.Args = cloneJSONMap(value.Commands[index].Action.Args)
		if value.Commands[index].Confirmation != nil {
			confirmation := *value.Commands[index].Confirmation
			command.Confirmation = &confirmation
		}
		if value.Commands[index].Execution != nil {
			execution := *value.Commands[index].Execution
			execution.SingleFlight = cloneOptionalBool(value.Commands[index].Execution.SingleFlight)
			execution.RetrySafe = cloneOptionalBool(value.Commands[index].Execution.RetrySafe)
			command.Execution = &execution
		}
	}
	for index := range cloned.Views {
		if value.Views[index].Source != nil {
			source := *value.Views[index].Source
			cloned.Views[index].Source = &source
		}
	}
	return cloned
}

func cloneJSONMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, item := range values {
		cloned[key] = cloneJSONValue(item)
	}
	return cloned
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case json.RawMessage:
		return slices.Clone(typed)
	case map[string]any:
		return cloneJSONMap(typed)
	case map[string]string:
		return maps.Clone(typed)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneJSONValue(item)
		}
		return cloned
	case []string:
		return slices.Clone(typed)
	case []map[string]any:
		cloned := make([]map[string]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneJSONMap(item)
		}
		return cloned
	case []map[string]string:
		cloned := make([]map[string]string, len(typed))
		for index, item := range typed {
			cloned[index] = maps.Clone(item)
		}
		return cloned
	default:
		return typed
	}
}

func cloneOptionalBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func normalizeDescribeNetworkParticipation(
	value *NetworkParticipationRequirement,
) *contracts.DescribeNetworkParticipation {
	if value == nil {
		return nil
	}
	return &contracts.DescribeNetworkParticipation{
		Required: value.Required, Mode: strings.TrimSpace(value.Mode),
		ChannelScopes: normalizeStrings(value.ChannelScopes),
	}
}

func (e *Extension) writeDescribe(output io.Writer) error {
	payload, err := e.Describe()
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return NewInternalError("encode describe payload: " + err.Error())
	}
	return nil
}

func describeModeRequested(args []string) bool {
	return len(args) > 1 && strings.TrimSpace(args[len(args)-1]) == describeArgument
}

func requiredStringList(values []string) []string {
	result := normalizeStrings(values)
	if result == nil {
		return []string{}
	}
	return result
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	return maps.Clone(values)
}
