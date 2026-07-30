package spec

import (
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

type extensionAuthoringComponentSchema struct {
	name  string
	value any
}

var extensionAuthoringComponentSchemas = []extensionAuthoringComponentSchema{
	{name: "IssueSeverity", value: contract.IssueSeverity("")},
	{name: "ValidationIssue", value: contract.ValidationIssue{}},
	{name: "ConsentArea", value: contract.ConsentArea{}},
	{name: "ExtensionManifestSummary", value: contract.ExtensionManifestSummary{}},
	{name: "ExtensionValidatePayload", value: contract.ExtensionValidatePayload{}},
}

func registerExtensionAuthoringComponentSchemas(schemas openapi3.Schemas) error {
	for _, component := range extensionAuthoringComponentSchemas {
		schemaRef, err := schemaRefForValue(component.value, schemas)
		if err != nil {
			return fmt.Errorf("build %s: %w", component.name, err)
		}
		if generated := schemas[component.name]; generated != nil && generated.Value != nil {
			continue
		}
		schemas[component.name] = schemaRef
	}
	for _, component := range extensionAuthoringComponentSchemas {
		schemaRef := schemas[component.name]
		if schemaRef == nil || schemaRef.Value == nil {
			return fmt.Errorf("component %s did not resolve to a concrete schema", component.name)
		}
	}
	return nil
}
