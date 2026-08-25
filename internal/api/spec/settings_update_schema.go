package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

func customizeSettingsUpdateApplyRequestSchema(schema *openapi3.Schema) {
	customizeSettingsUpdateTargetSetProperty(schema, "targets")
}

func customizeSettingsUpdateApplyResponseSchema(schema *openapi3.Schema) {
	customizeSettingsUpdateTargetSetProperty(schema, "targets")
}

func customizeSettingsUpdateTargetSetProperty(schema *openapi3.Schema, property string) {
	if schema == nil {
		return
	}
	propertySchema := schema.Properties[property]
	if propertySchema == nil {
		return
	}
	propertySchema.Value = settingsUpdateTargetSetSchema()
}

func settingsUpdateTargetSetSchema() *openapi3.Schema {
	targets := openapi3.NewArraySchema().WithItems(
		openapi3.NewStringSchema().WithEnum(
			string(contract.SettingsUpdateTargetRuntime),
			string(contract.SettingsUpdateTargetApp),
		),
	)
	targets.Enum = []any{
		[]string{string(contract.SettingsUpdateTargetRuntime)},
		[]string{string(contract.SettingsUpdateTargetApp)},
		[]string{
			string(contract.SettingsUpdateTargetRuntime),
			string(contract.SettingsUpdateTargetApp),
		},
	}
	return targets
}
