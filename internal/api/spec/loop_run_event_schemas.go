package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

func customizeLoopRunEventPayloadSchema(schema *openapi3.Schema) {
	otherKinds := make([]string, 0, len(contract.LoopRunEventKindValues())-2)
	for _, kind := range contract.LoopRunEventKindValues() {
		if kind != string(contract.LoopRunEventGenerationStarted) &&
			kind != string(contract.LoopRunEventGateVerdict) {
			otherKinds = append(otherKinds, kind)
		}
	}
	setLoopRunEventKindSchema(schema, otherKinds...)
}

func customizeLoopGenerationStartedRunEventPayloadSchema(schema *openapi3.Schema) {
	setLoopRunEventKindSchema(schema, string(contract.LoopRunEventGenerationStarted))
}

func customizeLoopGateVerdictRunEventPayloadSchema(schema *openapi3.Schema) {
	setLoopRunEventKindSchema(schema, string(contract.LoopRunEventGateVerdict))
}

func setLoopRunEventKindSchema(schema *openapi3.Schema, kinds ...string) {
	if schema == nil {
		return
	}
	kind := schema.Properties["kind"]
	if kind == nil || kind.Value == nil {
		return
	}
	kind.Value.Enum = enumAsAny(kinds)
}
