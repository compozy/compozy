package spec

import "github.com/getkin/kin-openapi/openapi3"

func finalizeAgentInstanceBudgetUpdateSchema(schemas openapi3.Schemas) {
	update := schemas["AgentInstanceUpdateRequest"].Value
	caps := openapi3.NewSchemaRef("#/components/schemas/AgentInstanceDailyBudgetInput",
		schemas["AgentInstanceDailyBudgetInput"].Value)
	const checksField = "checks_enabled"
	const budgetField = "daily_budget"
	absent := &openapi3.Schema{Nullable: true, Enum: []any{nil}}
	checks := openapi3.NewObjectSchema().
		WithProperty(checksField, openapi3.NewBoolSchema()).
		WithProperty(budgetField, absent).
		WithRequired([]string{checksField})
	budget := openapi3.NewObjectSchema().
		WithProperty(checksField, absent).
		WithRequired([]string{budgetField})
	budget.Properties[budgetField] = caps
	update.Description = "Set either checks_enabled or daily_budget. An empty daily_budget object removes both caps."
	update.OneOf = openapi3.SchemaRefs{{Value: checks}, {Value: budget}}
}

func customizeAgentInstanceDailyBudgetInputSchema(schema *openapi3.Schema) {
	schema.WithoutAdditionalProperties()
	schema.Properties["tokens"].Value.WithMin(0)
	schema.Properties["usd"].Value.WithMin(0)
}
