package spec

import "github.com/getkin/kin-openapi/openapi3"

func finalizeAgentInstanceBudgetUpdateSchema(schemas openapi3.Schemas) {
	update := schemas["AgentInstanceUpdateRequest"].Value
	caps := openapi3.NewSchemaRef("#/components/schemas/AgentInstanceDailyBudgetInput",
		schemas["AgentInstanceDailyBudgetInput"].Value)
	const checksField = "checks_enabled"
	const budgetField = "daily_budget"
	const autoApproveField = "skill_auto_approve"
	absent := &openapi3.Schema{Nullable: true, Enum: []any{nil}}
	checks := openapi3.NewObjectSchema().
		WithProperty(checksField, openapi3.NewBoolSchema()).
		WithProperty(budgetField, absent).
		WithProperty(autoApproveField, absent).
		WithRequired([]string{checksField})
	budget := openapi3.NewObjectSchema().WithoutAdditionalProperties().
		WithProperty(checksField, absent).
		WithProperty(autoApproveField, absent).
		WithRequired([]string{budgetField})
	budget.Properties[budgetField] = caps
	autoApprove := openapi3.NewObjectSchema().
		WithProperty(autoApproveField, openapi3.NewBoolSchema()).
		WithProperty(checksField, absent).
		WithProperty(budgetField, absent).
		WithRequired([]string{autoApproveField})
	update.Description = "Set exactly one of checks_enabled, daily_budget, or skill_auto_approve. " +
		"An empty daily_budget object removes both caps. Only an operator can turn skill_auto_approve on."
	update.OneOf = openapi3.SchemaRefs{{Value: checks}, {Value: budget}, {Value: autoApprove}}
}

func customizeAgentInstanceDailyBudgetInputSchema(schema *openapi3.Schema) {
	schema.WithoutAdditionalProperties()
	schema.Properties["tokens"].Value.WithMin(0)
	schema.Properties["usd"].Value.WithMin(0)
}
