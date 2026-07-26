package spec

import (
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/getkin/kin-openapi/openapi3"
)

const openapiTSWidenAdditionalPropertiesExtension = "x-agh-typescript-widen-additional-properties"

func bridgeProviderConfigSchema() *openapi3.Schema {
	return openapi3.NewObjectSchema().
		WithNullable().
		WithAdditionalProperties(openapi3.NewSchema())
}

func bridgeDeliveryDefaultsSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithNullable().
		WithProperty("peer_id", openapi3.NewStringSchema()).
		WithProperty("thread_id", openapi3.NewStringSchema()).
		WithProperty("group_id", openapi3.NewStringSchema()).
		WithProperty("mode", openapi3.NewStringSchema().WithEnum(enumAsAny(deliveryModeValues())...)).
		WithProperty("progress", bridgeProgressConfigSchema()).
		WithAdditionalProperties(openapi3.NewStringSchema())
	schema.Extensions = map[string]any{openapiTSWidenAdditionalPropertiesExtension: true}
	return schema
}

func bridgeProgressConfigSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("tool_progress", openapi3.NewStringSchema().WithEnum(enumAsAny([]string{
			string(bridgepkg.ProgressModeOff),
			string(bridgepkg.ProgressModeNew),
			string(bridgepkg.ProgressModeAll),
			string(bridgepkg.ProgressModeVerbose),
		})...)).
		WithProperty("grouping", openapi3.NewStringSchema().WithEnum(enumAsAny([]string{
			string(bridgepkg.ProgressGroupingAccumulate),
			string(bridgepkg.ProgressGroupingSeparate),
		})...)).
		WithProperty("typing", openapi3.NewBoolSchema()).
		WithProperty("reactions", openapi3.NewBoolSchema()).
		WithoutAdditionalProperties()
	schema.Required = []string{"tool_progress", "grouping"}
	return schema
}
