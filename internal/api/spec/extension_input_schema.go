package spec

import "github.com/getkin/kin-openapi/openapi3"

func customizeExtensionInputValueSchema(schema *openapi3.Schema) {
	value := &openapi3.Schema{WriteOnly: true, OneOf: []*openapi3.SchemaRef{
		{Value: openapi3.NewStringSchema()}, {Value: openapi3.NewBoolSchema()},
	}}
	value.Description = "A manifest-typed value. Strings are limited to 8 KiB of UTF-8 and must not contain NUL."
	valueVariant := openapi3.NewObjectSchema().WithProperty("value", value).
		WithRequired([]string{"value"}).WithoutAdditionalProperties()
	ref := openapi3.NewStringSchema().WithMinLength(1)
	ref.Pattern = `^vault:extensions/.+$`
	refVariant := openapi3.NewObjectSchema().WithProperty("vault_ref", ref).
		WithRequired([]string{"vault_ref"}).WithoutAdditionalProperties()
	*schema = openapi3.Schema{OneOf: []*openapi3.SchemaRef{{Value: valueVariant}, {Value: refVariant}}}
}

func customizeExtensionInstallRequestSchema(schema *openapi3.Schema) {
	if scope := schema.Properties["scope"]; scope != nil && scope.Value != nil {
		scope.Value.Enum = []any{"global", "workspace"}
		scope.Value.Description = "Overrides manifest server defaults. Mixed defaults require an explicit scope. " +
			"A workspace default requires a workspace-bound caller or workspace_id."
	}
	if profile := schema.Properties["profile"]; profile != nil && profile.Value != nil {
		profile.Value.Description = "Profile name. Omitted for a local operator keeps the installation " +
			"available to all profiles."
	}
	if workspace := schema.Properties["workspace_id"]; workspace != nil && workspace.Value != nil {
		workspace.Value.Description = "Registered workspace ID. Required for workspace scope " +
			"unless the caller is already workspace-bound."
	}
	if digest := schema.Properties["expected_digest"]; digest != nil && digest.Value != nil {
		digest.Value.Pattern = `^[a-fA-F0-9]{64}$`
	}
}

func customizeExtensionUpdateRequestSchema(schema *openapi3.Schema) {
	if scope := schema.Properties["scope"]; scope != nil && scope.Value != nil {
		scope.Value.Enum = []any{"global", "workspace"}
		scope.Value.Description = "Select an existing installation; updates preserve every package attachment."
	}
	if profile := schema.Properties["profile"]; profile != nil && profile.Value != nil {
		profile.Value.Description = "Profile name for input validation and persistence. " +
			"Inputs do not cross profile boundaries."
	}
}
