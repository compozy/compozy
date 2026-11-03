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

// agentKindRequestDescription marks the v0.5.0 spelling as an input alias. Removal: v0.5.2.
const agentKindRequestDescription = "Agent kind: on_demand (default) or always_on; fixed at creation. " +
	"stays_on is the deprecated v0.5.0 spelling of always_on and is removed in v0.5.2."

// setAgentKindRequestSchema lists the accepted request kinds, including the deprecated stays_on
// alias. Responses keep a plain string kind. Removal of stays_on: v0.5.2.
func setAgentKindRequestSchema(schema *openapi3.Schema) {
	if kind := schema.Properties["kind"]; kind != nil && kind.Value != nil {
		kind.Value.Enum = []any{
			compozyconfig.AgentKindOnDemand,
			compozyconfig.AgentKindAlwaysOn,
			compozyconfig.AgentKindStaysOnAlias,
		}
		kind.Value.Description = agentKindRequestDescription
	}
}

func customizeDuplicateAgentOverridesSchema(schema *openapi3.Schema) {
	setAgentKindRequestSchema(schema)
}

func customizeCreateAgentPayloadSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "name", compozyconfig.AgentNamePattern)
	setAgentKindRequestSchema(schema)
	for key, limit := range map[string]uint64{"display_name": 80, "description": 280} {
		if field := schema.Properties[key]; field != nil && field.Value != nil {
			field.Value.MaxLength = new(limit)
		}
	}
}

func customizeDuplicateAgentRequestSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "name", compozyconfig.AgentNamePattern)
	setAgentKindRequestSchema(schema)
	for key, limit := range map[string]uint64{"display_name": 80, "description": 280} {
		if field := schema.Properties[key]; field != nil && field.Value != nil {
			field.Value.MaxLength = new(limit)
		}
	}
}

func customizeAgentSpawnRequestSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "agent_name", compozyconfig.AgentNamePattern)
}

func customizeCreateSessionRequestSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "agent_name", compozyconfig.OptionalAgentNamePattern)
}

func customizeWorkspaceAgentNameSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "default_agent", compozyconfig.OptionalAgentNamePattern)
}

func customizeJobAgentNameSchema(schema *openapi3.Schema) {
	customizeUserScopeInputSchema(schema)
	setAgentNameSchema(schema, "agent_name", compozyconfig.OptionalAgentNamePattern)
}

func customizeTriggerAgentNameSchema(schema *openapi3.Schema) {
	customizeUserScopeInputSchema(schema)
	setAgentNameSchema(schema, "agent_name", compozyconfig.OptionalAgentNamePattern)
}

func customizeSettingsDefaultsSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "agent", compozyconfig.OptionalAgentNamePattern)
}

func customizeSettingsRoleSchema(schema *openapi3.Schema) {
	setAgentNameSchema(schema, "agent", compozyconfig.OptionalAgentNamePattern)
}
