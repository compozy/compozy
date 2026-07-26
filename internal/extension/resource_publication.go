package extensionpkg

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	toolspkg "github.com/compozy/agh/internal/tools"
)

var defaultManifestToolInputSchema = json.RawMessage(`{"type":"object"}`)

// ManifestToolDescriptor is a manifest-authoritative cold descriptor plus runtime proof metadata.
type ManifestToolDescriptor struct {
	Name              string
	Tool              toolspkg.Tool
	RuntimeDescriptor toolspkg.ExtensionToolRuntimeDescriptor
}

// ResolveManifestToolDescriptors converts manifest tool declarations into cold specs and digest proofs.
func ResolveManifestToolDescriptors(manifest *Manifest) ([]ManifestToolDescriptor, error) {
	if manifest == nil || len(manifest.Resources.Tools) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(manifest.Resources.Tools))
	for name := range manifest.Resources.Tools {
		names = append(names, name)
	}
	slices.Sort(names)

	descriptors := make([]ManifestToolDescriptor, 0, len(names))
	for _, name := range names {
		descriptor, err := resolveManifestToolDescriptor(
			strings.TrimSpace(manifest.Name),
			name,
			manifest.Resources.Tools[name],
		)
		if err != nil {
			return nil, err
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, nil
}

// ResolveManifestToolResources converts manifest tool declarations into tool specs.
func ResolveManifestToolResources(manifest *Manifest) ([]toolspkg.Tool, error) {
	descriptors, err := ResolveManifestToolDescriptors(manifest)
	if err != nil {
		return nil, err
	}
	tools := make([]toolspkg.Tool, 0, len(descriptors))
	for i := range descriptors {
		tools = append(tools, descriptors[i].Tool)
	}
	return tools, nil
}

func resolveManifestToolDescriptor(
	extensionName string,
	name string,
	cfg ToolConfig,
) (ManifestToolDescriptor, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return ManifestToolDescriptor{}, toolspkg.NewValidationError(
			"name",
			toolspkg.ReasonIDEmpty,
			"tool name is required",
		)
	}
	id, err := manifestToolID(extensionName, trimmedName, cfg.ID)
	if err != nil {
		return ManifestToolDescriptor{}, fmt.Errorf("extension: tool %q id: %w", trimmedName, err)
	}
	backend, source, err := manifestToolBackend(extensionName, trimmedName, cfg)
	if err != nil {
		return ManifestToolDescriptor{}, fmt.Errorf("extension: tool %q backend: %w", trimmedName, err)
	}
	inputSchema := cloneRawMessage(cfg.InputSchema)
	if len(inputSchema) == 0 {
		inputSchema = cloneRawMessage(defaultManifestToolInputSchema)
	}
	outputSchema := cloneRawMessage(cfg.OutputSchema)
	risk, err := manifestToolRisk(cfg)
	if err != nil {
		return ManifestToolDescriptor{}, fmt.Errorf("extension: tool %q risk: %w", trimmedName, err)
	}
	toolsets, err := manifestToolsets(cfg.Toolsets)
	if err != nil {
		return ManifestToolDescriptor{}, fmt.Errorf("extension: tool %q toolsets: %w", trimmedName, err)
	}
	visibility, err := manifestToolVisibility(cfg.Visibility)
	if err != nil {
		return ManifestToolDescriptor{}, fmt.Errorf("extension: tool %q visibility: %w", trimmedName, err)
	}
	concurrencySafe := cfg.ConcurrencySafe
	if !concurrencySafe && cfg.ReadOnly {
		concurrencySafe = true
	}
	tool := toolspkg.Tool{
		ID:           id,
		DisplayTitle: manifestToolDisplayTitle(trimmedName, cfg.DisplayTitle),
		ToolPresentation: toolspkg.NewToolPresentation(
			strings.TrimSpace(cfg.FriendlyVerb),
			strings.TrimSpace(cfg.Preview),
		),
		Description:         strings.TrimSpace(cfg.Description),
		Backend:             backend,
		InputSchema:         inputSchema,
		OutputSchema:        outputSchema,
		Source:              source,
		Visibility:          visibility,
		Risk:                risk,
		ReadOnly:            cfg.ReadOnly,
		Destructive:         cfg.Destructive,
		OpenWorld:           cfg.OpenWorld,
		RequiresInteraction: cfg.RequiresInteraction,
		ConcurrencySafe:     concurrencySafe,
		MaxResultBytes:      cfg.MaxResultBytes,
		Toolsets:            toolsets,
		Tags:                normalizeStrings(cfg.Tags),
		SearchHints:         normalizeStrings(cfg.SearchHints),
	}
	runtimeDescriptor, err := manifestRuntimeDescriptor(tool)
	if err != nil {
		return ManifestToolDescriptor{}, err
	}
	tool.InputSchemaDigest = runtimeDescriptor.InputSchemaDigest
	tool.OutputSchemaDigest = runtimeDescriptor.OutputSchemaDigest
	if err := tool.Validate(); err != nil {
		return ManifestToolDescriptor{}, fmt.Errorf("descriptor: %w", err)
	}
	return ManifestToolDescriptor{
		Name:              trimmedName,
		Tool:              tool,
		RuntimeDescriptor: runtimeDescriptor,
	}, nil
}

func manifestToolID(extensionName string, name string, explicit string) (toolspkg.ToolID, error) {
	namespace, err := toolspkg.CanonicalToolID("ext", extensionName)
	if err != nil {
		return "", err
	}
	id := toolspkg.ToolID(strings.TrimSpace(explicit))
	if id == "" {
		id, err = toolspkg.CanonicalToolID("ext", extensionName, name)
		if err != nil {
			return "", err
		}
	}
	if err := id.Validate(); err != nil {
		return "", err
	}
	if strings.HasPrefix(id.String(), "agh__") {
		return "", toolspkg.NewValidationError(
			"tool_id",
			toolspkg.ReasonReservedNamespace,
			"extension tools cannot claim agh__ namespace",
		)
	}
	if !strings.HasPrefix(id.String(), namespace.String()+"__") {
		return "", toolspkg.NewValidationError(
			"tool_id",
			toolspkg.ReasonReservedNamespace,
			fmt.Sprintf(
				"extension tools must stay under %q namespace",
				namespace.String()+"__",
			),
		)
	}
	return id, nil
}

func manifestToolBackend(
	extensionName string,
	name string,
	cfg ToolConfig,
) (toolspkg.BackendRef, toolspkg.SourceRef, error) {
	kind := toolspkg.BackendKind(strings.TrimSpace(cfg.Backend.Kind))
	switch kind {
	case toolspkg.BackendExtensionHost:
		handler, err := manifestToolHandler(cfg)
		if err != nil {
			return toolspkg.BackendRef{}, toolspkg.SourceRef{}, err
		}
		return toolspkg.BackendRef{
				Kind:        toolspkg.BackendExtensionHost,
				ExtensionID: strings.TrimSpace(extensionName),
				Handler:     handler,
				RequiresCapabilities: mergeRequiredCapabilities(
					cfg.RequiredCapabilities,
					extensionprotocol.CapabilityToolProvider,
				),
			},
			toolspkg.SourceRef{
				Kind:        toolspkg.SourceExtension,
				Owner:       strings.TrimSpace(extensionName),
				RawToolName: name,
			},
			nil
	case toolspkg.BackendMCP:
		server := strings.TrimSpace(cfg.Backend.Server)
		tool := strings.TrimSpace(cfg.Backend.Tool)
		if server == "" {
			return toolspkg.BackendRef{}, toolspkg.SourceRef{}, toolspkg.NewValidationError(
				"backend.server",
				toolspkg.ReasonMCPUnreachable,
				"mcp backend requires server",
			)
		}
		if tool == "" {
			return toolspkg.BackendRef{}, toolspkg.SourceRef{}, toolspkg.NewValidationError(
				"backend.tool",
				toolspkg.ReasonDependencyMissing,
				"mcp backend requires tool",
			)
		}
		return toolspkg.BackendRef{
				Kind:      toolspkg.BackendMCP,
				MCPServer: server,
				MCPTool:   tool,
			},
			toolspkg.SourceRef{
				Kind:          toolspkg.SourceMCP,
				Owner:         server,
				RawServerName: server,
				RawToolName:   tool,
			},
			nil
	case "":
		return toolspkg.BackendRef{}, toolspkg.SourceRef{}, toolspkg.NewValidationError(
			"backend.kind",
			toolspkg.ReasonBackendNotExecutable,
			"tool backend kind is required",
		)
	default:
		return toolspkg.BackendRef{}, toolspkg.SourceRef{}, toolspkg.NewValidationError(
			"backend.kind",
			toolspkg.ReasonBackendNotExecutable,
			"extension manifest tools support extension_host or mcp backends",
		)
	}
}

func manifestToolHandler(cfg ToolConfig) (string, error) {
	handler := strings.TrimSpace(cfg.Backend.Handler)
	topLevel := strings.TrimSpace(cfg.Handler)
	if topLevel != "" {
		if handler != "" && handler != topLevel {
			return "", toolspkg.NewValidationError(
				"handler",
				toolspkg.ReasonHandlerMissing,
				"handler fields conflict",
			)
		}
		handler = topLevel
	}
	if handler == "" {
		return "", toolspkg.NewValidationError(
			"handler",
			toolspkg.ReasonHandlerMissing,
			"extension_host backend requires handler",
		)
	}
	if !validManifestToolHandler(handler) {
		return "", toolspkg.NewValidationError(
			"handler",
			toolspkg.ReasonHandlerMissing,
			"handler contains unsupported characters",
		)
	}
	return handler, nil
}

func validManifestToolHandler(handler string) bool {
	if strings.TrimSpace(handler) == "" {
		return false
	}
	for _, r := range handler {
		if r <= ' ' || r == '/' || r == '\\' || r == '{' || r == '}' {
			return false
		}
	}
	return true
}

func manifestToolRisk(cfg ToolConfig) (toolspkg.RiskClass, error) {
	risk := toolspkg.RiskClass(strings.TrimSpace(cfg.Risk))
	if risk == "" {
		switch {
		case cfg.Destructive:
			return toolspkg.RiskDestructive, nil
		case cfg.OpenWorld:
			return toolspkg.RiskOpenWorld, nil
		case cfg.ReadOnly:
			return toolspkg.RiskRead, nil
		default:
			return toolspkg.RiskMutating, nil
		}
	}
	if err := risk.Validate("risk"); err != nil {
		return "", err
	}
	switch risk {
	case toolspkg.RiskRead:
		if !cfg.ReadOnly {
			return "", toolspkg.NewValidationError("risk", toolspkg.ReasonPolicyDenied, "read risk requires read_only")
		}
	case toolspkg.RiskOpenWorld:
		if !cfg.OpenWorld {
			return "", toolspkg.NewValidationError(
				"risk",
				toolspkg.ReasonPolicyDenied,
				"open_world risk requires open_world",
			)
		}
	case toolspkg.RiskDestructive:
		if !cfg.Destructive {
			return "", toolspkg.NewValidationError(
				"risk",
				toolspkg.ReasonPolicyDenied,
				"destructive risk requires destructive",
			)
		}
	}
	return risk, nil
}

func manifestToolsets(values []string) ([]toolspkg.ToolsetID, error) {
	if len(values) == 0 {
		return nil, nil
	}
	toolsets := make([]toolspkg.ToolsetID, 0, len(values))
	for i, value := range values {
		id := toolspkg.ToolsetID(strings.TrimSpace(value))
		if err := id.Validate(); err != nil {
			return nil, fmt.Errorf("toolsets[%d]: %w", i, err)
		}
		toolsets = append(toolsets, id)
	}
	return toolsets, nil
}

func manifestToolVisibility(value string) (toolspkg.Visibility, error) {
	visibility := toolspkg.Visibility(strings.TrimSpace(value))
	if visibility == "" {
		return toolspkg.VisibilityOperator, nil
	}
	if err := visibility.Validate("visibility"); err != nil {
		return "", err
	}
	return visibility, nil
}

func manifestToolDisplayTitle(name string, value string) string {
	if title := strings.TrimSpace(value); title != "" {
		return title
	}
	return name
}

func manifestRuntimeDescriptor(tool toolspkg.Tool) (toolspkg.ExtensionToolRuntimeDescriptor, error) {
	presentation := tool.Presentation()
	inputDigest, err := toolspkg.SchemaDigest(tool.InputSchema)
	if err != nil {
		return toolspkg.ExtensionToolRuntimeDescriptor{}, fmt.Errorf(
			"extension: tool %q input schema digest: %w",
			tool.ID,
			err,
		)
	}
	var outputDigest string
	if len(tool.OutputSchema) > 0 {
		outputDigest, err = toolspkg.SchemaDigest(tool.OutputSchema)
		if err != nil {
			return toolspkg.ExtensionToolRuntimeDescriptor{}, fmt.Errorf(
				"extension: tool %q output schema digest: %w",
				tool.ID,
				err,
			)
		}
	}
	descriptor := toolspkg.ExtensionToolRuntimeDescriptor{
		ID:                 tool.ID,
		Handler:            tool.Backend.Handler,
		FriendlyVerb:       presentation.FriendlyVerb,
		Preview:            presentation.Preview,
		InputSchemaDigest:  inputDigest,
		OutputSchemaDigest: outputDigest,
		ReadOnly:           tool.ReadOnly,
		Risk:               tool.Risk,
		Capabilities:       append([]string(nil), tool.Backend.RequiresCapabilities...),
	}
	if err := descriptor.Validate(); err != nil {
		return toolspkg.ExtensionToolRuntimeDescriptor{}, err
	}
	return descriptor, nil
}

func mergeRequiredCapabilities(values []string, required string) []string {
	merged := normalizeStrings(values)
	if strings.TrimSpace(required) == "" || slices.Contains(merged, required) {
		return merged
	}
	merged = append(merged, required)
	slices.Sort(merged)
	return merged
}

// ResolveManifestMCPServerResources converts manifest MCP declarations into MCP server specs.
func ResolveManifestMCPServerResources(
	rootDir string,
	manifest *Manifest,
	getenv func(string) string,
) ([]aghconfig.MCPServer, error) {
	if manifest == nil || len(manifest.Resources.MCPServers) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(manifest.Resources.MCPServers))
	for name := range manifest.Resources.MCPServers {
		names = append(names, name)
	}
	slices.Sort(names)

	servers := make([]aghconfig.MCPServer, 0, len(names))
	for _, name := range names {
		decl := manifest.Resources.MCPServers[name]
		command, err := resolveManifestCommand(rootDir, decl.Command, getenv, nil)
		if err != nil {
			return nil, err
		}
		args, err := resolveManifestStringSlice(rootDir, decl.Args, getenv, nil)
		if err != nil {
			return nil, err
		}
		env, err := resolveManifestStringMap(rootDir, decl.Env, getenv, nil)
		if err != nil {
			return nil, err
		}
		server := aghconfig.MCPServer{
			Name:      strings.TrimSpace(name),
			Command:   command,
			Args:      args,
			Env:       env,
			SecretEnv: normalizeStringMap(decl.SecretEnv),
		}
		if err := server.Validate("extension.resources.mcp_servers[" + name + "]"); err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}
	return servers, nil
}
