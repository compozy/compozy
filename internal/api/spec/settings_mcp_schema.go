package spec

import "github.com/getkin/kin-openapi/openapi3"

const settingsMCPAuthRedirectURLKey = "redirect_url"

func customizeSettingsMCPAuthExchangeRequestSchema(schema *openapi3.Schema) {
	redirectURL := openapi3.NewStringSchema().WithMinLength(1)
	redirectURL.WriteOnly = true

	*schema = *openapi3.NewObjectSchema().
		WithProperty(settingsMCPAuthRedirectURLKey, redirectURL).
		WithRequired([]string{settingsMCPAuthRedirectURLKey}).
		WithoutAdditionalProperties()
}
