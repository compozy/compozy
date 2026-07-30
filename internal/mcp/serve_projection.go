package mcp

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"
)

const hostAPIToolPrefix = "compozy_host__"

type workspaceBinding = extensionpkg.HostAPIWorkspaceBinding

const (
	workspaceBindingNone       = extensionpkg.HostAPIWorkspaceBindingNone
	workspaceBindingActor      = extensionpkg.HostAPIWorkspaceBindingActor
	workspaceBindingPath       = extensionpkg.HostAPIWorkspaceBindingPath
	workspaceBindingID         = extensionpkg.HostAPIWorkspaceBindingID
	workspaceBindingTask       = extensionpkg.HostAPIWorkspaceBindingTask
	workspaceBindingMemory     = extensionpkg.HostAPIWorkspaceBindingMemory
	workspaceBindingResource   = extensionpkg.HostAPIWorkspaceBindingResource
	workspaceBindingAutomation = extensionpkg.HostAPIWorkspaceBindingAutomation
)

type projectionDecision struct {
	Publish bool
	Reason  string
}

// ProjectedHostAPIMethods returns the canonical, ordered Host API subset exposed by MCP serve.
func ProjectedHostAPIMethods() []string {
	methods := make([]string, 0, len(hostAPIProjectionDecisions))
	for _, method := range extensionprotocol.AllHostAPIMethods() {
		decision, ok := hostAPIProjectionDecisions[method]
		if ok && decision.Publish {
			methods = append(methods, string(method))
		}
	}
	return methods
}

// HostAPIProjectionCoverage reports methods missing an explicit publish or exclusion decision.
func HostAPIProjectionCoverage() []string {
	missing := make([]string, 0)
	for _, method := range extensionprotocol.AllHostAPIMethods() {
		if _, ok := hostAPIProjectionDecisions[method]; !ok {
			missing = append(missing, string(method))
		}
	}
	slices.Sort(missing)
	return missing
}

func projectedHostAPITools() ([]mcpgo.Tool, error) {
	specs := make(map[extensionprotocol.HostAPIMethod]extensioncontract.HostAPIMethodSpec)
	for _, spec := range extensioncontract.HostAPIMethodSpecs() {
		specs[spec.Method] = spec
	}

	tools := make([]mcpgo.Tool, 0, len(hostAPIProjectionDecisions))
	for _, method := range extensionprotocol.AllHostAPIMethods() {
		decision, ok := hostAPIProjectionDecisions[method]
		if !ok || !decision.Publish {
			continue
		}
		spec, ok := specs[method]
		if !ok {
			return nil, fmt.Errorf("mcp: host api method %q has no canonical contract", method)
		}
		binding, ok := extensionpkg.HostAPIWorkspaceBindingFor(method)
		if !ok || binding == workspaceBindingNone {
			return nil, fmt.Errorf("mcp: host api method %q has no workspace binding", method)
		}
		rawSchema, err := hostAPIInputSchema(spec, binding)
		if err != nil {
			return nil, fmt.Errorf("mcp: build input schema for %q: %w", method, err)
		}
		tools = append(tools, mcpgo.Tool{
			Name:        hostAPIToolName(method),
			Description: "Invoke the Compozy Host API method " + string(method) + " in the workspace bound to this MCP server.",
			InputSchema: rawSchema,
		})
	}
	return tools, nil
}

func hostAPIInputSchema(
	spec extensioncontract.HostAPIMethodSpec,
	binding workspaceBinding,
) (json.RawMessage, error) {
	typeOf := reflect.TypeOf(spec.Params.Value)
	if typeOf == nil {
		return nil, fmt.Errorf("canonical params type is nil for %q", spec.Method)
	}
	schema, err := jsonschema.ForType(typeOf, &jsonschema.ForOptions{IgnoreInvalidTypes: true})
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	return makeBoundFieldsOptional(raw, binding)
}

func makeBoundFieldsOptional(raw json.RawMessage, binding workspaceBinding) (json.RawMessage, error) {
	boundFields := map[string]struct{}{}
	switch binding {
	case workspaceBindingPath:
		boundFields[hostAPIWorkspaceLiteral] = struct{}{}
	case workspaceBindingID:
		boundFields["workspace_id"] = struct{}{}
	case workspaceBindingTask, workspaceBindingMemory:
		boundFields["scope"] = struct{}{}
		boundFields[hostAPIWorkspaceLiteral] = struct{}{}
	case workspaceBindingResource:
		boundFields["scope"] = struct{}{}
	case workspaceBindingAutomation:
		boundFields["scope"] = struct{}{}
		boundFields["workspace_id"] = struct{}{}
	case workspaceBindingActor:
	}
	var schema any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, err
	}
	removeRequiredFields(schema, boundFields)
	return json.Marshal(schema)
}

func removeRequiredFields(value any, fields map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		if required, ok := typed["required"].([]any); ok {
			filtered := required[:0]
			for _, item := range required {
				name, ok := item.(string)
				if _, remove := fields[name]; ok && remove {
					continue
				}
				filtered = append(filtered, item)
			}
			typed["required"] = filtered
		}
		for _, child := range typed {
			removeRequiredFields(child, fields)
		}
	case []any:
		for _, child := range typed {
			removeRequiredFields(child, fields)
		}
	}
}

func hostAPIToolName(method extensionprotocol.HostAPIMethod) string {
	return hostAPIToolPrefix + strings.ReplaceAll(string(method), "/", "__")
}

func hostAPIMethodFromToolName(name string) (extensionprotocol.HostAPIMethod, bool) {
	if !strings.HasPrefix(name, hostAPIToolPrefix) {
		return "", false
	}
	method := extensionprotocol.HostAPIMethod(strings.ReplaceAll(
		strings.TrimPrefix(name, hostAPIToolPrefix),
		"__",
		"/",
	))
	decision, ok := hostAPIProjectionDecisions[method]
	return method, ok && decision.Publish
}
