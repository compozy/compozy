package spec

import "github.com/getkin/kin-openapi/openapi3"

func customizePutNetworkCoordinationRequestSchema(schema *openapi3.Schema) {
	customizeNetworkCoordinationMutationSchema(schema, "enabled")
}

func customizePutNetworkCoordinationInvitationRequestSchema(schema *openapi3.Schema) {
	customizeNetworkCoordinationMutationSchema(schema, "dismissed")
}

func customizeNetworkCoordinationMutationSchema(schema *openapi3.Schema, mutationProperty string) {
	*schema = openapi3.Schema{
		OneOf: []*openapi3.SchemaRef{
			{Value: networkCoordinationMutationBranch(specWorkspaceKey, mutationProperty)},
			{Value: networkCoordinationMutationBranch("task", mutationProperty)},
		},
	}
}

func networkCoordinationMutationBranch(scope string, mutationProperty string) *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty(specScopeKey, stringEnumSchema(scope)).
		WithProperty("run_id", openapi3.NewStringSchema().WithMinLength(1)).
		WithProperty(mutationProperty, openapi3.NewBoolSchema()).
		WithProperty("expected_revision", openapi3.NewInt64Schema().WithMin(0)).
		WithoutAdditionalProperties()
	if scope == "task" {
		schema.WithProperty("task_id", openapi3.NewStringSchema().WithMinLength(1))
		schema.Required = []string{specScopeKey, "task_id", mutationProperty, "expected_revision"}
		return schema
	}
	schema.Required = []string{specScopeKey, mutationProperty, "expected_revision"}
	return schema
}
