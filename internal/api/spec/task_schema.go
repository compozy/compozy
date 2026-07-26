package spec

import "github.com/getkin/kin-openapi/openapi3"

func describeTaskBlockedReasonsProperty(schema *openapi3.Schema) {
	if schema == nil || schema.Properties == nil {
		return
	}
	property := schema.Properties["blocked_reasons"]
	if property == nil || property.Value == nil {
		return
	}
	property.Value.Description = "Read projection of current blocking causes; mutation responses may omit it."
}
