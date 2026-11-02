package builtin

import (
	"encoding/json"
	"slices"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	descriptorKeywordCatalog     = "catalog"
	descriptorKeywordClient      = "client"
	descriptorKeywordStatus      = "status"
	descriptorKeywordPause       = "pause"
	descriptorKeywordResume      = "resume"
	descriptorKeywordArchive     = "archive"
	descriptorKeywordAgents      = "agents"
	descriptorKeywordCreate      = "create"
	descriptorKeywordDiagnostics = "diagnostics"
	descriptorKeywordAudit       = "audit"
	descriptorKeywordSearch      = "search"
	descriptorKeywordSnapshot    = "snapshot"
	descriptorKeywordUpdate      = "update"
	descriptorKeywordValidate    = "validate"
	descriptorKeywordCancel      = "cancel"
	descriptorKeywordDestructive = "destructive"
	descriptorKeywordNode        = "node"
	descriptorKeywordReplace     = "replace"
	descriptorKeywordAgent       = "agent"
	descriptorKeywordSession     = "session"
	emptyInputSchema             = `{"type":"object","additionalProperties":false}`
)

// NativeDescriptors returns the MVP native_go built-in descriptors.
func NativeDescriptors() []toolspkg.Descriptor {
	groups := [][]toolspkg.Descriptor{
		catalogDescriptors(),
		cmdPaletteDescriptors(),
		toolArtifactDescriptors(),
		toolApprovalDescriptors(),
		clarifyDescriptors(),
		skillDescriptors(),
		sessionDescriptors(),
		subagentDescriptors(),
		sessionAttentionDescriptors(),
		sessionMonitorDescriptors(),
		sessionProcessDescriptors(),
		sessionOrchestrationDescriptors(),
		authoredContextDescriptors(),
		agentInstanceDescriptors(),
		agentMemoryDescriptors(),
		workspaceDescriptors(),
		profileDescriptors(),
		vaultDescriptors(),
		worktreeDescriptors(),
		providerModelsDescriptors(),
		providerUsageDescriptors(),
		observeDescriptors(),
		gatewayDescriptors(),
		taskDescriptors(),
		autonomyDescriptors(),
		configDescriptors(),
		hookDescriptors(),
		loopDescriptors(),
		automationDescriptors(),
		marketplaceDescriptors(),
		extensionDescriptors(),
		resourceDescriptors(),
		terminalDescriptors(),
		windowManagerDescriptors(),
		mcpDescriptors(),
		mcpAuthDescriptors(),
	}
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	descriptors := make([]toolspkg.Descriptor, 0, total)
	for _, group := range groups {
		for _, descriptor := range group {
			descriptors = append(descriptors, cloneDescriptor(descriptor))
		}
	}
	return descriptors
}

func withRequiredCapabilities(descriptor toolspkg.Descriptor, capabilities ...string) toolspkg.Descriptor {
	descriptor.Backend.RequiresCapabilities = append([]string(nil), capabilities...)
	return descriptor
}

func nativeDescriptor(
	id toolspkg.ToolID,
	nativeName string,
	title string,
	description string,
	inputSchema string,
	risk toolspkg.RiskClass,
	readOnly bool,
	destructive bool,
	openWorld bool,
	toolsets []toolspkg.ToolsetID,
	tags []string,
	searchHints []string,
) toolspkg.Descriptor {
	return toolspkg.Descriptor{
		ID:               id,
		Backend:          toolspkg.BackendRef{Kind: toolspkg.BackendNativeGo, NativeName: nativeName},
		ToolPresentation: toolspkg.NewToolPresentation(title, "", ""),
		Description:      description,
		InputSchema:      nativeProfileInputSchema(inputSchema, readOnly),
		OutputSchema:     json.RawMessage(`{"type":"object"}`),
		Source:           Source(),
		Visibility:       toolspkg.VisibilityModel,
		Risk:             risk,
		ReadOnly:         readOnly,
		Destructive:      destructive,
		OpenWorld:        openWorld,
		ConcurrencySafe:  true,
		Toolsets:         cloneToolsets(toolsets),
		Tags:             cloneStrings(tags),
		SearchHints:      cloneStrings(searchHints),
	}
}

func cloneDescriptor(src toolspkg.Descriptor) toolspkg.Descriptor {
	cloned := src
	cloned.ToolPresentation = toolspkg.CloneToolPresentation(src.ToolPresentation)
	cloned.ToolExecutionMetadata = toolspkg.CloneToolExecutionMetadata(src.ToolExecutionMetadata)
	cloned.InputSchema = cloneRawMessage(src.InputSchema)
	cloned.OutputSchema = cloneRawMessage(src.OutputSchema)
	cloned.Toolsets = cloneToolsets(src.Toolsets)
	cloned.Tags = cloneStrings(src.Tags)
	cloned.SearchHints = cloneStrings(src.SearchHints)
	return cloned
}

func cloneRawMessage(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	return slices.Clone(src)
}

func cloneToolsets(src []toolspkg.ToolsetID) []toolspkg.ToolsetID {
	if len(src) == 0 {
		return nil
	}
	return slices.Clone(src)
}

func cloneStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	return slices.Clone(src)
}

func nativeProfileInputSchema(input string, readOnly bool) json.RawMessage {
	var schema map[string]json.RawMessage
	if json.Unmarshal([]byte(input), &schema) != nil {
		return json.RawMessage(input)
	}
	var properties map[string]json.RawMessage
	if json.Unmarshal(schema["properties"], &properties) != nil {
		return json.RawMessage(input)
	}
	if _, ok := properties["workspace"]; !ok {
		return json.RawMessage(input)
	}
	properties["profile"] = json.RawMessage(`{"type":"string"}`)
	if readOnly {
		properties["all_profiles"] = json.RawMessage(`{"type":"boolean"}`)
	}
	encoded, err := json.Marshal(properties)
	if err != nil {
		return json.RawMessage(input)
	}
	schema["properties"] = encoded
	encoded, err = json.Marshal(schema)
	if err != nil {
		return json.RawMessage(input)
	}
	return encoded
}
