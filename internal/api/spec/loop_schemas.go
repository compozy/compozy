package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
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

func loopEffectListSchema() *openapi3.Schema {
	return openapi3.NewArraySchema().WithItems(loopEffectSchema())
}

func withLoopLifecycleProperties(schema *openapi3.Schema) *openapi3.Schema {
	return schema.
		WithProperty("deadline", openapi3.NewStringSchema()).
		WithProperty("result_contract", loopResultContractSchema()).
		WithProperty("on_error", loopErrorPolicySchema()).
		WithProperty("on_retry", loopEffectListSchema()).
		WithProperty("on_success", loopEffectListSchema()).
		WithProperty("on_pause", loopEffectListSchema()).
		WithProperty("on_timeout", loopEffectListSchema()).
		WithProperty("on_cancel", loopEffectListSchema()).
		WithProperty("on_quarantine", loopEffectListSchema()).
		WithProperty("on_parent_close", openapi3.NewStringSchema().
			WithEnum(enumAsAny(contract.LoopParentClosePolicyValues())...)).
		WithProperty("expires", loopWaitExpirySchema())
}

func loopResultContractSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("failure_field", openapi3.NewStringSchema()).
		WithProperty("message_field", openapi3.NewStringSchema()).
		WithoutAdditionalProperties()
	schema.Required = []string{"failure_field"}
	return schema
}

func loopErrorPolicySchema() *openapi3.Schema {
	return openapi3.NewObjectSchema().
		WithProperty("route", openapi3.NewStringSchema()).
		WithProperty("allow_fail", openapi3.NewBoolSchema()).
		WithProperty("effects", loopEffectListSchema()).
		WithoutAdditionalProperties()
}

func loopWaitExpirySchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("after", openapi3.NewStringSchema()).
		WithProperty("escalate", loopEffectListSchema()).
		WithProperty("route", openapi3.NewStringSchema()).
		WithoutAdditionalProperties()
	schema.Required = []string{"after"}
	return schema
}

func loopEffectSchema() *openapi3.Schema {
	emit := openapi3.NewObjectSchema().
		WithProperty("emit", loopEmitSchema()).
		WithoutAdditionalProperties()
	emit.Required = []string{"emit"}
	tool := openapi3.NewObjectSchema().
		WithProperty("tool", openapi3.NewStringSchema()).
		WithProperty("with", loopFreeformObjectSchema()).
		WithoutAdditionalProperties()
	tool.Required = []string{"tool"}
	return &openapi3.Schema{OneOf: []*openapi3.SchemaRef{{Value: emit}, {Value: tool}}}
}

func loopEmitSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("kind", openapi3.NewStringSchema()).
		WithProperty("payload", loopFreeformObjectSchema()).
		WithoutAdditionalProperties()
	schema.Required = []string{loopKindField}
	return schema
}

func loopRetrySchema() *openapi3.Schema {
	backoff := openapi3.NewObjectSchema().
		WithProperty("base", openapi3.NewStringSchema()).
		WithProperty("max", openapi3.NewStringSchema()).
		WithoutAdditionalProperties()
	return openapi3.NewObjectSchema().
		WithProperty("max_attempts", openapi3.NewIntegerSchema()).
		WithProperty("on_failure", openapi3.NewStringSchema()).
		WithProperty("backoff", backoff).
		WithProperty("non_retryable", openapi3.NewArraySchema().WithItems(openapi3.NewStringSchema())).
		WithoutAdditionalProperties()
}

func loopEnvironmentSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty("mode", openapi3.NewStringSchema().WithEnum(enumAsAny(loopEnvironmentModeValues())...)).
		WithProperty("worktree_ref", openapi3.NewStringSchema()).
		WithProperty("directory", openapi3.NewStringSchema()).
		WithoutAdditionalProperties()
	schema.Required = []string{"mode"}
	return schema
}

func loopActionParamsSchema() *openapi3.Schema {
	return openapi3.NewObjectSchema().
		WithProperty("environment", loopEnvironmentSchema()).
		WithAdditionalProperties(openapi3.NewSchema())
}

func loopGraphNodeSchema() *openapi3.Schema {
	schema := withLoopLifecycleProperties(openapi3.NewObjectSchema()).
		WithProperty("id", openapi3.NewStringSchema()).
		WithProperty("class", openapi3.NewStringSchema().WithEnum(enumAsAny(loopNodeClassValues())...)).
		WithProperty(loopKindField, openapi3.NewStringSchema()).
		WithProperty("session", loopFreeformObjectSchema()).
		WithProperty("timeout", openapi3.NewStringSchema()).
		WithProperty("retry", loopRetrySchema()).
		WithProperty("harvest", loopFreeformObjectSchema()).
		WithProperty("produces", loopFreeformObjectSchema()).
		WithProperty("params", loopActionParamsSchema()).
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
		WithProperty("contains", openapi3.NewStringSchema()).
		WithProperty("agent", openapi3.NewStringSchema()).
		WithProperty("rubric", openapi3.NewStringSchema()).
		WithProperty("prompt", openapi3.NewStringSchema()).
		WithProperty("tool", openapi3.NewStringSchema()).
		WithProperty("inputs", loopFreeformObjectSchema()).
		WithProperty("metric", loopMetricSchema()).
		WithAdditionalProperties(openapi3.NewSchema())
	schema.Required = []string{"type"}
	return schema
}

func loopMetricSchema() *openapi3.Schema {
	schema := openapi3.NewObjectSchema().
		WithProperty(
			"direction",
			openapi3.NewStringSchema().WithEnum(enumAsAny(contract.LoopMetricDirectionValues())...),
		).
		WithProperty("min_delta", openapi3.NewFloat64Schema()).
		WithoutAdditionalProperties()
	schema.Required = []string{"direction"}
	return schema
}

func loopFreeformObjectSchema() *openapi3.Schema {
	return openapi3.NewObjectSchema().WithAdditionalProperties(openapi3.NewSchema())
}
