package spec

import (
	"github.com/compozy/agh/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

const loopKindField = "kind"

func customizeLoopGraphSchema(schema *openapi3.Schema) {
	*schema = *openapi3.NewObjectSchema().
		WithProperty("nodes", openapi3.NewArraySchema().WithItems(loopGraphNodeSchema())).
		WithProperty("edges", openapi3.NewArraySchema().WithItems(loopGraphEdgeSchema())).
		WithoutAdditionalProperties()
	schema.Required = []string{"edges", "nodes"}
}

func loopGraphNodeSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("id", openapi3.NewStringSchema()).
		WithProperty("class", openapi3.NewStringSchema().WithEnum(enumAsAny(loopNodeClassValues())...)).
		WithProperty(loopKindField, openapi3.NewStringSchema()).
		WithProperty("session", loopFreeformObjectSchema()).
		WithProperty("timeout", openapi3.NewStringSchema()).
		WithProperty("retry", loopFreeformObjectSchema()).
		WithProperty("harvest", loopFreeformObjectSchema()).
		WithProperty("produces", loopFreeformObjectSchema()).
		WithProperty("params", loopFreeformObjectSchema()).
		WithProperty("collection", openapi3.NewStringSchema()).
		WithProperty("filter", openapi3.NewStringSchema()).
		WithProperty("batch_size", openapi3.NewIntegerSchema()).
		WithProperty("max_parallel", openapi3.NewIntegerSchema()).
		WithProperty("max_fan_out", openapi3.NewIntegerSchema()).
		WithProperty("condition", openapi3.NewStringSchema()).
		WithProperty("criteria", openapi3.NewArraySchema().WithItems(loopGateCriterionSchema())).
		WithProperty("verdict_policy", openapi3.NewStringSchema()).
		WithProperty("on_result", loopFreeformObjectSchema()).
		WithProperty("max_revisions", openapi3.NewIntegerSchema()).
		WithProperty("body", loopFreeformObjectSchema()).
		WithProperty("contract", loopFreeformObjectSchema()).
		WithProperty("input_ref", openapi3.NewStringSchema()).
		WithProperty("pattern", openapi3.NewStringSchema()).
		WithProperty("parse", openapi3.NewStringSchema()).
		WithProperty("watch", loopFreeformObjectSchema()).
		WithProperty("events", openapi3.NewArraySchema().WithItems(loopWatchEventSubscriptionSchema())).
		WithAdditionalProperties(openapi3.NewSchema())
	schema.Required = []string{"class", "id", loopKindField}
	return schema
}

func loopWatchEventSubscriptionSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty(
			loopKindField,
			openapi3.NewStringSchema().WithEnum(enumAsAny(contract.LoopWatchEventKindValues())...),
		).
		WithProperty("filter", openapi3.NewStringSchema())
	schema.Required = []string{loopKindField}
	return schema
}

func loopGraphEdgeSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("from", openapi3.NewStringSchema()).
		WithProperty("to", openapi3.NewStringSchema()).
		WithAdditionalProperties(openapi3.NewSchema())
	schema.Required = []string{"from", "to"}
	return schema
}

func loopGateCriterionSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("id", openapi3.NewStringSchema()).
		WithProperty("type", openapi3.NewStringSchema()).
		WithProperty("check", openapi3.NewStringSchema()).
		WithProperty("expect", openapi3.NewStringSchema()).
		WithProperty("agent", openapi3.NewStringSchema()).
		WithProperty("rubric", openapi3.NewStringSchema()).
		WithProperty("prompt", openapi3.NewStringSchema()).
		WithProperty("tool", openapi3.NewStringSchema()).
		WithProperty("inputs", loopFreeformObjectSchema()).
		WithAdditionalProperties(openapi3.NewSchema())
	schema.Required = []string{"type"}
	return schema
}

func loopFreeformObjectSchema() *openapi3.Schema {
	return openapi3.NewObjectSchema().WithAdditionalProperties(openapi3.NewSchema())
}
