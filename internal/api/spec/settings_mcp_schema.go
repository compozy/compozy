package spec

import "github.com/getkin/kin-openapi/openapi3"

const settingsMCPSecretValueKey = "value"

const (
	settingsMCPAuthCodeKey        = "code"
	settingsMCPAuthRedirectURLKey = "redirect_url"
)

func customizeSettingsMCPSecretInputSchema(schema *openapi3.Schema) {
	value := openapi3.NewStringSchema().WithMinLength(1)
	value.WriteOnly = true
	valueVariant := openapi3.NewObjectSchema().
		WithProperty(settingsMCPSecretValueKey, value).
		WithRequired([]string{settingsMCPSecretValueKey}).
		WithoutAdditionalProperties()

	vaultRef := openapi3.NewStringSchema().WithMinLength(1)
	vaultRef.Pattern = `^vault:mcp/.+$`
	vaultRefVariant := openapi3.NewObjectSchema().
		WithProperty("vault_ref", vaultRef).
		WithRequired([]string{"vault_ref"}).
		WithoutAdditionalProperties()

	*schema = openapi3.Schema{
		OneOf: []*openapi3.SchemaRef{
			{Value: valueVariant},
			{Value: vaultRefVariant},
		},
	}
}

func customizeSettingsMCPAuthExchangeRequestSchema(schema *openapi3.Schema) {
	code := openapi3.NewStringSchema().WithMinLength(1)
	code.WriteOnly = true
	codeVariant := openapi3.NewObjectSchema().
		WithProperty(settingsMCPAuthCodeKey, code).
		WithRequired([]string{settingsMCPAuthCodeKey}).
		WithoutAdditionalProperties()

	redirectURL := openapi3.NewStringSchema().WithMinLength(1)
	redirectURL.WriteOnly = true
	redirectURLVariant := openapi3.NewObjectSchema().
		WithProperty(settingsMCPAuthRedirectURLKey, redirectURL).
		WithRequired([]string{settingsMCPAuthRedirectURLKey}).
		WithoutAdditionalProperties()

	*schema = openapi3.Schema{
		OneOf: []*openapi3.SchemaRef{
			{Value: codeVariant},
			{Value: redirectURLVariant},
		},
	}
}
