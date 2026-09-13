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
	ref.Pattern = `^vault:(extensions|mcp)/.+$`
	refVariant := openapi3.NewObjectSchema().WithProperty("vault_ref", ref).
		WithRequired([]string{"vault_ref"}).WithoutAdditionalProperties()
	*schema = openapi3.Schema{OneOf: []*openapi3.SchemaRef{{Value: valueVariant}, {Value: refVariant}}}
}

func customizeExtensionInstallRequestSchema(schema *openapi3.Schema) {
	if digest := schema.Properties["expected_digest"]; digest != nil && digest.Value != nil {
		digest.Value.Pattern = `^[a-fA-F0-9]{64}$`
	}
}
