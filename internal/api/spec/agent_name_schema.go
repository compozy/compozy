package spec

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/getkin/kin-openapi/openapi3"
)

func setAgentNameSchema(schema *openapi3.Schema, property string, pattern string) {
	if schema == nil || schema.Properties[property] == nil || schema.Properties[property].Value == nil {
		return
	}

	propertySchema := schema.Properties[property]
	propertySchema.Value.Pattern = pattern
	maxLength := uint64(compozyconfig.AgentNameMaxLength)
	propertySchema.Value.MaxLength = &maxLength
}

func customizeCreateAgentPayloadSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "name", compozyconfig.AgentNamePattern)
}

func customizeDuplicateAgentRequestSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "name", compozyconfig.AgentNamePattern)
}

func customizeCreateSessionRequestSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "agent_name", compozyconfig.OptionalAgentNamePattern)
}

func customizeWorkspaceAgentNameSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "default_agent", compozyconfig.OptionalAgentNamePattern)
}

func customizeJobAgentNameSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "agent_name", compozyconfig.OptionalAgentNamePattern)
}

func customizeTriggerAgentNameSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "agent_name", compozyconfig.OptionalAgentNamePattern)
}

func customizeSettingsDefaultsSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "agent", compozyconfig.OptionalAgentNamePattern)
}

func customizeSettingsRoleSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "agent", compozyconfig.OptionalAgentNamePattern)
}
