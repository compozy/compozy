package spec

import (
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

type windowManagerComponentSchema struct {
	name  string
	value any
}

var windowManagerBaseComponentSchemas = []windowManagerComponentSchema{
	{name: "WindowManagerLayoutNode", value: contract.WindowManagerLayoutNode{}},
	{name: "WindowManagerLayoutGroup", value: contract.WindowManagerLayoutGroup{}},
	{name: "WindowManagerDesktop", value: contract.WindowManagerDesktop{}},
	{name: "WindowManagerSnapshot", value: contract.WindowManagerSnapshot{}},
	{name: "WindowManagerLayoutDocument", value: contract.WindowManagerLayoutDocument{}},
	{name: "WindowManagerCommandRequest", value: contract.WindowManagerCommandRequest{}},
	{name: "WindowManagerResult", value: contract.WindowManagerResult{}},
	{name: "WindowManagerPreview", value: contract.WindowManagerPreview{}},
	{name: "WindowManagerClientView", value: contract.WindowManagerClientView{}},
	{name: "WindowManagerClientsResponse", value: contract.WindowManagerClientsResponse{}},
	{name: "WindowManagerClientRegistration", value: contract.WindowManagerClientRegistration{}},
	{name: "WindowManagerLayoutValidationRequest", value: contract.WindowManagerLayoutValidationRequest{}},
	{name: "WindowManagerLayoutValidationResponse", value: contract.WindowManagerLayoutValidationResponse{}},
	{name: "WindowManagerLayoutReplaceRequest", value: contract.WindowManagerLayoutReplaceRequest{}},
	{name: "WindowManagerErrorPayload", value: contract.WindowManagerErrorPayload{}},
	{name: "WindowManagerSnapshotFrame", value: contract.WindowManagerSnapshotFrame{}},
	{name: "WindowManagerEventFrame", value: contract.WindowManagerEventFrame{}},
	{name: "WindowManagerClientFrame", value: contract.WindowManagerClientFrame{}},
	{name: "WindowManagerErrorFrame", value: contract.WindowManagerErrorFrame{}},
}

var windowManagerComponentSchemas = buildWindowManagerComponentSchemas()

func buildWindowManagerComponentSchemas() []windowManagerComponentSchema {
	components := append([]windowManagerComponentSchema(nil), windowManagerBaseComponentSchemas...)
	for _, command := range windowManagerCommandSchemaSpecs {
		components = append(components, command.payload)
	}
	return components
}

func registerWindowManagerComponentSchemas(schemas openapi3.Schemas) error {
	for _, component := range windowManagerComponentSchemas {
		schemaRef, err := schemaRefForValue(component.value, schemas)
		if err != nil {
			return fmt.Errorf("build %s: %w", component.name, err)
		}
		if generated := schemas[component.name]; generated != nil && generated.Value != nil {
			continue
		}
		schemas[component.name] = schemaRef
	}
	for _, component := range windowManagerComponentSchemas {
		schemaRef := schemas[component.name]
		if schemaRef == nil || schemaRef.Value == nil {
			return fmt.Errorf("component %s did not resolve to a concrete schema", component.name)
		}
	}
	customizeWindowManagerSchemas(schemas)
	return nil
}

func customizeWindowManagerSchemas(schemas openapi3.Schemas) {
	for _, component := range windowManagerComponentSchemas {
		if schemaRef := schemas[component.name]; schemaRef != nil && schemaRef.Value != nil {
			schemaRef.Value.WithoutAdditionalProperties()
		}
	}
	customizeWindowManagerCommandRequest(schemas)
	customizeWindowManagerRouteIntent(schemas)
}

func customizeWindowManagerRouteIntent(schemas openapi3.Schemas) {
	payload := schemas["WindowManagerNavigateWindowPayload"]
	if payload == nil || payload.Value == nil {
		return
	}
	route := payload.Value.Properties["route"]
	if route == nil || route.Value == nil {
		return
	}
	route.Value.WithoutAdditionalProperties()
}

func customizeWindowManagerFrameSchema(schema *openapi3.Schema, value string) {
	if schema == nil {
		return
	}
	typeRef := schema.Properties["type"]
	if typeRef != nil && typeRef.Value != nil {
		setStringEnum(typeRef.Value, []string{value})
	}
}
