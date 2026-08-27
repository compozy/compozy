package spec

import (
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/contracts"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/getkin/kin-openapi/openapi3"
)

const (
	callExpectSchemaDescription = "JSON Schema or example-shape shorthand for the required structured result."
	callResultSchemaDescription = "Free-form structured JSON returned under the call's declared result contract."
)

func customizeCallTargetRequestSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	schema.OneOf = openapi3.SchemaRefs{
		openapi3.NewObjectSchema().WithRequired([]string{"agent"}).WithProperty(
			"agent",
			openapi3.NewStringSchema().WithMinLength(1),
		).NewRef(),
		openapi3.NewObjectSchema().WithRequired([]string{"session_id"}).WithProperty(
			"session_id",
			openapi3.NewStringSchema().WithMinLength(1),
		).NewRef(),
	}
}

func customizeCreateCallItemRequestSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	schema.Required = []string{"target", "prompt"}
	if prompt := schema.Properties["prompt"]; prompt != nil && prompt.Value != nil {
		prompt.Value.MinLength = 1
	}
	if expect := schema.Properties["expect"]; expect != nil {
		expect.Value = callFreeFormObjectSchema(callExpectSchemaDescription)
	}
	setCallEnumProperty(schema, "result_overflow", callResultOverflowValues())
}

func customizeCreateCallRequestSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	schema.Required = nil
	if prompt := schema.Properties["prompt"]; prompt != nil && prompt.Value != nil {
		prompt.Value.MinLength = 1
	}
	single := openapi3.NewObjectSchema().WithRequired([]string{"target", "prompt"})
	single.Properties["target"] = schema.Properties["target"]
	single.Properties["prompt"] = schema.Properties["prompt"]
	single.Not = openapi3.NewObjectSchema().WithRequired([]string{"tasks"}).NewRef()
	batch := openapi3.NewObjectSchema().WithRequired([]string{"tasks"})
	batch.Properties["tasks"] = schema.Properties["tasks"]
	inlineFields := []string{
		"target", "prompt", "expect", "idle_ttl_seconds", "deadline_seconds", "strict",
		"result_budget", "result_overflow", "idempotency_key", "runtime", "narrow",
	}
	inline := openapi3.NewSchema()
	for _, field := range inlineFields {
		inline.AnyOf = append(inline.AnyOf, openapi3.NewObjectSchema().WithRequired([]string{field}).NewRef())
	}
	batch.Not = inline.NewRef()
	if tasks := schema.Properties["tasks"]; tasks != nil && tasks.Value != nil {
		tasks.Value.MinItems = 1
	}
	setCallEnumProperty(schema, "scope", callScopeValues())
	setCallEnumProperty(schema, "result_overflow", callResultOverflowValues())
	if expect := schema.Properties["expect"]; expect != nil {
		expect.Value = callFreeFormObjectSchema(callExpectSchemaDescription)
	}
	schema.OneOf = openapi3.SchemaRefs{single.NewRef(), batch.NewRef()}
}

func customizeCallRuntimeRequestSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setCallEnumProperty(schema, "speed", speedpkg.Values())
}

func customizeCallPayloadSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setCallEnumProperty(schema, "scope", callScopeValues())
	setCallEnumProperty(schema, "state", callStateValues())
	setCallEnumProperty(schema, "verdict", callVerdictValues())
	setCallEnumProperty(schema, "result_overflow", callResultOverflowValues())
	setCallFreeFormProperty(schema, "result_preview", callResultSchemaDescription)
	setCallFreeFormProperty(schema, "superseded_preview", callResultSchemaDescription)
}

func customizeCallCreatePayloadSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setCallEnumProperty(schema, "state", callStateValues())
}

func customizeCallBatchItemPayloadSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setCallEnumProperty(schema, "state", callStateValues())
}

func customizeAwaitCallsResponseSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setCallEnumProperty(schema, "outcome", []string{
		string(callspkg.AwaitOutcomeComplete),
		string(callspkg.AwaitOutcomePartial),
		string(callspkg.AwaitOutcomeTimeout),
	})
}

func customizeCallStateResponseSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setCallEnumProperty(schema, "state", callStateValues())
}

func customizeCallMessagePayloadSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setCallEnumProperty(schema, "scope", callScopeValues())
	setCallEnumProperty(schema, "delivery", callMessageDeliveryValues())
}

func customizeSendCallMessageResponseSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setCallEnumProperty(schema, "delivery", callMessageDeliveryValues())
}

func customizeCallResultResponseSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setCallFreeFormProperty(schema, "result", callResultSchemaDescription)
}

func setCallFreeFormProperty(schema *openapi3.Schema, name string, description string) {
	if property := schema.Properties[name]; property != nil {
		property.Value = callFreeFormObjectSchema(description)
	}
}

func callFreeFormObjectSchema(description string) *openapi3.Schema {
	schema := openapi3.NewObjectSchema().WithAnyAdditionalProperties()
	schema.Description = description
	return schema
}

func setCallEnumProperty(schema *openapi3.Schema, name string, values []string) {
	property := schema.Properties[name]
	if property == nil || property.Value == nil {
		return
	}
	enums := make([]any, 0, len(values))
	for _, value := range values {
		enums = append(enums, value)
	}
	property.Value.Enum = enums
}

func callScopeValues() []string {
	return []string{string(callspkg.ScopeGlobal), string(callspkg.ScopeWorkspace)}
}

func callStateValues() []string {
	return []string{
		string(callspkg.StateQueued),
		string(callspkg.StateRunning),
		string(callspkg.StateCompleted),
		string(callspkg.StateInvalidResult),
		string(callspkg.StateCompletedWithoutResult),
		string(callspkg.StateFailed),
		string(callspkg.StateCanceled),
		string(callspkg.StateTimeout),
		string(callspkg.StateExpired),
	}
}

func callVerdictValues() []string {
	return []string{
		string(callspkg.VerdictReturned),
		string(callspkg.VerdictExtracted),
		string(callspkg.VerdictRepaired),
	}
}

func callResultOverflowValues() []string {
	return []string{string(contracts.OverflowStore), string(contracts.OverflowReject)}
}

func callMessageDeliveryValues() []string {
	return []string{
		string(callspkg.MessageDeliveryQueued),
		string(callspkg.MessageDeliveryDeliveredIntoTurn),
		string(callspkg.MessageDeliveryWoke),
		string(callspkg.MessageDeliveryFailed),
	}
}
