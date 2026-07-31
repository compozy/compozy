package spec

import "github.com/getkin/kin-openapi/openapi3"

const settingsMCPCatalogInputValueKey = "value"

const settingsMCPAuthRedirectURLKey = "redirect_url"

func customizeSettingsMCPCatalogInputSchema(schema *openapi3.Schema) {
	value := openapi3.NewStringSchema().WithMinLength(1)
	value.WriteOnly = true
	valueVariant := openapi3.NewObjectSchema().
		WithProperty(settingsMCPCatalogInputValueKey, value).
		WithRequired([]string{settingsMCPCatalogInputValueKey}).
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
	redirectURL := openapi3.NewStringSchema().WithMinLength(1)
	redirectURL.WriteOnly = true

	*schema = *openapi3.NewObjectSchema().
		WithProperty(settingsMCPAuthRedirectURLKey, redirectURL).
		WithRequired([]string{settingsMCPAuthRedirectURLKey}).
		WithoutAdditionalProperties()
}
