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

func customizeSettingsGeneralConfigSchema(schema *openapi3.Schema) {
	if schema == nil {
		return
	}
	if timeout := schema.Properties["session_timeout"]; timeout != nil && timeout.Value != nil {
		timeout.Value.Deprecated = true
		timeout.Value.Description = "Deprecated: mirrors session.limits.timeout, which no session enforces. " +
			"Use idle_release_after. Removed in v0.5.0."
	}
	if release := schema.Properties["idle_release_after"]; release != nil && release.Value != nil {
		release.Value.Description = "Idle time before a resumable idle session's agent process is released " +
			"(session.supervision.idle_release_after); 0s disables it. Omit on update to keep the current value."
	}
}
